package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
	"github.com/urfave/cli/v2"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).Level(zerolog.InfoLevel)
}

var presetFlag = &cli.StringFlag{
	Name:     "preset",
	Aliases:  []string{"p"},
	Usage:    "filepath to the environment preset",
	Required: true,
}

var environmentFlag = &cli.StringFlag{
	Name:     "environment",
	Aliases:  []string{"e"},
	Usage:    "filepath to the environment file",
	Required: true,
}

func main() {
	app := &cli.App{
		Name:  "envcli",
		Usage: "setup default environment",
		Commands: []*cli.Command{
			{
				Name:    "new",
				Aliases: []string{"n"},
				Usage:   "create new environment from preset file",
				Flags: []cli.Flag{
					presetFlag,
					&cli.StringFlag{
						Name:     "outputFile",
						Aliases:  []string{"o"},
						Usage:    "file path for the outputted environment config",
						Required: false,
					},
				},
				Action: func(c *cli.Context) error {
					preset := c.String("preset")
					// Override the `Path` key in Config to dictate where the file is written
					if err := os.Setenv("CONFIG_PATH", c.String("outputFile")); err != nil {
						return err
					}
					e, err := environment.DeployOrLoadEnvironmentFromConfigFile(preset)
					if err != nil {
						return err
					}
					log.Info().
						Str("environmentFile", e.Path).
						Msg("Environment setup and written to file")
					return nil
				},
			},
			{
				Name:    "connect",
				Aliases: []string{"c"},
				Usage:   "connects to selected environment",
				Flags:   []cli.Flag{environmentFlag},
				Action: func(c *cli.Context) error {
					environmentPath := c.String("environment")
					e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
					if err != nil {
						return err
					}
					if err := e.ConnectAll(); err != nil {
						return err
					}
					defer func() {
						e.Disconnect()
						if err := e.ClearConfigLocalPorts(); err != nil {
							log.Error().Err(err).Msg("Error while clearing local ports in environment config")
						}
						log.Info().Str("Namespace", e.Namespace).Msg("Disconnected from environment")
					}()
					log.Info().
						Str("Namespace", e.Namespace).
						Msgf("Ports forwarded, view output or `%s` file for connection details", e.Path)

					// Wait until the user wants to stop the connection to the environment
					interrupt := make(chan os.Signal, 1)
					signal.Notify(interrupt, os.Interrupt)
					<-interrupt
					return nil
				},
			},
			{
				Name:    "remove",
				Aliases: []string{"rm"},
				Usage:   "remove the environment",
				Flags:   []cli.Flag{environmentFlag},
				Action: func(c *cli.Context) error {
					environmentPath := c.String("environment")
					e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
					if err != nil {
						return err
					}
					namespace := e.Namespace
					log.Info().Str("Namespace", namespace).Msg("Tearing down environment")
					if err := e.Teardown(); err != nil {
						return err
					}
					if err := e.ClearConfig(); err != nil {
						return err
					}
					log.Info().Msgf("Environment removed, it can be re-created by running: `envcli new -p %s`", e.Path)
					return nil
				},
			},
			{
				Name:    "dump",
				Aliases: []string{"d"},
				Flags: []cli.Flag{
					environmentFlag,
					&cli.StringFlag{
						Name:     "artifacts",
						Aliases:  []string{"a"},
						Usage:    "artifacts dir to store logs",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "database",
						Aliases:  []string{"db"},
						Usage:    "database name to dump",
						Required: true,
					},
				},
				Usage: "dump all the logs from the environment",
				Action: func(c *cli.Context) error {
					environmentPath := c.String("environment")
					artifactsDir := c.String("artifacts")
					dbName := c.String("database")
					e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
					if err != nil {
						return err
					}
					if err := e.Artifacts.DumpTestResult(artifactsDir, dbName); err != nil {
						return err
					}
					return nil
				},
			},
			{
				Name:    "chaos",
				Aliases: []string{"ch"},
				Usage:   "controls chaos experiments in a particular namespace",
				Subcommands: []*cli.Command{
					{
						Name:    "apply",
						Aliases: []string{"a"},
						Flags: []cli.Flag{
							environmentFlag,
							&cli.StringFlag{
								Name:     "template",
								Aliases:  []string{"t"},
								Usage:    "chaos template to be applied",
								Required: true,
							},
						},
						Action: func(c *cli.Context) error {
							environmentPath := c.String("environment")
							chaosTemplate := c.String("template")
							e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
							if err != nil {
								return err
							}
							if err = e.ApplyChaosExperimentFromTemplate(chaosTemplate); err != nil {
								return err
							}
							return nil
						},
					},
					{
						Name:    "stop",
						Aliases: []string{"s"},
						Flags: []cli.Flag{
							environmentFlag,
							&cli.StringFlag{
								Name:     "chaos_id",
								Aliases:  []string{"c"},
								Usage:    "chaos experiment name",
								Required: true,
							},
						},
						Usage: "stops particular chaos experiment",
						Action: func(c *cli.Context) error {
							environmentPath := c.String("environment")
							chaosID := c.String("chaos_id")
							e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
							if err != nil {
								return err
							}
							expInfo, ok := e.Config.Experiments[chaosID]
							if !ok {
								return fmt.Errorf("experiment with id %s not found", expInfo.Name)
							}
							if err = e.StopChaosStandaloneExperiment(expInfo); err != nil {
								return err
							}
							return nil
						},
					},
					{
						Name:    "clear",
						Aliases: []string{"c"},
						Usage:   "clears all applied chaos experiments",
						Flags:   []cli.Flag{environmentFlag},
						Action: func(c *cli.Context) error {
							environmentPath := c.String("environment")
							e, err := environment.DeployOrLoadEnvironmentFromConfigFile(environmentPath)
							if err != nil {
								return err
							}
							if err = e.ClearAllChaosStandaloneExperiments(e.Config.Experiments); err != nil {
								return err
							}
							return nil
						},
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Error().Err(err).Send()
	}
}
