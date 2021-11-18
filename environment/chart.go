package environment

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/kube"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// MaxPort max port value for forwarding
	MaxPort = 50000
	// MinPort min port value for forwarding
	MinPort = 20000
	// AppEnumerationLabelKey label used to enumerate instances of the same app to ease access
	AppEnumerationLabelKey = "app"
	// InstanceEnumerationLabelKey additional label to enumerate app instances
	InstanceEnumerationLabelKey = "instance"
)

// Chart represents a single Helm chart to be installed into a cluster
type Chart struct {
	ReleaseName      string                 `json:"release_name,omitempty"`
	Path             string                 `json:"path,omitempty"`
	OverrideValues   map[string]interface{} `json:"override_values,omitempty"`
	ChartConnections ChartConnections       `json:"chart_connections,omitempty"`
}

// HelmChart helm chart structure
type HelmChart struct {
	Name          string
	NamespaceName string

	settings      *Chart
	env           *Environment
	actionConfig  *action.Configuration
	podsList      *v1.PodList
}

// NewHelmChart creates a new Helm chart wrapper
func NewHelmChart(env *Environment, chart *Chart) (*HelmChart, error) {
	if chart.ChartConnections == nil {
		chart.ChartConnections = ChartConnections{}
	}
	hc := &HelmChart{
		Name:          chart.ReleaseName,
		env:           env,
		settings:      chart,
		NamespaceName: env.Config.NamespaceName,
	}
	if err := hc.init(); err != nil {
		return nil, err
	}
	return hc, nil
}

// Connect connects to all exposed containerPorts, forwards them to local
func (hc *HelmChart) Connect() error {
	var rangeErr error
	hc.settings.ChartConnections.Range(func(key string, chartConnection *ChartConnection) bool {
		if chartConnection.ForwarderPID != 0 {
			log.Info().
				Str("Pod", chartConnection.PodName).
				Interface("Ports", chartConnection.LocalPorts).
				Msg("Already connected")
			return true
		}
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
	return nil
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
		kube.GetConfig(kubeConfigPath, "", hc.NamespaceName),
		hc.NamespaceName,
		os.Getenv("HELM_DRIVER"),
		func(format string, v ...interface{}) {
			log.Debug().Str("LogType", "Helm").Msg(fmt.Sprintf(format, v...))
		}); err != nil {
		return err
	}
	return nil
}

// deployChart deploys the helm Charts
func (hc *HelmChart) deployChart() error {
	log.Info().Str("Path", hc.settings.Path).
		Str("Release", hc.settings.ReleaseName).
		Str("Namespace", hc.NamespaceName).
		Interface("Override values", hc.settings.OverrideValues).
		Msg("Installing Helm chart")
	chart, err := loader.Load(hc.settings.Path)
	if err != nil {
		return err
	}

	chart.Values, err = chartutil.CoalesceValues(chart, hc.settings.OverrideValues)
	if err != nil {
		return err
	}
	log.Debug().Interface("Values", chart.Values).Msg("Merged chart values")

	install := action.NewInstall(hc.actionConfig)
	install.Namespace = hc.NamespaceName
	install.ReleaseName = hc.settings.ReleaseName
	install.Timeout = HelmInstallTimeout
	// blocks until all podsPortsInfo are healthy
	install.Wait = true
	_, err = install.Run(chart, nil)
	if err != nil {
		return err
	}
	log.Info().
		Str("Namespace", hc.NamespaceName).
		Str("Release", hc.settings.ReleaseName).
		Str("Chart", hc.settings.Path).
		Msg("Successfully installed helm chart")
	return nil
}

func (hc *HelmChart) fetchPods() error {
	var err error
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.NamespaceName)
	hc.podsList, err = k8sPods.List(context.Background(), metaV1.ListOptions{
		LabelSelector: fmt.Sprintf("release=%s", hc.settings.ReleaseName),
	})
	if err != nil {
		return err
	}
	return nil
}

func (hc *HelmChart) addInstanceLabel(app string) error {
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.NamespaceName)
	l, err := k8sPods.List(context.Background(), metaV1.ListOptions{LabelSelector: fmt.Sprintf("app=%s", app)})
	if err != nil {
		return err
	}
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
			if err := hc.settings.ChartConnections.Store(app, instance, c.Name, &ChartConnection{
				PodName:    p.Name,
				PodIP:      p.Status.PodIP,
				Ports:      pm,
				LocalPorts: make(map[string]int),
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
	k8sPods := hc.env.k8sClient.CoreV1().Pods(hc.NamespaceName)
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

func (k *Environment) killForwarder(pid int) error {
	// 0 means no process
	// -1 can be set after detachment, don't try to kill it
	if pid == 0 || pid == -1 {
		return nil
	}
	cmd := exec.Command(
		"kill",
		"-9",
		strconv.Itoa(pid),
	)
	err := cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "failed to kill forwarder with pid: %d", pid)
	}
	return nil
}

func forkProcess(processName string, args []string) (int, error) {
	fork := NewForkProcess(
		os.Stdin, os.Stdout, os.Stderr,
		uint32(os.Getuid()), uint32(os.Getgid()), "/")
	pid, err := fork.Exec(processName, args)
	if err != nil {
		return 0, errors.Wrapf(err, "unable to fork process")
	}
	return pid, nil
}

// runProcessForwarder forking "kubectl port-forward" command and gets PID
func (k *Environment) runProcessForwarder(podName string, portRules []string) (int, error) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return 0, fmt.Errorf("kubectl doesn't appear to be installed on this machine: %v", err)
	}

	portRulesStr := strings.Join(portRules, " ")
	processArgs := []string{
		kubectlPath,
		"-n",
		k.Config.NamespaceName,
		"port-forward",
		fmt.Sprintf("%s/%s", "pods", podName),
	}
	processArgs = append(processArgs, portRules...)
	log.Debug().
		Str("Args", strings.Join(processArgs, " ")).
		Msg("Process args")
	pid, err := forkProcess(kubectlPath, processArgs)
	if err != nil {
		return 0, err
	}
	log.Debug().
		Interface("Ports", portRulesStr).
		Msg("Forwarded ports")
	return pid, nil
}

// runGoForwarder runs port forwarder as a goroutine
func (k *Environment) runGoForwarder(podName string, portRules []string) error {
	roundTripper, upgrader, err := spdy.RoundTripperFor(k.k8sConfig)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", k.Config.NamespaceName, podName)
	hostIP := strings.TrimLeft(k.k8sConfig.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: path, Host: hostIP}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: roundTripper}, http.MethodPost, &serverURL)

	stopChan, readyChan := make(chan struct{}, 1), make(chan struct{}, 1)
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)

	log.Debug().
		Str("Pod", podName).
		Msg("Attempting to forward port")

	forwarder, err := portforward.New(dialer, portRules, stopChan, readyChan, out, errOut)
	if err != nil {
		return err
	}
	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			log.Error().Str("Pod", podName).Err(err)
		}
	}()

	<-readyChan
	if len(errOut.String()) > 0 {
		return fmt.Errorf("error on forwarding k8s port: %v", errOut.String())
	}
	if len(out.String()) > 0 {
		msg := strings.ReplaceAll(out.String(), "\n", " ")
		log.Debug().Str("Pod", podName).Msgf("%s", msg)
	}
	return nil
}

func (hc *HelmChart) makePortRules(chartConnection *ChartConnection) ([]string, error) {
	rules := make([]string, 0)
	for portName, port := range chartConnection.Ports {
		freePort := rand.Intn(MaxPort-MinPort) + MinPort
		if portName == "" {
			return nil, fmt.Errorf("port %d must be named in helm chart", port)
		}
		chartConnection.LocalPorts[portName] = freePort
		rules = append(rules, fmt.Sprintf("%d:%d", freePort, port))
	}
	return rules, nil
}

func (hc *HelmChart) connectPod(connectionInfo *ChartConnection, rules []string) error {
	if len(rules) == 0 {
		return nil
	}
	if !hc.env.Config.PersistentConnection {
		if err := hc.env.runGoForwarder(connectionInfo.PodName, rules); err != nil {
			return err
		}
		return nil
	}
	pid, err := hc.env.runProcessForwarder(connectionInfo.PodName, rules)
	if err != nil {
		return err
	}
	connectionInfo.ForwarderPID = pid
	return nil
}
