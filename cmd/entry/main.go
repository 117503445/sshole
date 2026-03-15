package main

import (
	"context"

	"github.com/117503445/goutils/glog"
	"github.com/117503445/sshole/internal/buildinfo"
	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/entry"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer  string `env:"SSHOLE_ENTRY_HUB_SERVER"`
	Auth       string `env:"SSHOLE_ENTRY_AUTH"`
	AgentName  string `env:"SSHOLE_ENTRY_AGENT_NAME"`
	EntryPort  int    `env:"SSHOLE_ENTRY_PORT" default:"22222"`
	PublicKey  string `env:"SSHOLE_ENTRY_PUBLIC_KEY" default:"~/.ssh/id_ed25519.pub"`
	PrivateKey string `env:"SSHOLE_ENTRY_PRIVATE_KEY" default:"~/.ssh/id_ed25519"`
}

func init() {
	glog.InitZeroLog()
	common.SetCallerMarshalFunc()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
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

	if cli.HubServer == "" {
		log.Panic().Msg("HUB_SERVER is required")
	}

	cfg := entry.EntryConfig{
		HubAddr:        cli.HubServer,
		Token:          cli.Auth,
		AgentName:      cli.AgentName,
		EntryPort:      cli.EntryPort,
		PublicKeyPath:  cli.PublicKey,
		PrivateKeyPath: cli.PrivateKey,
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	runEntry(ctx, cfg)
}
