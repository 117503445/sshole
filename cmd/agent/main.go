package main

import (
	"context"
	"os"

	"github.com/117503445/sshole/internal/buildinfo"

	"github.com/117503445/goutils/glog"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer string `env:"HUB_SERVER"`
	Auth      string `env:"AUTH"`
	Name      string `env:"NAME"`
	SshdPort  int    `env:"SSHD_PORT" default:"22222"`
}

func init() {
	glog.InitZeroLog()
}

func main() {
	kong.Parse(&cli)

	log.Info().
		Str("BuildTime", buildinfo.BuildTime).
		Str("GitBranch", buildinfo.GitBranch).
		Str("GitCommit", buildinfo.GitCommit).
		Str("GitTag", buildinfo.GitTag).
		Str("GitDirty", buildinfo.GitDirty).
		Str("GitVersion", buildinfo.GitVersion).
		Str("BuildDir", buildinfo.BuildDir).
		Msg("build info")

	log.Info().Interface("cli", cli).
		Msg("Starting agent")

	if cli.HubServer == "" {
		log.Panic().Msg("HUB_SERVER is required")
	}
	if cli.Name == "" {
		// set hostname as name
		name, err := os.Hostname()
		if err != nil {
			log.Panic().Err(err).Msg("failed to get hostname")
		}
		cli.Name = name
		log.Info().Str("name", cli.Name).Msg("Using hostname as name")
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	cmdAgent(ctx)
}
