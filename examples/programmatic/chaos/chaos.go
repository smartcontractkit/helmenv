package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/chaos/experiments"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"os"
	"time"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	e, err := environment.NewEnvironmentFromConfig(
		environment.NewChainlinkConfig(nil),
		tools.ChartsRoot,
	)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer e.DeferTeardown()

	time.Sleep(10 * time.Second)
	_, err = e.ApplyChaosExperiment(&experiments.PodFailure{
		LabelKey:   "app",
		LabelValue: "chainlink-node",
		Duration:   10 * time.Second,
	})
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	time.Sleep(10 * time.Second)
	if err := e.Chaos.StopAll(); err != nil {
		log.Error().Msg(err.Error())
		return
	}
}
