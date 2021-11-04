package environment

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// DumpConfig dumps config to a file
func DumpConfig(cfg *Config, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	d, err := json.Marshal(cfg)
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
	if err := json.Unmarshal(d, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
