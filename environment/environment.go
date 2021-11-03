package environment

import (
	"context"
	"fmt"
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

// ConnectionInfo info about connected pod ports
type ConnectionInfo struct {
	PodName      string         `json:"pod_name"`
	ForwarderPID int            `json:"forwarder_pid"`
	PodIP        string         `json:"pod_ip"`
	Ports        map[string]int `json:"ports"`
	LocalPorts   map[string]int `json:"local_port"`
}

// HelmEnvironmentConfig environment config with all charts info
type HelmEnvironmentConfig struct {
	Persistent         bool                      `json:"persistent"`
	KubeCtlProcessName string                    `json:"kube_ctl_process_name"`
	NamespaceName      string                    `json:"namespace_name"`
	Name               string                    `json:"name"`
	ChartsInfo         map[string]*ChartSettings `json:"chart_settings"`
}

// Environment environment build and deployed from several helm Charts
type Environment struct {
	CLISettings          *cli.EnvSettings
	Config               *HelmEnvironmentConfig
	releaseName          string
	Charts               map[string]*HelmChart
	chartsDeploySequence []*HelmChart
	k8sClient            *kubernetes.Clientset
	k8sConfig            *rest.Config
}

// NewEnvironment creates new environment from charts
func NewEnvironment(cfg *HelmEnvironmentConfig) (*Environment, error) {
	ks, kc, err := GetLocalK8sDeps()
	if err != nil {
		return nil, err
	}
	if cfg.ChartsInfo == nil {
		cfg.ChartsInfo = map[string]*ChartSettings{}
	}
	he := &Environment{
		Config:               cfg,
		releaseName:          cfg.Name,
		k8sClient:            ks,
		k8sConfig:            kc,
		Charts:               make(map[string]*HelmChart),
		chartsDeploySequence: make([]*HelmChart, 0),
	}
	return he, nil
}

func (k *Environment) createNamespace() error {
	var err error
	log.Info().Str("Namespace", k.releaseName).Msg("Creating environment")
	ns, err := k.k8sClient.CoreV1().Namespaces().Create(
		context.Background(),
		&v1.Namespace{
			ObjectMeta: metaV1.ObjectMeta{
				GenerateName: k.releaseName + "-",
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
	for _, c := range k.Charts {
		log.Debug().Str("Release", c.Name).Msg("Uninstalling Helm release")
		if _, err := action.NewUninstall(c.actionConfig).Run(c.Name); err != nil {
			return err
		}
	}
	if err := k.removeNamespace(); err != nil {
		return err
	}
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
	k.CLISettings = settings
	return nil
}

// Init inits namespace for an env and configure helm for k8s and that namespace
func (k *Environment) Init() error {
	if err := k.createNamespace(); err != nil {
		return err
	}
	if err := k.configureHelm(); err != nil {
		return err
	}
	return nil
}

// SyncConfig dumps config in Persistent mode
func (k *Environment) SyncConfig() error {
	if k.Config.Persistent {
		if err := DumpConfigJSON(k.Config, fmt.Sprintf("%s.json", k.releaseName)); err != nil {
			return err
		}
	}
	return nil
}

// DeployAll deploys all deploy sequence at once
func (k *Environment) DeployAll() error {
	for _, c := range k.chartsDeploySequence {
		if err := c.Deploy(); err != nil {
			return err
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

func (k *Environment) removeNamespace() error {
	log.Info().
		Str("Namespace", k.Config.NamespaceName).
		Msg("Shutting down environment")
	if err := k.k8sClient.CoreV1().Namespaces().Delete(context.Background(), k.Config.NamespaceName, metaV1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// AddChart adds chart to deploy
func (k *Environment) AddChart(settings *ChartSettings) error {
	k.Config.ChartsInfo[settings.ReleaseName] = settings
	hc, err := NewHelmChart(k, settings)
	if err != nil {
		return err
	}
	k.Charts[settings.ReleaseName] = hc
	k.chartsDeploySequence = append(k.chartsDeploySequence, hc)
	return nil
}

// Connect connects to all containerPorts for all charts, dump config in JSON if Persistent flag is present
func (k *Environment) Connect() error {
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

// Disconnect disconnects from all deployed charts, only working in Persistent mode
func (k *Environment) Disconnect() error {
	for _, c := range k.Charts {
		log.Info().
			Str("Release", c.Name).
			Msg("Disconnecting")
		for _, connectionInfo := range c.settings.PodsInfo {
			if err := k.killForwarder(connectionInfo.ForwarderPID); err != nil {
				return err
			}
		}
		for _, ci := range c.settings.PodsInfo {
			ci.ForwarderPID = 0
			ci.LocalPorts = make(map[string]int)
		}
	}
	if err := k.SyncConfig(); err != nil {
		return err
	}
	return nil
}

// LoadHelmEnvironment loads helm environment
func LoadHelmEnvironment(cfg *HelmEnvironmentConfig) (*Environment, error) {
	log.Info().
		Interface("Settings", cfg).
		Msg("Loading environment")
	he, err := NewEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	if err := he.configureHelm(); err != nil {
		return nil, err
	}
	for _, set := range he.Config.ChartsInfo {
		hc, err := NewHelmChart(he, set)
		if err != nil {
			return nil, err
		}
		he.Charts[hc.Name] = hc
	}
	return he, nil
}
