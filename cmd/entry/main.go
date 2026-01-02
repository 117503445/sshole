package main

import (
	"context"

	"github.com/117503445/goutils/glog"
	"github.com/117503445/sshole/internal/buildinfo"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer  string `env:"HUB_SERVER"`
	Auth       string `env:"AUTH"`
	AgentName  string `env:"AGENT_NAME"`
	SshPort    int    `env:"SSH_PORT" default:"22222"`
	PrivateKey string `env:"PRIVATE_KEY" description:"The path to the private key pam file"`
	PublicKey  string `env:"PUBLIC_KEY" description:"The path to the public key pam file"`
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
		Msg("Starting entry")

	if cli.HubServer == "" {
		log.Panic().Msg("HUB_SERVER is required")
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	cmdEntry(ctx)
}
