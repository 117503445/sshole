package main

import (
	"context"
	"os"
	"time"

	"github.com/117503445/goutils/glog"
	"github.com/117503445/sshole/internal/buildinfo"
	"github.com/117503445/sshole/pkg/agent"
	"github.com/117503445/sshole/pkg/common"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var cli struct {
	HubServer  string        `env:"HUB_SERVER"`
	Auth       string        `env:"AUTH"`
	Name       string        `env:"NAME"`
	LocalPort  int           `env:"LOCAL_PORT" default:"22222"`
	SkipSSHD   bool          `env:"SKIP_SSHD" help:"Skip starting embedded sshd, connect to existing SSH server on LocalPort"`
	TunnelDial time.Duration `env:"TUNNEL_DIAL_TIMEOUT" default:"5s"`
}

func init() {
	glog.InitZeroLog()
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
	if cli.Name == "" {
		name, err := os.Hostname()
		if err != nil {
			log.Panic().Err(err).Msg("failed to get hostname")
		}
		cli.Name = name
		log.Info().Str("name", cli.Name).Msg("Using hostname as name")
	}

	cfg := agent.AgentConfig{
		HubURL:    cli.HubServer,
		Token:     cli.Auth,
		AgentName: cli.Name,
		LocalPort: cli.LocalPort,
		SkipSSHD:  cli.SkipSSHD,
	}
	cfg.Timeouts = common.DefaultTimeouts()
	cfg.Timeouts.TunnelDialTimeout = cli.TunnelDial

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	runAgent(ctx, cfg)
}
