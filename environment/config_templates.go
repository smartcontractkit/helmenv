package environment

import (
	"reflect"

	"github.com/rs/zerolog/log"
)

// NewChainlinkChart returns a default Chainlink Helm chart based on a set of override values
func NewChainlinkChart(index int, values map[string]interface{}) *HelmChart {
	return &HelmChart{Path: "chainlink", Values: values, Index: index}
}

// NewChainlinkCCIPConfig returns a Chainlink environment for the purpose of CCIP testing
func NewChainlinkCCIPConfig(chainlinkValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-ccip",
		Charts: Charts{
			"localterra": {Index: 1},
			"geth-reorg": {Index: 2},
			"chainlink":  NewChainlinkChart(3, chainlinkValues),
		},
	}
}

// NewTerraChainlinkConfig returns a Chainlink environment designed for testing with a Terra relay
func NewTerraChainlinkConfig(chainlinkValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-terra",
		Charts: Charts{
			"localterra": {Index: 1},
			"geth-reorg": {Index: 2},
			"chainlink":  NewChainlinkChart(3, ChainlinkReplicas(2, chainlinkValues)),
		},
	}
}

// NewChainlinkReorgConfig returns a Chainlink environment designed for simulating re-orgs within testing
func NewChainlinkReorgConfig(chainlinkValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-reorg",
		Charts: Charts{
			"geth-reorg": {Index: 1},
			"chainlink":  NewChainlinkChart(2, chainlinkValues),
		},
	}
}

// Organizes passed in values for simulated network charts
type networkChart struct {
	Replicas int
	Values   map[string]interface{}
}

// NewChainlinkConfig returns a vanilla Chainlink environment used for generic functional testing. Geth networks can
// be passed in to launch differently configured simulated geth instances.
func NewChainlinkConfig(
	chainlinkValues map[string]interface{},
	optionalNamespacePrefix string,
	networks ...SimulatedNetwork,
) *Config {
	nameSpacePrefix := "chainlink"
	if optionalNamespacePrefix != "" {
		nameSpacePrefix = optionalNamespacePrefix
	}
	charts := Charts{
		"mockserver-config": {Index: 2},
		"mockserver":        {Index: 3},
		"chainlink":         NewChainlinkChart(4, chainlinkValues),
	}

	networkCharts := map[string]*networkChart{}
	for _, networkFunc := range networks {
		chartName, networkValues := networkFunc()
		if networkValues == nil {
			networkValues = map[string]interface{}{}
		}
		// TODO: If multiple networks with the same chart name are present, only use the values from the first one.
		// This means that we can't have mixed network values with the same type
		// (e.g. all geth deployments need to have the same values).
		// Enabling different behavior is a bit of a niche case.
		if _, present := networkCharts[chartName]; !present {
			networkCharts[chartName] = &networkChart{Replicas: 1, Values: networkValues}
		} else {
			if !reflect.DeepEqual(networkValues, networkCharts[chartName].Values) {
				log.Warn().Msg("If trying to launch multiple networks with different underlying values but the same type, " +
					"(e.g. 1 geth performance and 1 geth realistic), that behavior is not currently fully supported. " +
					"Only replicas of the first of that network type will be launched.")
			}
			networkCharts[chartName].Replicas++
		}
	}

	for chartName, networkChart := range networkCharts {
		networkChart.Values["replicas"] = networkChart.Replicas
		charts[chartName] = &HelmChart{Index: 1, Values: networkChart.Values}
	}

	return &Config{
		NamespacePrefix: nameSpacePrefix,
		Charts:          charts,
	}
}

// SimulatedNetwork is a function that enables launching a simulated network with a returned chart name
// and corresponding values
type SimulatedNetwork func() (string, map[string]interface{})

// DefaultGeth sets up a basic, low-power simulated geth instance. Really just returns empty map to use default values
func DefaultGeth() (string, map[string]interface{}) {
	return "geth", map[string]interface{}{}
}

// PerformanceGeth sets up the simulated geth instance with more power, bigger blocks, and faster mining
func PerformanceGeth() (string, map[string]interface{}) {
	values := map[string]interface{}{}
	values["resources"] = map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    "4",
			"memory": "4096Mi",
		},
		"limits": map[string]interface{}{
			"cpu":    "4",
			"memory": "4096Mi",
		},
	}
	values["config_args"] = map[string]interface{}{
		"--dev.period":      "1",
		"--miner.threads":   "4",
		"--miner.gasprice":  "10000000000",
		"--miner.gastarget": "30000000000",
		"--cache":           "4096",
	}
	return "geth", values
}

// RealisticGeth sets up the simulated geth instance to emulate the actual ethereum mainnet as close as possible
func RealisticGeth() (string, map[string]interface{}) {
	values := map[string]interface{}{}
	values["resources"] = map[string]interface{}{
		"requests": map[string]interface{}{
			"cpu":    "4",
			"memory": "4096Mi",
		},
		"limits": map[string]interface{}{
			"cpu":    "4",
			"memory": "4096Mi",
		},
	}
	values["config_args"] = map[string]interface{}{
		"--dev.period":      "14",
		"--miner.threads":   "4",
		"--miner.gasprice":  "10000000000",
		"--miner.gastarget": "15000000000",
		"--cache":           "4096",
	}

	return "geth", values
}

// ChainlinkVersion sets the version of the chainlink image to use
func ChainlinkVersion(version string, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	values["chainlink"] = map[string]interface{}{
		"image": map[string]interface{}{
			"version": version,
		},
	}
	return values
}

// ChainlinkReplicas sets the replica count of chainlink nodes to use
func ChainlinkReplicas(count int, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	values["replicas"] = count
	return values
}
