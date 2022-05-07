package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smartcontractkit/helmenv/environment"
)

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

func main() {
	e, err := environment.DeployOrLoadEnvironment(
		environment.NewChainlinkConfig(nil, "helmenv-new-env", environment.DefaultGeth))
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	defer e.DeferTeardown()

	if err := e.ConnectAll(); err != nil {
		log.Error().Msg(err.Error())
		return
	}

	logger := log.Info()
	urls, err := e.Config.Charts.Connections("geth").LocalURLsByPort("ws-rpc", environment.WS)
	if err != nil {
		log.Error().Msg(err.Error())
		return
	}
	for i, url := range urls {
		logger.Str(fmt.Sprintf("URL %d", i), url.String())
	}
	logger.Msg("Connected")
}
