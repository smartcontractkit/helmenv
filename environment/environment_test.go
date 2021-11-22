package environment_test

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	uuid "github.com/satori/go.uuid"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func teardown(t *testing.T, e *environment.Environment) {
	err := e.Teardown()
	require.NoError(t, err)
}

func TestCanDeployAll(t *testing.T) {
	envName := fmt.Sprintf("test-env-%s", uuid.NewV4().String())
	e, err := environment.NewEnvironment(&environment.Config{})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init(envName)
	require.NoError(t, err)

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
		Index:       2, // Deliberate unordered keys to test the OrderedKeys function in Charts
	})
	require.NoError(t, err)

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Index:       4, // Deliberate unordered keys to test the OrderedKeys function in Charts
	})
	require.NoError(t, err)

	err = e.DeployAll()
	require.NoError(t, err)
	err = e.ConnectAll()
	require.NoError(t, err)

	require.NotEmpty(t, e.Config.Charts["geth"].ChartConnections["geth_0_geth-network"].RemotePorts["ws-rpc"])
	require.NotEmpty(t, e.Config.Charts["geth"].ChartConnections["geth_0_geth-network"].LocalPorts["ws-rpc"])

	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_node"].RemotePorts["access"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_node"].LocalPorts["access"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_chainlink-db"].RemotePorts["postgres"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_chainlink-db"].LocalPorts["postgres"])
}

func TestMultipleChartsSeparate(t *testing.T) {
	envName := fmt.Sprintf("test-env-%s", uuid.NewV4().String())
	e, err := environment.NewEnvironment(&environment.Config{})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init(envName)
	require.NoError(t, err)

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
		Index:       1,
	})
	require.NoError(t, err)
	err = e.Deploy("geth")
	require.NoError(t, err)
	err = e.Connect("geth")
	require.NoError(t, err)

	err = e.AddChart(&environment.HelmChart{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		Index:       2,
	})
	require.NoError(t, err)
	err = e.Deploy("chainlink")
	require.NoError(t, err)
	err = e.Connect("chainlink")
	require.NoError(t, err)

	require.NotEmpty(t, e.Config.Charts["geth"].ChartConnections["geth_0_geth-network"].RemotePorts["ws-rpc"])
	require.NotEmpty(t, e.Config.Charts["geth"].ChartConnections["geth_0_geth-network"].LocalPorts["ws-rpc"])

	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_node"].RemotePorts["access"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_node"].LocalPorts["access"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_chainlink-db"].RemotePorts["postgres"])
	require.NotEmpty(t, e.Config.Charts["chainlink"].ChartConnections["chainlink-node_0_chainlink-db"].LocalPorts["postgres"])
}

func TestCanConnectProgrammatically(t *testing.T) {
	// TODO
}

func TestCanConnectCLI(t *testing.T) {
	// TODO
}
