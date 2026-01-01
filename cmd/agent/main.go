package main

import (
	"context"

	"github.com/117503445/goutils/glog"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer string `env:"HUB_SERVER" default:"localhost:9000"`
	Auth      string `env:"AUTH"`
	SshdPort  int32  `env:"SSHD_PORT" default:"22222"`
}

func main() {
	glog.InitZeroLog()

	kong.Parse(&cli)
	log.Info().Interface("cli", cli).Msg("Starting agent")

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	cmdAgent(ctx)
}
