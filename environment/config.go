package environment

import (
	"github.com/smartcontractkit/helmenv/chaos"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
)

// Config environment config with all charts info,
// it is used both in runtime and can be persistent in JSON
type Config struct {
	Persistent           bool                             `json:"persistent"`
	PersistentConnection bool                             `json:"persistent_connection"`
	NamespaceName        string                           `json:"namespace_name,omitempty"`
	Charts               Charts                           `json:"charts,omitempty"`
	Experiments          map[string]*chaos.ExperimentInfo `json:"experiments,omitempty"`
}

// Charts represents a map of charts with some helper methods
type Charts map[string]*Chart

// Connections is a helper method for simply accessing chart connections, also safely allowing method chaining
func (c Charts) Connections(chart string) *ChartConnections {
	if chart, ok := c[chart]; !ok {
		return &ChartConnections{}
	} else {
		return &chart.ChartConnections
	}
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

// LoadConfig loads config from a file
func LoadConfig(path string) (*Config, error) {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg *Config
	if err := yaml.Unmarshal(d, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
