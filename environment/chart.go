package environment

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
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
	"strconv"
	"strings"

	"helm.sh/helm/v3/pkg/action"
)

const (
	// MaxPort max port value for forwarding
	MaxPort = 50000
	// MinPort min port value for forwarding
	MinPort = 20000
	// AppEnumerationLabelKey label used to enumerate instances of the same app to ease access
	AppEnumerationLabelKey = "app"
)

// ChartSettings chart settings config
type ChartSettings struct {
	ReleaseName    string                     `json:"release_name"`
	Path           string                     `json:"path"`
	Values         map[string]interface{}     `json:"values"`
	ConnectionInfo map[string]*ConnectionInfo `json:"pods_info"`
}

// HelmChart helm chart structure
type HelmChart struct {
	Name          string
	NamespaceName string
	settings      *ChartSettings
	env           *Environment
	actionConfig  *action.Configuration
	podsList      *v1.PodList
}

// NewHelmChart creates a new Helm chart wrapper
func NewHelmChart(env *Environment, cfg *ChartSettings) (*HelmChart, error) {
	hc := &HelmChart{
		Name:          cfg.ReleaseName,
		env:           env,
		settings:      cfg,
		NamespaceName: env.Config.NamespaceName,
	}
	if err := hc.initAction(); err != nil {
		return nil, err
	}
	return hc, nil
}

func (hc *HelmChart) makePortRules(connectionInfo *ConnectionInfo) ([]string, error) {
	rules := make([]string, 0)
	for portName, port := range connectionInfo.Ports {
		freePort := rand.Intn(MaxPort-MinPort) + MinPort
		if portName == "" {
			return nil, fmt.Errorf("port %d must be named in helm chart", port)
		}
		connectionInfo.LocalPorts[portName] = freePort
		rules = append(rules, fmt.Sprintf("%d:%d", freePort, port))
	}
	return rules, nil
}

func (hc *HelmChart) connectPod(connectionInfo *ConnectionInfo, rules []string) error {
	if len(rules) == 0 {
		return nil
	}
	if !hc.env.Config.Persistent {
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

// Connect connects to all exposed containerPorts, forwards them to local
func (hc *HelmChart) Connect() error {
	for _, connectionInfo := range hc.settings.ConnectionInfo {
		if connectionInfo.ForwarderPID != 0 {
			log.Info().
				Str("Pod", connectionInfo.PodName).
				Interface("Ports", connectionInfo.LocalPorts).
				Msg("Already connected")
			continue
		}
		rules, err := hc.makePortRules(connectionInfo)
		if err != nil {
			return err
		}
		if err := hc.connectPod(connectionInfo, rules); err != nil {
			return err
		}
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

func (hc *HelmChart) initAction() error {
	hc.actionConfig = &action.Configuration{}
	if err := hc.actionConfig.Init(
		hc.env.CLISettings.RESTClientGetter(),
		hc.NamespaceName,
		"configmap",
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
		Msg("Installing Helm chart")
	chart, err := loader.Load(hc.settings.Path)
	if err != nil {
		return err
	}

	chart.Values, err = chartutil.CoalesceValues(chart, hc.settings.Values)
	if err != nil {
		return err
	}

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
		Msg("Succesfully installed helm chart")
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
			if hc.settings.ConnectionInfo == nil {
				hc.settings.ConnectionInfo = make(map[string]*ConnectionInfo)
			}
			app, ok := p.Labels[AppEnumerationLabelKey]
			if !ok {
				log.Warn().Str("Container", c.Name).Msg("App label not found")
			}
			instance := p.Labels["instance"]
			if !ok {
				log.Warn().Str("Container", c.Name).Msg("Instance label not found")
			}
			log.Info().
				Str("Container", c.Name).
				Interface("PodPorts", c.Ports).
				Msg("Container info")
			instanceKey := fmt.Sprintf("%s:%s:%s", app, instance, c.Name)
			if _, ok := hc.settings.ConnectionInfo[instanceKey]; ok {
				return fmt.Errorf("ambiguous instance key: %s", instanceKey)
			}
			pm := map[string]int{}
			for _, port := range c.Ports {
				pm[port.Name] = int(port.ContainerPort)
			}
			hc.settings.ConnectionInfo[instanceKey] = &ConnectionInfo{
				PodName:    p.Name,
				PodIP:      p.Status.PodIP,
				Ports:      pm,
				LocalPorts: make(map[string]int),
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
	portRulesStr := strings.Join(portRules, " ")
	processArgs := []string{
		k.Config.KubeCtlProcessName,
		"-n",
		k.Config.NamespaceName,
		"port-forward",
		fmt.Sprintf("%s/%s", "pods", podName),
	}
	processArgs = append(processArgs, portRules...)
	log.Debug().
		Str("Args", strings.Join(processArgs, " ")).
		Msg("Process args")
	pid, err := forkProcess(k.Config.KubeCtlProcessName, processArgs)
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

	log.Debug().Str("Pod", podName).Msg("Waiting on podsPortsInfo forwarded forwardingRuleStrings to be ready")
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
