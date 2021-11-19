package environment

// NewChainlinkChart returns a default Chainlink Helm chart based on a set of override values
func NewChainlinkChart(chainlinkOverrideValues map[string]interface{}) *Chart {
	return &Chart{
		Path:           "chainlink",
		OverrideValues: chainlinkOverrideValues,
	}
}

// NewChainlinkCCIPConfig returns a Chainlink environment for the purpose of CCIP testing
func NewChainlinkCCIPConfig(chainlinkOverrideValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-ccip",
		Charts: []*Chart{
			{Path: "localterra"},
			{Path: "geth-reorg"},
			NewChainlinkChart(chainlinkOverrideValues),
		},
	}
}

// NewTerraChainlinkConfig returns a Chainlink environment designed for testing with a Terra relay
func NewTerraChainlinkConfig(chainlinkOverrideValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-terra",
		Charts: []*Chart{
			{Path: "localterra"},
			{Path: "terra-relay"},
			{Path: "geth-reorg"},
			NewChainlinkChart(ChainlinkReplicas(2, nil)),
		},
	}
}

// NewChainlinkReorgConfig returns a Chainlink environment designed for simulating re-orgs within testing
func NewChainlinkReorgConfig(chainlinkOverrideValues map[string]interface{}) *Config {
	return &Config{
		NamespacePrefix: "chainlink-reorg",
		Charts: []*Chart{
			{Path: "geth-reorg"},
			NewChainlinkChart(chainlinkOverrideValues),
		},
	}
}

// NewChainlinkConfig returns a vanilla Chainlink environment used for generic functional testing
func NewChainlinkConfig(chainlinkOverrideValues map[string]interface{}) *Config {
	return &Config{
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
