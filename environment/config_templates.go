package environment

import (
	"fmt"

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

// NewChainlinkConfig returns a vanilla Chainlink environment used for generic functional testing
func NewChainlinkConfig(
	chainlinkValues map[string]interface{},
	networks []string,
	optionalNamespacePrefix string,
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
	// TODO: This will break or have weird behavior with multiple geth networks selected.
	// helmenv needs to be able to distinguish different geth instances
	if len(networks) > 1 {
		log.Warn().
			Str("Networks", fmt.Sprintf("%v", networks)).
			Msg("If attempting to launch multiple geth instances, helmenv has some odd behavior, and doesn't fully support this usage yet")
	}
	for _, network := range networks {
		if network == "geth" {
			charts["geth"] = &HelmChart{Index: 1}
		} else if network == "geth_performance" {
			charts["geth"] = &HelmChart{Index: 1, Values: performanceGeth()}
		} else if network == "geth_realistic" {
			charts["geth"] = &HelmChart{Index: 1, Values: realisticGeth()}
		}
	}
	return &Config{
		NamespacePrefix: nameSpacePrefix,
		Charts:          charts,
	}
}

// performanceGeth sets up the simulated geth instance with more power, bigger blocks, and faster mining
func performanceGeth() map[string]interface{} {
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
	return values
}

// realisticGeth sets up the simulated geth instance to emulate the actual ethereum mainnet as close as possible
func realisticGeth() map[string]interface{} {
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
		"--dev.period":      "7",
		"--miner.threads":   "4",
		"--miner.gasprice":  "10000000000",
		"--miner.gastarget": "15000000000",
	}

	return values
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
