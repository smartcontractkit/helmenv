package environment

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/smartcontractkit/helmenv/chaos"
	"io/ioutil"
	"os"
)

// SetDefaults set default values for config
func (cfg *Config) SetDefaults() {
	if cfg.ChartsInfo == nil {
		cfg.ChartsInfo = map[string]*ChartSettings{}
	}
	if cfg.KubeCtlProcessName == "" {
		cfg.KubeCtlProcessName = DefaultKubeCTLProcessPath
	}
	if cfg.Preset == nil {
		cfg.Preset = &Preset{
			Name:     cfg.Name,
			Filename: cfg.Name,
		}
	}
	if cfg.Experiments == nil {
		cfg.Experiments = make(map[string]*chaos.ExperimentInfo)
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
	d, err := ioutil.ReadFile(fmt.Sprintf("%s.yaml", path))
	if err != nil {
		return nil, err
	}
	var cfg *Config
	if err := yaml.Unmarshal(d, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
