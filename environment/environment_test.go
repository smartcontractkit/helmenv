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

func TestDeployAll(t *testing.T) {
	envName := fmt.Sprintf("test-env-%s", uuid.NewV4().String())
	e, err := environment.NewEnvironment(&environment.Config{
		Name: envName,
	})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init()
	require.NoError(t, err)

	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
	})
	require.NoError(t, err)

	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
	})
	require.NoError(t, err)

	err = e.DeployAll()
	require.NoError(t, err)
	err = e.Connect()
	require.NoError(t, err)

	require.NotEmpty(t, e.Config.ChartsInfo["geth"].ConnectionInfo["geth:0:geth-network"].Ports["ws-rpc"])
	require.NotEmpty(t, e.Config.ChartsInfo["geth"].ConnectionInfo["geth:0:geth-network"].LocalPorts["ws-rpc"])

	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:node"].Ports["access"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:node"].LocalPorts["access"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:chainlink-db"].Ports["postgres"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:chainlink-db"].LocalPorts["postgres"])
}

func TestMultipleChartsSeparate(t *testing.T) {
	envName := fmt.Sprintf("test-env-%s", uuid.NewV4().String())
	e, err := environment.NewEnvironment(&environment.Config{
		Name: envName,
	})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init()
	require.NoError(t, err)

	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "geth",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
	})
	require.NoError(t, err)
	err = e.Charts["geth"].Deploy()
	require.NoError(t, err)
	err = e.Charts["geth"].Connect()
	require.NoError(t, err)

	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
	})
	require.NoError(t, err)
	err = e.Charts["chainlink"].Deploy()
	require.NoError(t, err)
	err = e.Charts["chainlink"].Connect()
	require.NoError(t, err)

	require.NotEmpty(t, e.Config.ChartsInfo["geth"].ConnectionInfo["geth:0:geth-network"].Ports["ws-rpc"])
	require.NotEmpty(t, e.Config.ChartsInfo["geth"].ConnectionInfo["geth:0:geth-network"].LocalPorts["ws-rpc"])

	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:node"].Ports["access"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:node"].LocalPorts["access"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:chainlink-db"].Ports["postgres"])
	require.NotEmpty(t, e.Config.ChartsInfo["chainlink"].ConnectionInfo["chainlink-node:0:chainlink-db"].LocalPorts["postgres"])
}
