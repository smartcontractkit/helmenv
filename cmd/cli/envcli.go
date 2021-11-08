package main

import (
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
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error().Err(err).Send()
	}
}
