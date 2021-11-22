package environment

import (
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
	"math/rand"
	"os"
	"path/filepath"
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

// HelmChart represents a single Helm chart to be installed into a cluster
type HelmChart struct {
	ReleaseName      string                 `yaml:"release_name,omitempty" json:"release_name,omitempty" envconfig:"release_name"`
	Path             string                 `yaml:"path,omitempty" json:"path,omitempty" envconfig:"path"`
	Values           map[string]interface{} `yaml:"values,omitempty" json:"values,omitempty" envconfig:"values"`
	Index            int                    `yaml:"index,omitempty" json:"index,omitempty" envconfig:"index"`
	ChartConnections ChartConnections       `yaml:"chart_connections,omitempty" json:"chart_connections,omitempty" envconfig:"chart_connections"`

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
		kube.GetConfig(kubeConfigPath, "", hc.namespaceName),
		hc.namespaceName,
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
	log.Info().Str("Path", hc.Path).
		Str("Release", hc.ReleaseName).
		Str("Namespace", hc.namespaceName).
		Interface("Override values", hc.Values).
		Msg("Installing Helm chart")
	chart, err := loader.Load(hc.Path)
	if err != nil {
		return err
	}

	chart.Values, err = chartutil.CoalesceValues(chart, hc.Values)
	if err != nil {
		return err
	}
	log.Debug().Interface("Values", chart.Values).Msg("Merged chart values")

	install := action.NewInstall(hc.actionConfig)
	install.Namespace = hc.namespaceName
	install.ReleaseName = hc.ReleaseName
	install.Timeout = HelmInstallTimeout
	// blocks until all podsPortsInfo are healthy
	install.Wait = true
	_, err = install.Run(chart, nil)
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
		freePort := rand.Intn(MaxPort-MinPort) + MinPort
		if portName == "" {
			return nil, fmt.Errorf("port %d must be named in helm chart", port)
		}
		if chartConnection.LocalPorts == nil {
			chartConnection.LocalPorts = map[string]int{}
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
	return hc.env.runGoForwarder(connectionInfo.PodName, rules)
}
