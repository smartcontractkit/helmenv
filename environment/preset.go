package environment

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/tools"
	"github.com/spf13/viper"
	"path"
	"path/filepath"
	"strings"
)

// LoadPresetConfig loads preset config with viper, allows to override yaml values from env
func LoadPresetConfig(cfgPath string) (*Config, error) {
	dir, file := path.Split(cfgPath)
	log.Info().
		Str("Dir", dir).
		Str("File", file).
		Msg("Loading preset file")
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	v.SetConfigName(file)
	if dir == "" {
		v.AddConfigPath(".")
	} else {
		v.AddConfigPath(dir)
	}
	v.SetConfigType("yml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg *Config
	err := v.Unmarshal(&cfg)
	return cfg, err
}

// NewEnvironmentFromPreset creates environment preset from config file
func NewEnvironmentFromPreset(presetFilepath string) (*Environment, error) {
	cfg, err := LoadPresetConfig(presetFilepath)
	if err != nil {
		return nil, err
	}
	cfg.Preset.Filename = presetFilepath
	switch cfg.Preset.Type {
	case "chainlink-cluster":
		return NewChainlinkEnv(cfg)
	default:
		return nil, fmt.Errorf("no suitable preset found: %s", cfg.Preset.Name)
	}
}

// NewTerraChainlink new chainlink env with LocalTerra blockchain
func NewTerraChainlink(cfg *Config) (*Environment, error) {
	e, err := NewEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	if err := e.Init(); err != nil {
		return nil, err
	}
	if err := e.AddChart(&ChartSettings{
		ReleaseName:    "localterra",
		Path:           filepath.Join(tools.ChartsRoot, "localterra"),
		OverrideValues: nil,
	}); err != nil {
		return nil, err
	}
	// TODO: awaiting ability to setup IC/CI creds via API, otherwise we have a circular dependency  in deployment
	//if err := e.AddChart(&environment.ChartSettings{
	//	ReleaseName: "terra-relay",
	//	Path:        filepath.Join(tools.ChartsRoot, "terra-relay"),
	//	OverrideValues:      nil,
	//}); err != nil {
	//	return nil, err
	//}
	if err := e.AddChart(&ChartSettings{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		OverrideValues: map[string]interface{}{
			"replicaCount": 2,
		},
	}); err != nil {
		return nil, err
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, nil
}

// NewChainlinkEnv creates new chainlink environment
func NewChainlinkEnv(cfg *Config) (*Environment, error) {
	e, err := NewEnvironment(cfg)
	if err != nil {
		return nil, err
	}
	if err := e.Init(); err != nil {
		return nil, err
	}
	gethReleaseName := "geth"
	if err := e.AddChart(&ChartSettings{
		ReleaseName:    gethReleaseName,
		Path:           filepath.Join(tools.ChartsRoot, gethReleaseName),
		OverrideValues: nil,
	}); err != nil {
		return nil, err
	}
	chainlinkReleaseName := "chainlink"
	if err := e.AddChart(&ChartSettings{
		ReleaseName:    chainlinkReleaseName,
		Path:           filepath.Join(tools.ChartsRoot, chainlinkReleaseName),
		OverrideValues: cfg.Preset.Values[chainlinkReleaseName].(map[string]interface{}),
	}); err != nil {
		return nil, err
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, nil
}
