package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

// buildOne 编译单个组件到 out，携带统一的 buildinfo ldflags。
func buildOne(ctx context.Context, buildInfo *goutils.BuildInfo, reqID, pkgPath, out, goos, goarch string) error {
	ctx = log.Output(glog.NewConsoleWriter(
		glog.ConsoleWriterConfig{RequestId: reqID},
	)).WithContext(ctx)

	log.Ctx(ctx).Info().Msg("building")

	ldflags := fmt.Sprintf(
		"-X 'github.com/117503445/sshole/internal/buildinfo.BuildTime=%s' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.GitBranch=%s' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.GitCommit=%s' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.GitTag=%s' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.GitDirty=%t' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.GitVersion=%s' "+
			"-X 'github.com/117503445/sshole/internal/buildinfo.BuildDir=%s'",
		buildInfo.BuildTime, buildInfo.GitBranch, buildInfo.GitCommit,
		buildInfo.GitTag, buildInfo.GitDirty, buildInfo.GitVersion, buildInfo.BuildDir,
	)

	cmd := exec.Command("go", "build", "-o", out, "-ldflags", ldflags, "-trimpath", pkgPath)
	cmd.Dir = "../.."
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("GOOS=%s", goos),
		fmt.Sprintf("GOARCH=%s", goarch),
		"CGO_ENABLED=0",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("build %s: %w, output: %s", pkgPath, err, output)
	}

	log.Ctx(ctx).Info().Str("output", out).Msg("built successfully")
	return nil
}

// hubBinsDir 是 hub 内嵌二进制的注入目录（相对 scripts/script）。
const hubBinsDir = "../../cmd/hub/bins"

// resetHubBins 清空 hub 内嵌目录中的旧产物（保留 .gitkeep 占位）。
func resetHubBins() error {
	entries, err := os.ReadDir(hubBinsDir)
	if err != nil {
		return fmt.Errorf("read hub bins dir: %w", err)
	}
	for _, e := range entries {
		if e.Name() == ".gitkeep" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(hubBinsDir, e.Name())); err != nil {
			return fmt.Errorf("remove stale bin %s: %w", e.Name(), err)
		}
	}
	return nil
}

// embedBins 把构建产物复制到 hub 内嵌目录，文件名带平台后缀。
func embedBins(srcs ...string) error {
	if err := resetHubBins(); err != nil {
		return err
	}
	for _, src := range srcs {
		// data/release/sshole_agent-linux-amd64 -> sshole_agent-linux-amd64
		name := filepath.Base(src)
		if !strings.HasPrefix(name, "sshole_") {
			name = "sshole_" + name
		}
		if !strings.Contains(name, "-linux-") {
			name += "-linux-amd64"
		}
		dst := filepath.Join(hubBinsDir, name)
		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("embed %s: %w", src, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
