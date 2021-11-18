package environment

import (
	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"os"
	"path"
)

// Preset represents a series of Helm charts to be installed into a new namespace
type Preset struct {
	NamespacePrefix string   `json:"namespace_prefix"`
	Charts          []*Chart `json:"charts"`
}

// NewEnvironmentFromPreset returns a deployed environment from a given preset that can be ones pre-defined within
// the library, or passed in as part of lib usage
func NewEnvironmentFromPreset(config *Config, preset *Preset, chartDirectory string) (*Environment, error) {
	e, err := NewEnvironment(config)
	if err != nil {
		return nil, err
	}
	if err := e.Init(preset.NamespacePrefix); err != nil {
		return nil, err
	}
	for _, chart := range preset.Charts {
		if len(chart.ReleaseName) == 0 {
			chart.ReleaseName = chart.Path
		}
		if len(chartDirectory) > 0 {
			chart.Path = path.Join(chartDirectory, chart.Path)
		}
		if err := e.AddChart(chart); err != nil {
			return nil, err
		}
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			return nil, errors.Wrapf(err, "failed to shutdown namespace")
		}
		return nil, err
	}
	return e, e.SyncConfig()
}

// NewEnvironmentFromPresetFile returns an environment based on a preset file, mostly for use as a presets CLI
func NewEnvironmentFromPresetFile(chartDirectory, filePath string) (*Environment, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	preset := &Preset{}
	if err := yaml.Unmarshal(contents, &preset); err != nil {
		return nil, err
	}
	return NewEnvironmentFromPreset(&Config{Persistent: true, PersistentConnection: true}, preset, chartDirectory)
}

// NewChainlinkChart returns a default Chainlink Helm chart based on a set of override values
func NewChainlinkChart(chainlinkOverrideValues map[string]interface{}) *Chart {
	return &Chart{
		Path:           "chainlink",
		OverrideValues: chainlinkOverrideValues,
	}
}

// NewChainlinkCCIPPreset returns a Chainlink environment for the purpose of CCIP testing
func NewChainlinkCCIPPreset(chainlinkOverrideValues map[string]interface{}) *Preset {
	return &Preset{
		NamespacePrefix: "chainlink-ccip",
		Charts: []*Chart{
			{Path: "localterra"},
			{Path: "geth-reorg"},
			NewChainlinkChart(chainlinkOverrideValues),
		},
	}
}

// NewTerraChainlinkPreset returns a Chainlink environment designed for testing with a Terra relay
func NewTerraChainlinkPreset(chainlinkOverrideValues map[string]interface{}) *Preset {
	return &Preset{
		NamespacePrefix: "chainlink-terra",
		Charts: []*Chart{
			{Path: "localterra"},
			{Path: "terra-relay"},
			{Path: "geth-reorg"},
			NewChainlinkChart(ChainlinkReplicas(2, nil)),
		},
	}
}

// NewChainlinkReorgPreset returns a Chainlink environment designed for simulating re-orgs within testing
func NewChainlinkReorgPreset(chainlinkOverrideValues map[string]interface{}) *Preset {
	return &Preset{
		NamespacePrefix: "chainlink-reorg",
		Charts: []*Chart{
			{Path: "geth-reorg"},
			NewChainlinkChart(chainlinkOverrideValues),
		},
	}
}

// NewChainlinkPreset returns a vanilla Chainlink environment used for generic functional testing
func NewChainlinkPreset(chainlinkOverrideValues map[string]interface{}) *Preset {
	return &Preset{
		NamespacePrefix: "chainlink",
		Charts: []*Chart{
			{Path: "geth"},
			{Path: "mockserver-config"},
			{Path: "mockserver"},
			NewChainlinkChart(chainlinkOverrideValues),
		},
	}
}

func ChainlinkVersion(version string, chainlinkOverrideValues map[string]interface{}) map[string]interface{} {
	if chainlinkOverrideValues == nil {
		chainlinkOverrideValues = map[string]interface{}{}
	}
	chainlinkOverrideValues["chainlink"] = map[string]interface{}{
		"image": map[string]interface{}{
			"version": version,
		},
	}
	return chainlinkOverrideValues
}

func ChainlinkReplicas(count int, chainlinkOverrideValues map[string]interface{}) map[string]interface{} {
	if chainlinkOverrideValues == nil {
		chainlinkOverrideValues = map[string]interface{}{}
	}
	chainlinkOverrideValues["replicas"] = count
	return chainlinkOverrideValues
}
