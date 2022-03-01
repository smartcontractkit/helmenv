package environment

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
func NewChainlinkConfig(chainlinkValues map[string]interface{}, optionalNamespacePrefix string) *Config {
	nameSpacePrefix := "chainlink"
	if optionalNamespacePrefix != "" {
		nameSpacePrefix = optionalNamespacePrefix
	}
	return &Config{
		NamespacePrefix: nameSpacePrefix,
		Charts: Charts{
			"geth":              {Index: 1},
			"mockserver-config": {Index: 2},
			"mockserver":        {Index: 3},
			"chainlink":         NewChainlinkChart(4, chainlinkValues),
		},
	}
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
