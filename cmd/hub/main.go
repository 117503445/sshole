package main

import (
	"context"
	"embed"
	"io/fs"
	"time"

	"github.com/117503445/goutils/glog"
	"github.com/117503445/sshole/internal/buildinfo"
	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/hub"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// bins 目录由 CI 构建脚本注入 entry/agent 二进制（见 scripts/script）。
//
//go:embed all:bins
var binsEmbed embed.FS

var cli struct {
	AuthToken   string        `env:"SSHOLE_HUB_AUTH"`
	HTTPAddr    string        `env:"SSHOLE_HUB_HTTP_ADDR" default:":9000"`
	MappingFile string        `env:"SSHOLE_HUB_MAPPING_FILE" default:"data/port_mapping.json"`
	Pending     time.Duration `env:"SSHOLE_HUB_PENDING_TIMEOUT" default:"10s"`
	TunnelDial  time.Duration `env:"SSHOLE_HUB_TUNNEL_DIAL_TIMEOUT" default:"5s"`
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

	// 内嵌的 entry/agent 二进制（仅 CI 构建产物包含，本地开发为空占位）
	binsFS, err := fs.Sub(binsEmbed, "bins")
	if err != nil {
		log.Panic().Err(err).Msg("load embedded bins failed")
	}

	cfg := hub.HubConfig{
		AuthToken:         cli.AuthToken,
		HTTPAddr:          cli.HTTPAddr,
		MappingFile:       cli.MappingFile,
		PendingTimeout:    cli.Pending,
		TunnelDialTimeout: cli.TunnelDial,
		BinsFS:            binsFS,
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
