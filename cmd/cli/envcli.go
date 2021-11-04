package main

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/smartcontractkit/helmenv/preset"
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
				Name:     "environment",
				Aliases:  []string{"n"},
				Usage:    "environment name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "kubectlProcess",
				Aliases:  []string{"kctl"},
				Value:    "/usr/local/bin/kubectl",
				Usage:    "kubectl process name to use forking for port forwarding",
				Required: false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:    "new",
				Aliases: []string{"n"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "preset",
						Aliases:  []string{"p"},
						Usage:    "environment preset name",
						Required: true,
					},
				},
				Usage: "create new environment",
				Action: func(c *cli.Context) error {
					envName := c.String("environment")
					presetName := c.String("preset")
					kctlProcessName := c.String("kubectlProcess")
					_, err := preset.UsePreset(presetName, &environment.Config{
						Persistent:         true,
						KubeCtlProcessName: kctlProcessName,
						Name:               envName,
					})
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
					envName := c.String("environment")
					envFile := fmt.Sprintf("%s.json", envName)
					cc, err := environment.LoadConfig(envFile)
					if err != nil {
						return err
					}
					e, err := environment.LoadEnvironment(cc)
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
					envName := c.String("environment")
					envFile := fmt.Sprintf("%s.json", envName)
					cc, err := environment.LoadConfig(envFile)
					if err != nil {
						return err
					}
					e, err := environment.LoadEnvironment(cc)
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
					envName := c.String("environment")
					envFile := fmt.Sprintf("%s.json", envName)
					cc, err := environment.LoadConfig(envFile)
					if err != nil {
						return err
					}
					e, err := environment.LoadEnvironment(cc)
					if err != nil {
						return err
					}
					if err := e.Teardown(); err != nil {
						return err
					}
					if err := os.Remove(envFile); err != nil {
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
