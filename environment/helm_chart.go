package environment

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cavaliercoder/grab"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/cmd/cp"
)

const (
	// AppEnumerationLabelKey label used to enumerate instances of the same app to ease access
	AppEnumerationLabelKey = "app"
	// InstanceEnumerationLabelKey additional label to enumerate app instances
	InstanceEnumerationLabelKey = "instance"
)

// Hook is an environment hook to be ran either before or after a deployment
type Hook func(environment *Environment) error

// HelmChart represents a single Helm chart to be installed into a cluster
type HelmChart struct {
	ReleaseName      string                 `yaml:"release_name,omitempty" json:"release_name,omitempty" envconfig:"release_name"`
	Path             string                 `yaml:"path,omitempty" json:"path,omitempty" envconfig:"path"`
	URL              string                 `yaml:"url,omitempty" json:"url,omitempty" envconfig:"url"`
	Values           map[string]interface{} `yaml:"values,omitempty" json:"values,omitempty" envconfig:"values"`
	Index            int                    `yaml:"index,omitempty" json:"index,omitempty" envconfig:"index"`
	AutoConnect      bool                   `yaml:"auto_connect" json:"auto_connect" envconfig:"auto_connect"`
	ChartConnections ChartConnections       `yaml:"chart_connections,omitempty" json:"chart_connections,omitempty" envconfig:"chart_connections"`
	BeforeHook       Hook                   `yaml:"-" json:"-" envconfig:"-"`
	AfterHook        Hook                   `yaml:"-" json:"-" envconfig:"-"`

	// Internal properties used for deployment
	namespaceName string
	env           *Environment
	actionConfig  *action.Configuration
	podsList      *v1.PodList
}

// Init sets up the connection to helm for the chart to be managed
func (hc *HelmChart) Init(env *Environment) error {
	if hc.ChartConnections == nil {
		hc.ChartConnections = ChartConnections{}
	}
	hc.env = env
	hc.namespaceName = env.Namespace
	return hc.init()
}

// Connect connects to all exposed containerPorts, forwards them to local
func (hc *HelmChart) Connect() error {
	var rangeErr error
	hc.ChartConnections.Range(func(key string, chartConnection *ChartConnection) bool {
		rules, err := hc.makePortRules(chartConnection)
		if err != nil {
			rangeErr = err
			return false
		}
		if err := hc.connectPod(chartConnection, rules); err != nil {
			rangeErr = err
			return false
		}
		return true
	})
	if rangeErr != nil {
		return rangeErr
	}
	return nil
}

// Deploy deploys a chart and update config settings
func (hc *HelmChart) Deploy() error {
	if len(hc.URL) > 0 {
		if err := hc.downloadChart(); err != nil {
			return err
		}
	}
	if hc.BeforeHook != nil {
		if err := hc.BeforeHook(hc.env); err != nil {
			return err
		}
	}
	if err := hc.deployChart(); err != nil {
		return err
	}
	if err := hc.enumerateApps(); err != nil {
		return err
	}
	if err := hc.fetchPods(); err != nil {
		return err
	}
	if err := hc.updateChartSettings(); err != nil {
		return err
	}
	if hc.AutoConnect {
		if err := hc.Connect(); err != nil {
			return err
		}
	}
	if hc.AfterHook != nil {
		if err := hc.AfterHook(hc.env); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall uninstalls the helm chart
func (hc *HelmChart) Uninstall() error {
	log.Debug().Str("Release", hc.ReleaseName).Msg("Uninstalling Helm release")
	if _, err := action.NewUninstall(hc.actionConfig).Run(hc.ReleaseName); err != nil {
		if !strings.Contains(err.Error(), "release: not found") { // If the release isn't installed, assume it didn't make it that far
			return err
		}
		log.Warn().Str("Release Name", hc.ReleaseName).Msg("Unable to find release to uninstall it")
	}
	return nil
}

// Upgrade an already deployed Helm chart with new values
func (hc *HelmChart) Upgrade() error {
	helmChart, err := hc.loadChart()
	if err != nil {
		return err
	}

	upgrader := action.NewUpgrade(hc.actionConfig)
	upgrader.Namespace = hc.namespaceName
	upgrader.Timeout = HelmInstallTimeout
	// blocks until all podsPortsInfo are healthy
	upgrader.Wait = true

	if _, err := upgrader.Run(hc.ReleaseName, helmChart, hc.Values); err != nil {
		return err
	}
	if err := hc.enumerateApps(); err != nil {
		return err
	}
	if err := hc.fetchPods(); err != nil {
		return err
	}
	return hc.updateChartSettings()
}

// CopyToPod copies src to a particular container. Destination should be in the form of a proper K8s destination path
// NAMESPACE/POD_NAME:folder/FILE_NAME
func (hc *HelmChart) CopyToPod(src, destination, containername string) (*bytes.Buffer, *bytes.Buffer, *bytes.Buffer, error) {
	hc.env.k8sConfig.APIPath = "/api"
	hc.env.k8sConfig.GroupVersion = &schema.GroupVersion{Version: "v1"} // this targets the core api groups so the url path will be /api/v1
	hc.env.k8sConfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
	ioStreams, in, out, errOut := genericclioptions.NewTestIOStreams()

	copyOptions := cp.NewCopyOptions(ioStreams)
	copyOptions.Clientset = hc.env.k8sClient
	copyOptions.ClientConfig = hc.env.k8sConfig
	copyOptions.Container = containername
	copyOptions.Namespace = hc.env.Namespace

	formatted, err := regexp.MatchString(".*?\\/.*?\\:.*", destination)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Could not run copy operation: %v", err)
	}
	if !formatted {
		return nil, nil, nil, fmt.Errorf("Destination string improperly formatted, see reference 'NAMESPACE/POD_NAME:folder/FILE_NAME'")
	}

	log.Debug().
		Str("Namespace", hc.env.Namespace).
		Str("Chart", hc.ReleaseName).
		Str("Source", src).
		Str("Destination", destination).
		Str("Container", containername).
		Msg("Uploading file to pod")

	err = copyOptions.Run([]string{src, destination})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Could not run copy operation: %v", err)
	}
	return in, out, errOut, nil
}

// ExecuteInPod is similar to kubectl exec
func (hc *HelmChart) ExecuteInPod(podName string, containerName string, command []string) ([]byte, []byte, error) {
	req := hc.env.k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(hc.namespaceName).
		SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(hc.env.k8sConfig, "POST", req.URL())
	if err != nil {
		return []byte{}, []byte{}, err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if err != nil {
		return []byte{}, []byte{}, err
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

// GetPodsByNameSubstring retrieves all running pods whose names contain the provided substring
func (hc *HelmChart) GetPodsByNameSubstring(nameSubstring string) ([]v1.Pod, error) {
	if len(hc.podsList.Items) == 0 {
		return nil, errors.New("There are no pods in this chart")
	}
	pods := []v1.Pod{}
	for _, p := range hc.podsList.Items {
		if strings.Contains(p.Name, nameSubstring) {
			pods = append(pods, p)
		}
	}
	return pods, nil
}

func (hc *HelmChart) init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// TODO: So, this is annoying, and not really all that important, I SHOULD be able to just use our K8sConfig function
	// and pass that in as our config, but K8s has like 10 different config types, all of which don't talk to each other,
	// and this wants an interface, instead of the rest config that we use everywhere else. Creating such an interface is
	// also a huge hassle and... well anyway, if you've got some time to burn to make this more sensical, I hope you like
	// digging into K8s code with sparse to no docs.
	kubeConfigPath := filepath.Join(homeDir, DefaultK8sConfigPath)
	if len(os.Getenv("KUBECONFIG")) > 0 {
		kubeConfigPath = os.Getenv("KUBECONFIG")
	}
	hc.actionConfig = &action.Configuration{}
	if err := hc.actionConfig.Init(
		kube.GetConfig(kubeConfigPath, "", hc.namespaceName),
		hc.namespaceName,
		os.Getenv("HELM_DRIVER"),
		func(format string, v ...interface{}) {
			log.Info().Str("LogType", "Helm").Msg(fmt.Sprintf(format, v...))
		}); err != nil {
		return err
	}
	return nil
}

//getFSFiles gets files from selected FS
func (hc *HelmChart) getFSFiles(f fs.FS, root string) ([]string, error) {
	res := make([]string, 0)
	err := fs.WalkDir(f, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		res = append(res, path)
		return nil
	})
	return res, err
}

// loadEmbeddedChartFiles loads embedded chart files
func (hc *HelmChart) loadEmbeddedChartFiles() ([]*loader.BufferedFile, error) {
	log.Info().Str("Name", hc.ReleaseName).Msg("Resolving embedded FS chart")
	fpaths, err := hc.getFSFiles(ChartsFS, hc.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "chart: %s not found", hc.Path)
		}
		return nil, err
	}
	log.Debug().Interface("Paths", fpaths).Msg("Embedded charts paths")
	var bfs []*loader.BufferedFile
	for _, fpath := range fpaths {
		b, err := fs.ReadFile(ChartsFS, fpath)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read embedded file")
		}
		// must be resolved from the root for every chart where Chart.yaml is located
		fname := strings.TrimPrefix(fpath, hc.Path+"/")
		bf := &loader.BufferedFile{
			Name: fname,
			Data: b,
		}
		bfs = append(bfs, bf)
	}
	return bfs, nil
}

func (hc *HelmChart) loadChart() (*chart.Chart, error) {
	var err error
	var loadedChart *chart.Chart
	if hc.Path == "" {
		hc.Path = filepath.Join("charts", hc.ReleaseName)
	}
	log.Info().Str("Path", hc.Path).Msg("Searching chart")
	loadedChart, err = loader.Load(hc.Path)
	source := "host"
	if err != nil {
		source = "embedded"
		bfs, err := hc.loadEmbeddedChartFiles()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve embedded chart: %s", hc.Path)
		}
		loadedChart, err = loader.LoadFiles(bfs)
		if err != nil {
			return nil, errors.Wrapf(err, "faild to load embedded char files: %s", hc.Path)
		}
	}
	log.Info().Str("Path", hc.Path).
		Str("Source", source).
		Str("Release", hc.ReleaseName).
		Str("Namespace", hc.namespaceName).
		Interface("Overrides", hc.Values).
		Msg("Installing Helm chart")
	loadedChart.Values, err = chartutil.CoalesceValues(loadedChart, hc.Values)
	if err != nil {
		return nil, err
	}
	log.Debug().Interface("Values", loadedChart.Values).Msg("Merged chart values")
	return loadedChart, nil
}

// deployChart deploys the helm Charts
func (hc *HelmChart) deployChart() error {
	install := action.NewInstall(hc.actionConfig)
	install.Namespace = hc.namespaceName
	install.ReleaseName = hc.ReleaseName
	install.Timeout = HelmInstallTimeout
	// blocks until all podsPortsInfo are healthy
	install.Wait = true

	helmChart, err := hc.loadChart()
	if err != nil {
		return err
	}
	_, err = install.Run(helmChart, nil)
	if err != nil {
		return err
	}
	log.Info().
		Str("Namespace", hc.namespaceName).
		Str("Release", hc.ReleaseName).
		Str("HelmChart", hc.Path).
		Msg("Successfully installed helm chart")
	return nil
}

func (hc *HelmChart) downloadChart() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	downloadDir := filepath.Join(homeDir, ".helmenv")
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		if err := os.Mkdir(downloadDir, 0755); err != nil {
			return fmt.Errorf("failed to create helmenv directory: %v", err)
		}
	}
	chartURL, err := url.Parse(hc.URL)
	if err != nil {
		log.Error().Err(err).Msg("Invalid URL given for the Helm chart")
	}

	fileName := path.Base(chartURL.Path)
	filePath := filepath.Join(downloadDir, fileName)
	if _, err := os.Stat(filePath); err == nil {
		log.Debug().Str("URL", hc.URL).Msg("Chart already downloaded")
		hc.Path = filePath
		return nil
	}

	log.Info().Str("URL", hc.URL).Msg("Downloading Helm chart from repository")
	resp, err := grab.Get(downloadDir, hc.URL)
	if err != nil {
		return fmt.Errorf("failed to download Helm chart: %v", err)
	}
	hc.Path = resp.Filename
	return nil
}

func (hc *HelmChart) fetchPods() error {
	var err error
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.namespaceName)
	hc.podsList, err = k8sPods.List(context.Background(), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("release=%s", hc.ReleaseName),
	})
	if err != nil {
		return err
	}
	return nil
}

func (hc *HelmChart) addInstanceLabel(app string) error {
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.namespaceName)
	l, err := k8sPods.List(context.Background(), metaV1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", app)})
	if err != nil {
		return err
	}
	// Ensures that instance label applies in order to port mapping
	sort.Slice(l.Items, func(i, j int) bool {
		return l.Items[i].Status.PodIP < l.Items[j].Status.PodIP
	})
	for i, pod := range l.Items {
		labelPatch := fmt.Sprintf(`[{"op":"add","path":"/metadata/labels/%s","value":"%d" }]`, "instance", i)
		_, err := k8sPods.Patch(context.Background(), pod.GetName(), types.JSONPatchType, []byte(labelPatch), metaV1.PatchOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to update labels %s for pod %s", labelPatch, pod.Name)
		}
	}
	return nil
}

func (hc *HelmChart) updateChartSettings() error {
	for _, p := range hc.podsList.Items {
		for _, c := range p.Spec.Containers {
			app, ok := p.Labels[AppEnumerationLabelKey]
			if !ok {
				log.Warn().Str("Container", c.Name).Msg("App label not found")
			}
			instance := p.Labels[InstanceEnumerationLabelKey]
			if !ok {
				log.Warn().Str("Container", c.Name).Msg("Instance label not found")
			}
			log.Info().
				Str("Container", c.Name).
				Interface("PodPorts", c.Ports).
				Msg("Container info")
			pm := map[string]int{}
			for _, port := range c.Ports {
				pm[port.Name] = int(port.ContainerPort)
			}
			if err := hc.ChartConnections.Store(app, instance, c.Name, &ChartConnection{
				PodName:     p.Name,
				PodIP:       p.Status.PodIP,
				RemotePorts: pm,
				LocalPorts:  make(map[string]int),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (hc *HelmChart) enumerateApps() error {
	apps, err := hc.uniqueAppLabels(AppEnumerationLabelKey)
	if err != nil {
		return err
	}
	for _, app := range apps {
		if err := hc.addInstanceLabel(app); err != nil {
			return err
		}
	}
	return nil
}

func (hc *HelmChart) uniqueAppLabels(selector string) ([]string, error) {
	uniqueLabels := make([]string, 0)
	isUnique := make(map[string]bool)
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.namespaceName)
	podList, err := k8sPods.List(context.Background(), metaV1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "no labels with selector %s found for enumeration", selector)
	}
	for _, p := range podList.Items {
		appLabel := p.Labels[AppEnumerationLabelKey]
		if _, ok := isUnique[appLabel]; !ok {
			uniqueLabels = append(uniqueLabels, appLabel)
		}
	}
	log.Info().
		Interface("AppLabels", uniqueLabels).
		Msg("Apps found")
	return uniqueLabels, nil
}

func (hc *HelmChart) makePortRules(chartConnection *ChartConnection) ([]string, error) {
	rules := make([]string, 0)
	for portName, port := range chartConnection.RemotePorts {
		if portName == "" {
			return nil, fmt.Errorf("port %d must be named in helm chart", port)
		}
		rules = append(rules, fmt.Sprintf(":%d", port))
	}
	return rules, nil
}

func (hc *HelmChart) connectPod(connectionInfo *ChartConnection, rules []string) error {
	if len(rules) == 0 {
		return nil
	}
	return hc.env.runGoForwarder(connectionInfo, rules, time.Second*30)
}
