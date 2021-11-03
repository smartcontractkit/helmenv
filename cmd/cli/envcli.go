package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
	"helmenv/environment"
	"helmenv/tools"
	"os"
	"path/filepath"
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
				Usage:   "create new environment",
				Action: func(c *cli.Context) error {
					envName := c.String("environment")
					kctlProcessName := c.String("kubectlProcess")
					e, err := environment.NewEnvironment(&environment.HelmEnvironmentConfig{
						Persistent:         true,
						KubeCtlProcessName: kctlProcessName,
						Name:               envName,
					})
					if err != nil {
						return err
					}
					if err := e.Init(); err != nil {
						return err
					}
					if err := e.AddChart(&environment.ChartSettings{
						ReleaseName: "geth",
						Path:        filepath.Join(tools.ChartsRoot, "geth"),
						Values:      nil,
					}); err != nil {
						return err
					}
					if err := e.AddChart(&environment.ChartSettings{
						ReleaseName: "chainlink",
						Path:        filepath.Join(tools.ChartsRoot, "chainlink"),
						Values:      nil,
					}); err != nil {
						return err
					}
					if err := e.DeployAll(); err != nil {
						if err := e.Teardown(); err != nil {
							return errors.Wrapf(err, "failed to shutdown namespace")
						}
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
					cc, err := environment.LoadConfigJSON(&environment.HelmEnvironmentConfig{}, envFile)
					if err != nil {
						return err
					}
					log.Debug().Str("NamespaceName", cc.NamespaceName).Send()
					e, err := environment.LoadHelmEnvironment(cc)
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
					cc, err := environment.LoadConfigJSON(&environment.HelmEnvironmentConfig{}, envFile)
					if err != nil {
						return err
					}
					e, err := environment.LoadHelmEnvironment(cc)
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
					cc, err := environment.LoadConfigJSON(&environment.HelmEnvironmentConfig{}, envFile)
					if err != nil {
						return err
					}
					e, err := environment.LoadHelmEnvironment(cc)
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
