package environment_test

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"helmenv/environment"
	"helmenv/tools"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func teardown(t *testing.T, e *environment.HelmEnvironment) {
	err := e.Teardown()
	require.NoError(t, err)
}

func TestMultipleAppsAtOnceSetup(t *testing.T) {
	e, err := environment.NewEnvironment(&environment.HelmEnvironmentConfig{
		Name: "env-1",
		ChartsInfo: map[string]*environment.ChartSettings{
			"app-1": {
				ReleaseName: "app-1",
				Path:        filepath.Join(tools.ChartsRoot, "localterra_clone"),
			},
			"app-2": {
				ReleaseName: "app-2",
				Path:        filepath.Join(tools.ChartsRoot, "geth"),
			},
		},
	})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init()
	require.NoError(t, err)
	err = e.Deploy()
	require.NoError(t, err)
	err = e.Connect()
	require.NoError(t, err)
	require.NotEmpty(t, e.Config.ChartsInfo["app-1"].PodsInfo["terrad:0"].LocalPorts["lcd"])
}

func TestMultipleChartsSeparate(t *testing.T) {
	e, err := environment.NewEnvironment(&environment.HelmEnvironmentConfig{
		Name: "env-2",
	})
	defer teardown(t, e)
	require.NoError(t, err)
	err = e.Init()
	require.NoError(t, err)
	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "app-1",
		Path:        filepath.Join(tools.ChartsRoot, "localterra_clone"),
	})
	require.NoError(t, err)
	err = e.Charts["app-1"].Deploy()
	require.NoError(t, err)
	err = e.Charts["app-1"].Connect()
	require.NoError(t, err)
	require.NotEmpty(t, e.Config.ChartsInfo["app-1"].PodsInfo["terrad:0"].LocalPorts["lcd"])
	err = e.AddChart(&environment.ChartSettings{
		ReleaseName: "app-2",
		Path:        filepath.Join(tools.ChartsRoot, "geth"),
	})
	require.NoError(t, err)
	err = e.Charts["app-2"].Deploy()
	require.NoError(t, err)
	err = e.Charts["app-2"].Connect()
	require.NoError(t, err)
	require.NotEmpty(t, e.Config.ChartsInfo["app-2"].PodsInfo["geth:0"].LocalPorts["ws-rpc"])
}
