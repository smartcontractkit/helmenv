package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"os"
	"path/filepath"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func deployMyPersistentEnv(presetFileName string) error {
	e, err := environment.NewEnvironment(&environment.Config{
		Persistent: true,
		Name:       presetFileName,
	})
	if err != nil {
		panic(err)
	}
	if err := e.Init(); err != nil {
		panic(err)
	}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName:    "geth",
		Path:           filepath.Join(tools.ChartsRoot, "geth"),
		OverrideValues: nil,
	}); err != nil {
		panic(err)
	}
	if err := e.AddChart(&environment.ChartSettings{
		ReleaseName:    "chainlink",
		Path:           filepath.Join(tools.ChartsRoot, "chainlink"),
		OverrideValues: nil,
	}); err != nil {
		panic(err)
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			panic(err)
		}
		panic(err)
	}
	return nil
}

func main() {
	presetFileName := "persistent-env-example-preset"
	if err := deployMyPersistentEnv(presetFileName); err != nil {
		panic(err)
	}
	e, err := environment.LoadEnvironment(presetFileName)
	if err != nil {
		panic(err)
	}
	// if you don't need to keep forwarders as a forked process, for example in test
	e.Config.PersistentConnection = false
	if err := e.Connect(); err != nil {
		panic(err)
	}
	log.Info().
		Int("Remote port", e.Config.ChartsInfo["geth"].ConnectionInfo["geth_0_geth-network"].Ports["ws-rpc"]).
		Int("Local connected port", e.Config.ChartsInfo["geth"].ConnectionInfo["geth_0_geth-network"].LocalPorts["ws-rpc"]).
		Msg("Connected")
	if err := e.Teardown(); err != nil {
		panic(err)
	}
}
