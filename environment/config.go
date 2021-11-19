package environment

import (
	"github.com/smartcontractkit/helmenv/chaos"
	"gopkg.in/yaml.v3"
	"os"
)

// Config represents the full configuration of an environment, it can either be defined
// programmatically at runtime, or defined in files to be used in a CLI or any other application
type Config struct {
	Persistent           bool                             `json:"persistent"`
	PersistentConnection bool                             `json:"persistent_connection"`
	NamespacePrefix      string                           `json:"namespace_prefix,omitempty"`
	Namespace            string                           `json:"namespace,omitempty"`
	Charts               Charts                           `json:"charts,omitempty"`
	Experiments          map[string]*chaos.ExperimentInfo `json:"experiments,omitempty"`
}

// Charts represents a series of charts with some helper methods
type Charts []*Chart

// Connections is a helper method for simply accessing chart connections, also safely allowing method chaining
func (c Charts) Connections(chart string) *ChartConnections {
	for _, c := range c {
		if c.ReleaseName == chart {
			return &c.ChartConnections
		}
	}
	return &ChartConnections{}
}

// DumpConfig dumps config to a file
func DumpConfig(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	d, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := f.Write(d); err != nil {
		return err
	}
	return nil
}

// NewEnvironmentFromConfig returns a deployed environment from a given preset that can be ones pre-defined within
// the library, or passed in as part of lib usage
func NewEnvironmentFromConfig(config *Config, chartDirectory string) (*Environment, error) {
	if len(config.Namespace) > 0 {
		return LoadEnvironment(config)
	}
	return DeployEnvironment(config, chartDirectory)
}

// NewEnvironmentFromConfigFile returns an environment based on a preset file, mostly for use as a presets CLI
func NewEnvironmentFromConfigFile(chartDirectory, filePath string) (*Environment, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	config := &Config{}
	if err := yaml.Unmarshal(contents, &config); err != nil {
		return nil, err
	}
	return NewEnvironmentFromConfig(config, chartDirectory)
}

