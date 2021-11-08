package environment

import (
	"fmt"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"os"
)

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
