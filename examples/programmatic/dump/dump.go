package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"os"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	e, err := environment.NewEnvironmentFromPreset(
		&environment.Config{
			Persistent: false,
		},
		environment.NewChainlinkPreset(nil),
		tools.ChartsRoot,
	)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer e.DeferTeardown()

	if err := e.Artifacts.DumpTestResult("test_1", "chainlink"); err != nil {
		log.Error().Msg(err.Error())
	}
}
