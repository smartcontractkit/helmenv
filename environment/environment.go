package environment

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/smartcontractkit/helmenv/chaos"
	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/cli"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"

	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/action"
)

const (
	// HelmInstallTimeout timeout for installing a helm chart
	HelmInstallTimeout = 200 * time.Second
	// DefaultK8sConfigPath the default path for kube
	DefaultK8sConfigPath = ".kube/config"
)

// Environment build and deployed from several helm Charts
type Environment struct {
	*Config
	Artifacts *Artifacts
	Chaos     *chaos.Controller

	k8sClient  *kubernetes.Clientset
	k8sConfig  *rest.Config
	forwarders []*portforward.PortForwarder
}

// NewEnvironment creates new environment from charts
func NewEnvironment(config *Config) (*Environment, error) {
	ks, kc, err := GetLocalK8sDeps()
	if err != nil {
		return nil, err
	}
	if config.Charts == nil {
		config.Charts = map[string]*HelmChart{}
	}
	he := &Environment{
		Config:    config,
		k8sClient: ks,
		k8sConfig: kc,
	}
	return he, nil
}

// DeployEnvironment returns a deployed environment from a given config that can be pre-defined within
// the library, or passed in as part of lib usage
func DeployEnvironment(config *Config, chartDirectory string) (*Environment, error) {
	e, err := NewEnvironment(config)
	if err != nil {
		return nil, err
	}
	if err := e.Init(config.NamespacePrefix); err != nil {
		return nil, err
	}
	for key, chart := range config.Charts {
		if len(chart.Path) == 0 {
			chart.Path = key
		}
		if len(chart.ReleaseName) == 0 {
			chart.ReleaseName = key
		}
		if len(chartDirectory) > 0 && !strings.Contains(chart.Path, chartDirectory) {
			chart.Path = path.Join(chartDirectory, chart.Path)
		}
		if err := e.AddChart(chart); err != nil {
			return nil, err
		}
	}
	if err := e.DeployAll(); err != nil {
		log.Error().Err(err).Msg("Error while deploying the environment")
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, e.SyncConfig()
}

// LoadEnvironment loads an already deployed environment from config
func LoadEnvironment(config *Config) (*Environment, error) {
	log.Info().
		Interface("Namespace", config.Namespace).
		Msg("Loading environment")
	environment, err := NewEnvironment(config)
	if err != nil {
		return nil, err
	}
	if err := environment.configureHelm(); err != nil {
		return nil, err
	}
	artifacts, err := NewArtifacts(environment)
	if err != nil {
		return nil, err
	}
	environment.Artifacts = artifacts
	cc, err := chaos.NewController(&chaos.Config{
		Client:        environment.k8sClient,
		NamespaceName: config.Namespace,
	})
	if err != nil {
		return nil, err
	}
	environment.Chaos = cc
	for _, chart := range environment.Config.Charts {
		if err := chart.Init(environment); err != nil {
			return environment, err
		}
	}
	return environment, nil
}

// GetLocalK8sDeps get local k8s connection deps
func GetLocalK8sDeps() (*kubernetes.Clientset, *rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	k8sConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, nil, err
	}
	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, nil, err
	}
	return k8sClient, k8sConfig, nil
}

// Disconnect closes any current open port forwarder rules
func (k *Environment) Disconnect() {
	log.Info().Str("Namespace", k.Namespace).Msg("Disconnecting all open forwarded ports")
	for _, forwarder := range k.forwarders {
		forwarder.Close()
	}
	return
}

// Teardown tears down the helm releases
func (k *Environment) Teardown() error {
	k.Disconnect()
	for _, c := range k.Charts {
		log.Debug().Str("Release", c.ReleaseName).Msg("Uninstalling Helm release")
		if _, err := action.NewUninstall(c.actionConfig).Run(c.ReleaseName); err != nil {
			return err
		}
	}
	if err := k.removeNamespace(); err != nil {
		return err
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// DeferTeardown wraps teardown and logs on error, to be used in deferred function calls
func (k *Environment) DeferTeardown() {
	if err := k.Teardown(); err != nil {
		log.Error().Err(err)
	}
}

// Init inits namespace for an env and configure helm for k8s and that namespace
func (k *Environment) Init(namespacePrefix string) error {
	if len(namespacePrefix) == 0 {
		return fmt.Errorf("namespace_prefix cannot be empty, exiting")
	}
	if err := k.createNamespace(namespacePrefix); err != nil {
		return err
	}
	if err := k.configureHelm(); err != nil {
		return err
	}
	a, err := NewArtifacts(k)
	if err != nil {
		return err
	}
	k.Artifacts = a
	cc, err := chaos.NewController(&chaos.Config{
		Client:        k.k8sClient,
		NamespaceName: k.Config.Namespace,
	})
	if err != nil {
		return err
	}
	k.Chaos = cc
	return nil
}

// ClearConfig resets the config so only the preset config remains
func (k *Environment) ClearConfig() error {
	for _, chart := range k.Charts {
		chart.ChartConnections = nil
	}
	k.Namespace = ""
	if err := DumpConfig(k.Config, k.Path); err != nil {
		return err
	}
	return nil
}

// ClearConfigLocalPorts removes the local ports set within config
func (k *Environment) ClearConfigLocalPorts() error {
	for _, chart := range k.Charts {
		chart.ChartConnections.Range(func(_ string, chartConnection *ChartConnection) bool {
			chartConnection.LocalPorts = nil
			return true
		})
	}
	if err := DumpConfig(k.Config, k.Path); err != nil {
		return err
	}
	return nil
}

// SyncConfig dumps config in Persistent mode
func (k *Environment) SyncConfig() error {
	if k.Config.Persistent {
		if len(k.Path) == 0 {
			k.Path = fmt.Sprintf("%s.yaml", k.Namespace)
		}
		if err := DumpConfig(k.Config, k.Path); err != nil {
			return err
		}
	}
	return nil
}

// Deploy a single chart
func (k *Environment) Deploy(chartName string) error {
	var chart *HelmChart
	for _, c := range k.Charts {
		if c.ReleaseName == chartName {
			chart = c
			break
		}
	}
	if chart == nil {
		return fmt.Errorf("chart %s doesn't exist", chartName)
	}
	return chart.Deploy()
}

// DeployAll deploys all deploy sequence at once
func (k *Environment) DeployAll() error {
	for _, keySlice := range k.Charts.OrderedKeys() {
		group := &errgroup.Group{}
		for _, key := range keySlice {
			chart, ok := k.Charts[key]
			if !ok {
				continue
			}
			group.Go(chart.Deploy)
		}
		if err := group.Wait(); err != nil {
			return err
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// AddChart adds chart to deploy
func (k *Environment) AddChart(chart *HelmChart) error {
	if chart.Index == 0 {
		return fmt.Errorf("chart index cannot be 0")
	}
	if err := chart.Init(k); err != nil {
		return err
	}
	k.Charts[chart.ReleaseName] = chart
	return nil
}

// Connect to a single chart
func (k *Environment) Connect(chartName string) error {
	var chart *HelmChart
	for _, c := range k.Charts {
		if c.ReleaseName == chartName {
			chart = c
			break
		}
	}
	if chart == nil {
		return fmt.Errorf("chart %s doesn't exist", chartName)
	}
	return chart.Connect()
}

// ConnectAll connects to all containerPorts for all charts, dump config in JSON if Persistent flag is present
func (k *Environment) ConnectAll() error {
	for _, c := range k.Charts {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// GetSecretField retrieves field data from k8s secret
func (k *Environment) GetSecretField(namespace string, secretName string, fieldName string) (string, error) {
	res, err := k.k8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metaV1.GetOptions{})
	log.Debug().Interface("Data", res.Data).Send()
	if err != nil {
		return "", err
	}
	return string(res.Data[fieldName]), nil
}

func (k *Environment) createNamespace(namespacePrefix string) error {
	log.Info().Str("Namespace Prefix", namespacePrefix).Msg("Creating environment")
	ns, err := k.k8sClient.CoreV1().Namespaces().Create(
		context.Background(),
		&v1.Namespace{
			ObjectMeta: metaV1.ObjectMeta{
				GenerateName: namespacePrefix + "-",
			},
		},
		metaV1.CreateOptions{},
	)
	if err != nil {
		return err
	}
	k.Config.Namespace = ns.Name
	log.Info().Str("Namespace", k.Config.Namespace).Msg("Created namespace")
	return nil
}

func (k *Environment) configureHelm() error {
	if err := os.Setenv("HELM_NAMESPACE", k.Config.Namespace); err != nil {
		return err
	}
	settings := cli.New()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settings.KubeConfig = filepath.Join(homeDir, DefaultK8sConfigPath)
	return nil
}

func (k *Environment) removeNamespace() error {
	log.Info().
		Str("Namespace", k.Config.Namespace).
		Msg("Deleting namespace")
	if err := k.k8sClient.CoreV1().Namespaces().Delete(
		context.Background(),
		k.Config.Namespace,
		metaV1.DeleteOptions{},
	); err != nil {
		return err
	}
	return nil
}

// runGoForwarder runs port forwarder as a goroutine
func (k *Environment) runGoForwarder(chartConnection *ChartConnection, portRules []string) error {
	podName := chartConnection.PodName
	roundTripper, upgrader, err := spdy.RoundTripperFor(k.k8sConfig)
	if err != nil {
		return err
	}
	httpPath := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", k.Config.Namespace, podName)
	hostIP := strings.TrimLeft(k.k8sConfig.Host, "htps:/")
	serverURL := url.URL{Scheme: "https", Path: httpPath, Host: hostIP}

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
		log.Info().Str("Pod", podName).Msgf("%s", msg)
	}
	k.forwarders = append(k.forwarders, forwarder)
	forwardedPorts, err := forwarder.GetPorts()
	if err != nil {
		return err
	}
	for portName, port := range chartConnection.RemotePorts {
		for _, forwardedPort := range forwardedPorts {
			fpr := int(forwardedPort.Remote)
			if port == fpr {
				if chartConnection.LocalPorts == nil {
					chartConnection.LocalPorts = map[string]int{}
				}
				fpl := int(forwardedPort.Local)
				chartConnection.LocalPorts[portName] = fpl
			}
		}
	}
	return nil
}
