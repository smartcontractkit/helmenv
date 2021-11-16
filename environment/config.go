package environment

import (
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/smartcontractkit/helmenv/chaos"
	"io/ioutil"
	"os"
	"strings"
)

// TODO: some naming convention in charts must be used to simplify this part for a generic case
func (cfg *Config) chainlinkClusterURLs() error {
	cfg.NetworksURLs = map[string]map[string][]string{}
	cfg.NetworksURLs["chainlink"] = map[string][]string{}
	cfg.NetworksURLs["chainlink"]["local"] = []string{}
	cfg.NetworksURLs["chainlink"]["cluster"] = []string{}
	cfg.NetworksURLs["geth"] = map[string][]string{}
	cfg.NetworksURLs["mockserver"] = map[string][]string{}
	cfg.NetworksURLs["mockserver"]["local"] = []string{
		fmt.Sprintf("http://localhost:%d",
			cfg.ChartsInfo["mockserver"].ConnectionInfo["mockserver_0_mockserver"].LocalPorts["serviceport"],
		)}
	cfg.NetworksURLs["mockserver"]["cluster"] = []string{
		fmt.Sprintf("http://%s:%d",
			cfg.ChartsInfo["mockserver"].ConnectionInfo["mockserver_0_mockserver"].PodIP,
			cfg.ChartsInfo["mockserver"].ConnectionInfo["mockserver_0_mockserver"].Ports["serviceport"],
		)}
	chainlinkChart := cfg.ChartsInfo["chainlink"]
	for ciName, ci := range chainlinkChart.ConnectionInfo {
		if strings.HasPrefix(ciName, "chainlink-node") && strings.HasSuffix(ciName, "node") {
			cfg.NetworksURLs["chainlink"]["local"] = append(cfg.NetworksURLs["chainlink"]["local"],
				fmt.Sprintf("http://localhost:%d", ci.LocalPorts["access"]))
			cfg.NetworksURLs["chainlink"]["cluster"] = append(cfg.NetworksURLs["chainlink"]["cluster"],
				fmt.Sprintf("http://%s:%d", ci.PodIP, ci.Ports["access"]))
		}
	}
	gethChart := cfg.ChartsInfo["geth"]
	cfg.NetworksURLs["geth"]["local"] = append(
		cfg.NetworksURLs["geth"]["local"],
		fmt.Sprintf("ws://localhost:%d", gethChart.ConnectionInfo["geth_0_geth-network"].LocalPorts["ws-rpc"]),
	)
	return nil
}

func (cfg *Config) ccipURLs() error {
	//cfg.NetworksURLs = map[string]map[string][]string{}
	//chainlinkChart := cfg.ChartsInfo["chainlink"]
	//reorgChart := cfg.ChartsInfo["geth-reorg"]
	//terraChart := cfg.ChartsInfo["localterra"]
	//cfg.NetworksURLs["chainlink"] = []string{}
	//cfg.NetworksURLs["geth-reorg"] = []string{}
	//cfg.NetworksURLs["localterra"] = []string{}
	//cfg.NetworksURLs["chainlink"] = append(
	//	cfg.NetworksURLs["chainlink"],
	//	httpURL(chainlinkChart.ConnectionInfo["chainlink-node_0_node"].LocalPorts["access"]),
	//)
	//cfg.NetworksURLs["geth-reorg"] = append(
	//	cfg.NetworksURLs["geth-reorg"],
	//	wsURL(reorgChart.ConnectionInfo["geth_0_geth"].LocalPorts["ws-rpc"]),
	//)
	//cfg.NetworksURLs["geth-reorg"] = append(
	//	cfg.NetworksURLs["geth-reorg"],
	//	wsURL(reorgChart.ConnectionInfo["miner-node_0_geth-miner"].LocalPorts["ws-rpc"]),
	//)
	//cfg.NetworksURLs["geth-reorg"] = append(
	//	cfg.NetworksURLs["geth-reorg"],
	//	wsURL(reorgChart.ConnectionInfo["miner-node_1_geth-miner"].LocalPorts["ws-rpc"]),
	//)
	//cfg.NetworksURLs["localterra"] = append(
	//	cfg.NetworksURLs["localterra"],
	//	httpURL(terraChart.ConnectionInfo["terrad_0_terrad"].LocalPorts["lcd"]))
	//cfg.NetworksURLs["localterra"] = append(
	//	cfg.NetworksURLs["localterra"],
	//	httpURL(terraChart.ConnectionInfo["terrad_1_terrad"].LocalPorts["lcd"]))
	return nil
}

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
