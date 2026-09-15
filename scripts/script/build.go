package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

func build() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("build")

	// 创建输出目录
	dirs := []string{"./data/sshole"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Ctx(ctx).Error().Err(err).Str("dir", dir).Msg("failed to create directory")
			os.Exit(1)
		}
		log.Ctx(ctx).Info().Str("dir", dir).Msg("created directory")
	}

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}
	log.Ctx(ctx).Info().Interface("buildInfo", buildInfo).Msg("build info")

	// 第一阶段：并行构建 agent / entry（供 hub 内嵌）
	phase1 := []struct {
		name   string
		path   string
		out    string
		goos   string
		goarch string
	}{
		// linux/amd64 无后缀产物同时供 agent.Dockerfile / entry.Dockerfile 使用
		{"agent-linux-amd64", "./cmd/agent", "./data/agent/sshole_agent", "linux", "amd64"},
		{"agent-darwin-arm64", "./cmd/agent", "./data/agent/sshole_agent-darwin-arm64", "darwin", "arm64"},
		{"entry-linux-amd64", "./cmd/entry", "./data/entry/sshole_entry", "linux", "amd64"},
		{"entry-darwin-arm64", "./cmd/entry", "./data/entry/sshole_entry-darwin-arm64", "darwin", "arm64"},
	}
	var wg sync.WaitGroup
	for _, b := range phase1 {
		wg.Add(1)
		go func(b struct {
			name   string
			path   string
			out    string
			goos   string
			goarch string
		}) {
			defer wg.Done()
			if err := buildOne(ctx, buildInfo, "build-"+b.name, b.path, b.out, b.goos, b.goarch); err != nil {
				log.Ctx(ctx).Panic().Err(err).Msg("failed to build")
			}
		}(b)
	}
	wg.Wait()

	// 注入 hub 内嵌目录（agent/entry 的 linux-amd64 与 darwin-arm64）
	if err := embedBins(
		"../../data/agent/sshole_agent",
		"../../data/agent/sshole_agent-darwin-arm64",
		"../../data/entry/sshole_entry",
		"../../data/entry/sshole_entry-darwin-arm64",
	); err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("failed to embed bins")
	}
	log.Ctx(ctx).Info().Msg("embedded agent/entry into hub bins")

	// 第二阶段：构建 hub（内嵌 agent/entry）
	if err := buildOne(ctx, buildInfo, "build-hub", "./cmd/hub", "./data/hub/sshole_hub", "linux", "amd64"); err != nil {
		log.Ctx(ctx).Panic().Err(err).Msg("failed to build hub")
	}

	log.Ctx(ctx).Info().Msg("all builds completed")
}

func createFcZip(ctx context.Context, name, sourceFile, zipPath string) error {
	// 创建 zip 文件
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// 创建 zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 打开源文件
	source, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// 在 zip 中创建文件，文件名就是程序名（根目录），并设置执行权限
	header := &zip.FileHeader{
		Name:   name,
		Method: zip.Deflate,
	}
	header.SetMode(0755) // 设置执行权限 (rwxr-xr-x)
	zipEntry, err := zipWriter.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	// 复制文件内容
	_, err = io.Copy(zipEntry, source)
	if err != nil {
		return fmt.Errorf("failed to copy file to zip: %w", err)
	}

	return nil
}
