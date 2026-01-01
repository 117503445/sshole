package main

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
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

	// 构建程序列表
	builds := []struct {
		name string
		path string
		out  string
	}{
		{"agent", "./cmd/agent", "./data/agent/sshole_agent"},
		{"entry", "./cmd/entry", "./data/entry/sshole_entry"},
		{"hub", "./cmd/hub", "./data/hub/sshole_hub"},
	}

	// 并行构建
	var wg sync.WaitGroup

	for _, build := range builds {
		wg.Add(1)
		go func(build struct {
			name string
			path string
			out  string
		}) {
			defer wg.Done()

			ctx := log.Output(glog.NewConsoleWriter(
				glog.ConsoleWriterConfig{
					RequestId: "build-" + build.name,
				})).WithContext(ctx)

			log.Ctx(ctx).Info().Msg("building")

			ldflags := fmt.Sprintf(
				"-X 'github.com/117503445/go-template/internal/buildinfo.BuildTime=%s' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.GitBranch=%s' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.GitCommit=%s' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.GitTag=%s' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.GitDirty=%t' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.GitVersion=%s' "+
					"-X 'github.com/117503445/go-template/internal/buildinfo.BuildDir=%s'",
				buildInfo.BuildTime, buildInfo.GitBranch, buildInfo.GitCommit,
				buildInfo.GitTag, buildInfo.GitDirty, buildInfo.GitVersion, buildInfo.BuildDir,
			)

			cmd := exec.Command("go", "build", "-o", build.out, "-ldflags", ldflags, build.path)
			cmd.Dir = "../.."
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
			if output, err := cmd.CombinedOutput(); err != nil {
				log.Ctx(ctx).Panic().Err(err).Str("output", string(output)).Msg("failed to build")
				return
			}

			log.Ctx(ctx).Info().Str("output", build.out).Msg("built successfully")
		}(build)
	}

	wg.Wait()

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
