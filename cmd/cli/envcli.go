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
		},
		Commands: []*cli.Command{
			{
				Name:    "new",
				Aliases: []string{"n"},
				Usage:   "create new environment",
				Action: func(c *cli.Context) error {
					envName := c.String("environment")
					e, err := environment.NewEnvironment(&environment.HelmEnvironmentConfig{
						Persistent: true,
						Name:       envName,
						ChartsInfo: map[string]*environment.ChartSettings{
							"app-1": {
								ReleaseName: "app-1",
								Path:        filepath.Join(tools.ChartsRoot, "localterra_clone"),
								Values:      nil,
							},
							"app-2": {
								ReleaseName: "app-2",
								Path:        filepath.Join(tools.ChartsRoot, "geth"),
								Values:      nil,
							},
						},
					})
					if err != nil {
						return err
					}
					if err := e.Init(); err != nil {
						return err
					}
					if err := e.Deploy(); err != nil {
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
		log.Fatal().Err(err).Send()
	}
}
