package main

import (
	"fmt"
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
			Persistent: true,
		},
		environment.NewChainlinkPreset(nil),
		tools.ChartsRoot,
	)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer e.DeferTeardown()

	loadedEnv, err := environment.LoadPersistentEnvironment(fmt.Sprintf("%s.yaml", e.Config.NamespaceName))
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	// if you don't need to keep forwarders as a forked process, for example in test
	loadedEnv.Config.PersistentConnection = false
	if err := loadedEnv.ConnectAll(); err != nil {
		log.Error().Msg(err.Error())
		return
	}
	remoteURLs, err := loadedEnv.Config.Charts.Connections("geth").RemoteHTTPURLs("http-rpc")
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	log.Info().Interface("URLs", remoteURLs).Msg("Remote Geth")

	localURLs, err := loadedEnv.Config.Charts.Connections("geth").LocalHTTPURLs("http-rpc")
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	log.Info().Interface("URLs", localURLs).Msg("Local Geth")
}
