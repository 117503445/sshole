package main

import (
	"context"
	"github.com/117503445/sshole/internal/buildinfo"

	"github.com/117503445/goutils/glog"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer string `env:"HUB_SERVER" default:"localhost:9000"`
	Auth      string `env:"AUTH"`
	Name      string `env:"NAME"`
	SshdPort  int    `env:"SSHD_PORT" default:"22222"`
}

func main() {
	glog.InitZeroLog()

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

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	cmdAgent(ctx)
}
