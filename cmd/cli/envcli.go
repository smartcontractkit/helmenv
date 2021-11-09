package main

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/urfave/cli/v2"
	"os"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	app := &cli.App{
		Name:  "envcli",
		Usage: "setup default environment",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "kubectlProcess",
				Aliases:  []string{"kctl"},
				Value:    "/usr/local/bin/kubectl",
				Usage:    "kubectl process name to use forking for port forwarding",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "preset",
				Aliases:  []string{"p"},
				Usage:    "environment preset name",
				Required: true,
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "new",
				Aliases: []string{"n"},
				Usage:   "create new environment from preset file",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					_, err := environment.NewEnvironmentFromPreset(presetName)
					if err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "connect",
				Aliases: []string{"c"},
				Usage:   "connects to selected environment",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					if err := e.Connect(); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "disconnect",
				Aliases: []string{"dc"},
				Usage:   "disconnects from the environment",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					if err := e.Disconnect(); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "remove",
				Aliases: []string{"rm"},
				Usage:   "remove the environment",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					if err := e.Teardown(); err != nil {
						return err
					}
					if err := e.RemoveConfigConnectionInfo(); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "dump",
				Aliases: []string{"d"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "artifacts",
						Aliases:  []string{"a"},
						Usage:    "artifacts dir to store logs",
						Required: true,
					},
				},
				Usage: "dump all the logs from the environment",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					artifactsDir := c.String("artifacts")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					if err := e.Artifacts.DumpTestResult(artifactsDir); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "chaos",
				Aliases: []string{"ch"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "template",
						Aliases:  []string{"t"},
						Usage:    "chaos template to be applied",
						Required: true,
					},
				},
				Usage: "applies chaos to a particular namespace",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					chaosTemplate := c.String("template")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					if err = e.ApplyExperimentStandalone(chaosTemplate); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "stop chaos",
				Aliases: []string{"sch"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "id",
						Aliases:  []string{"i"},
						Usage:    "chaos experiment id",
						Required: true,
					},
				},
				Usage: "stops particular chaos experiment",
				Action: func(c *cli.Context) error {
					presetName := c.String("preset")
					chaosID := c.String("id")
					e, err := environment.LoadEnvironment(presetName)
					if err != nil {
						return err
					}
					expInfo, ok := e.Config.Experiments[chaosID]
					if !ok {
						return fmt.Errorf("experiment with id %s not found", expInfo.Name)
					}
					if err = e.StopExperimentStandalone(expInfo); err != nil {
						return err
					}
					return nil
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error().Err(err).Send()
	}
}
