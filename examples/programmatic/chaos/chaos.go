package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/chaos/experiments"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/tools"
	"os"
	"path/filepath"
	"time"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func deployMyEphemeralEnv() (*environment.Environment, error) {
	e, err := environment.NewEnvironment(&environment.Config{
		Persistent: false,
		Name:       "my-env",
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
		ReleaseName: "chainlink",
		Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
		OverrideValues: map[string]interface{}{
			"replicas": 2,
		},
	}); err != nil {
		panic(err)
	}
	if err := e.DeployAll(); err != nil {
		if err := e.Teardown(); err != nil {
			panic(err)
		}
		panic(err)
	}
	return e, nil
}

func main() {
	e, err := deployMyEphemeralEnv()
	if err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Second)
	_, err = e.ApplyExperiment(&experiments.PodFailure{
		LabelKey:   "app",
		LabelValue: "chainlink-node",
		Duration:   10 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	time.Sleep(10 * time.Second)
	if err := e.Chaos.StopAll(); err != nil {
		panic(err)
	}
}
