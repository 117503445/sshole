package main

import (
	"context"
	"time"

	"github.com/117503445/goutils/glog"
	"github.com/117503445/sshole/internal/buildinfo"
	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/hub"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var cli struct {
	AuthToken   string        `env:"AUTH"`
	HTTPAddr    string        `env:"HTTP_ADDR" default:":9000"`
	MappingFile string        `env:"MAPPING_FILE" default:"data/port_mapping.json"`
	Pending     time.Duration `env:"PENDING_TIMEOUT" default:"10s"`
	TunnelDial  time.Duration `env:"TUNNEL_DIAL_TIMEOUT" default:"5s"`
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

	cfg := hub.HubConfig{
		AuthToken:         cli.AuthToken,
		HTTPAddr:          cli.HTTPAddr,
		MappingFile:       cli.MappingFile,
		PendingTimeout:    cli.Pending,
		TunnelDialTimeout: cli.TunnelDial,
	}
	log.Info().Interface("cfg", cfg).Msg("hub config")

	h, err := hub.NewHub(cfg)
	if err != nil {
		log.Panic().Err(err).Msg("init hub failed")
	}

	ctx := context.Background()
	if err := h.Start(ctx); err != nil {
		log.Panic().Err(err).Msg("hub stopped with error")
	}
}
