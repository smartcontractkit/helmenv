package environment

import (
	"context"
	"fmt"
	"github.com/smartcontractkit/helmenv/chaos"
	"helm.sh/helm/v3/pkg/cli"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"time"

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
	Artifacts *Artifacts
	Chaos     *chaos.Controller
	Config    *Config

	charts    []*HelmChart
	k8sClient *kubernetes.Clientset
	k8sConfig *rest.Config
}

// NewEnvironment creates new environment from charts
func NewEnvironment(config *Config) (*Environment, error) {
	ks, kc, err := GetLocalK8sDeps()
	if err != nil {
		return nil, err
	}
	if config.Charts == nil {
		config.Charts = map[string]*Chart{}
	}
	he := &Environment{
		Config:    config,
		k8sClient: ks,
		k8sConfig: kc,
	}
	return he, nil
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

// Teardown tears down the helm releases
func (k *Environment) Teardown() error {
	if err := k.Disconnect(); err != nil {
		return err
	}
	for _, c := range k.charts {
		log.Debug().Str("Release", c.Name).Msg("Uninstalling Helm release")
		if _, err := action.NewUninstall(c.actionConfig).Run(c.Name); err != nil {
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
		NamespaceName: k.Config.NamespaceName,
	})
	if err != nil {
		return err
	}
	k.Chaos = cc
	return nil
}

// RemoveConfigConnectionInfo removes config connection info when environment was removed
func (k *Environment) RemoveConfigConnectionInfo() error {
	if k.Config.Persistent {
		k.Config.Charts = nil
		k.Config.NamespaceName = ""
	}
	if err := DumpConfig(k.Config, fmt.Sprintf("%s.yaml", k.Config.NamespaceName)); err != nil {
		return err
	}
	return nil
}

// SyncConfig dumps config in Persistent mode
func (k *Environment) SyncConfig() error {
	if k.Config.Persistent {
		if err := DumpConfig(k.Config, fmt.Sprintf("%s.yaml", k.Config.NamespaceName)); err != nil {
			return err
		}
	}
	return nil
}

// Deploy a single chart
func (k *Environment) Deploy(chartName string) error {
	var chart *HelmChart
	for _, c := range k.charts {
		if c.Name == chartName {
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
	for _, c := range k.charts {
		if err := c.Deploy(); err != nil {
			return err
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// AddChart adds chart to deploy
func (k *Environment) AddChart(chart *Chart) error {
	k.Config.Charts[chart.ReleaseName] = chart
	hc, err := NewHelmChart(k, chart)
	if err != nil {
		return err
	}
	k.charts = append(k.charts, hc)
	return nil
}

// Connect to a single chart
func (k *Environment) Connect(chartName string) error {
	var chart *HelmChart
	for _, c := range k.charts {
		if c.Name == chartName {
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
	for _, c := range k.charts {
		if err := c.Connect(); err != nil {
			return err
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// Disconnect disconnects from all deployed charts, only working in Persistent mode
func (k *Environment) Disconnect() error {
	for _, c := range k.charts {
		log.Info().
			Str("Release", c.Name).
			Msg("Disconnecting")
		var rangeErr error
		c.settings.ChartConnections.Range(func(key string, chartConnection *ChartConnection) bool {
			if err := k.killForwarder(chartConnection.ForwarderPID); err != nil {
				rangeErr = err
				return false
			}
			chartConnection.ForwarderPID = 0
			chartConnection.LocalPorts = make(map[string]int)
			return true
		})
		if rangeErr != nil {
			return rangeErr
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// LoadPersistentEnvironment loads environment from config
func LoadPersistentEnvironment(filePath string) (*Environment, error) {
	cfg, err := LoadConfig(filePath)
	if err != nil {
		return nil, err
	}
	log.Info().
		Interface("Settings", cfg).
		Msg("Loading environment")
	environment, err := NewEnvironment(cfg)
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
		NamespaceName: cfg.NamespaceName,
	})
	if err != nil {
		return nil, err
	}
	environment.Chaos = cc
	for _, chart := range environment.Config.Charts {
		hc, err := NewHelmChart(environment, chart)
		if err != nil {
			return nil, err
		}
		environment.charts = append(environment.charts, hc)
	}
	return environment, nil
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
	k.Config.NamespaceName = ns.Name
	log.Info().Str("Namespace", k.Config.NamespaceName).Msg("Created namespace")
	return nil
}

func (k *Environment) configureHelm() error {
	if err := os.Setenv("HELM_NAMESPACE", k.Config.NamespaceName); err != nil {
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
		Str("Namespace", k.Config.NamespaceName).
		Msg("Shutting down environment")
	if err := k.k8sClient.CoreV1().Namespaces().Delete(
		context.Background(),
		k.Config.NamespaceName,
		metaV1.DeleteOptions{},
	); err != nil {
		return err
	}
	return nil
}
