package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func release() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("release build")

	// 创建输出目录
	releaseDir := "./data/release"
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		log.Ctx(ctx).Error().Err(err).Str("dir", releaseDir).Msg("failed to create release directory")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Str("dir", releaseDir).Msg("created release directory")

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}

	// 第一阶段：并行构建 entry 全平台 + agent linux/amd64（hub 内嵌依赖）
	targets := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
		{"windows", "arm64"},
	}

	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(target struct {
			os   string
			arch string
		}) {
			defer wg.Done()
			ext := ""
			if target.os == "windows" {
				ext = ".exe"
			}
			outFile := fmt.Sprintf("./data/release/sshole_entry-%s-%s%s", target.os, target.arch, ext)
			reqID := fmt.Sprintf("release-entry-%s-%s", target.os, target.arch)
			if err := buildOne(ctx, buildInfo, reqID, "./cmd/entry", outFile, target.os, target.arch); err != nil {
				log.Ctx(ctx).Panic().Err(err).Msg("failed to build release binary for entry")
			}
		}(target)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := buildOne(ctx, buildInfo, "release-sshole_agent-linux-amd64", "./cmd/agent", "./data/release/sshole_agent-linux-amd64", "linux", "amd64"); err != nil {
			log.Ctx(ctx).Panic().Err(err).Msg("failed to build release binary for agent")
		}
	}()

	wg.Wait()

	// 注入 hub 内嵌目录（entry / agent 的 linux-amd64）
	if err := embedBins("../../data/release/sshole_agent-linux-amd64", "../../data/release/sshole_entry-linux-amd64"); err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("failed to embed bins")
	}
	log.Ctx(ctx).Info().Msg("embedded agent/entry into hub bins")

	// 第二阶段：构建 hub（内嵌 agent/entry）
	if err := buildOne(ctx, buildInfo, "release-sshole_hub-linux-amd64", "./cmd/hub", "./data/release/sshole_hub-linux-amd64", "linux", "amd64"); err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("failed to build release binary for hub")
	}

	log.Ctx(ctx).Info().Msg("all release builds completed")
}
