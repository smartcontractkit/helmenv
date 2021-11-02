package environment

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

// DumpConfigJSON dumps arbitrary JSON to a file
func DumpConfigJSON(cfg *HelmEnvironmentConfig, path string) error {
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

// LoadConfigJSON loads arbitrary JSON from a file
func LoadConfigJSON(cfg *HelmEnvironmentConfig, path string) (*HelmEnvironmentConfig, error) {
	d, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(d, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
