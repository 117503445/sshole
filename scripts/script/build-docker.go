package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/117503445/goutils"
	"github.com/117503445/goutils/glog"
	"github.com/rs/zerolog/log"
)

// containerRuntime caches the detected container runtime
var containerRuntime string

// detectContainerRuntime detects and returns the available container runtime.
// It prefers podman over docker. Returns "podman", "docker", or panics if neither is available.
func detectContainerRuntime() string {
	if containerRuntime != "" {
		return containerRuntime
	}

	// Try podman first
	podmanCmd := exec.Command("podman", "info")
	if err := podmanCmd.Run(); err == nil {
		log.Info().Msg("Using podman as container runtime")
		containerRuntime = "podman"
		return containerRuntime
	}

	// Fall back to docker
	dockerCmd := exec.Command("docker", "info")
	if err := dockerCmd.Run(); err == nil {
		log.Info().Msg("Using docker as container runtime")
		containerRuntime = "docker"
		return containerRuntime
	}

	log.Panic().Msg("Neither podman nor docker is available. Please install one of them.")
	return ""
}

// pushImage 推送单个镜像
func pushImage(ctx context.Context, runtime, imageName, registry string) error {
	log.Ctx(ctx).Info().Str("image", imageName).Str("registry", registry).Msg("pushing container image")

	cmd := exec.Command(runtime, "push", imageName)
	cmd.Dir = "../.."
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("output", string(output)).Str("image", imageName).Str("registry", registry).Msg("failed to push container image")
		return err
	}

	log.Ctx(ctx).Info().Str("image", imageName).Str("registry", registry).Msg("pushed container image successfully")
	return nil
}

func buildDocker() {
	glog.InitZeroLog()

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("build container image")

	runtime := detectContainerRuntime()

	// 获取构建信息
	buildInfo, err := goutils.GetBuildInfo(ctx)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("failed to get build info")
		os.Exit(1)
	}

	// 构造 tag
	gitCommit := buildInfo.GitCommit
	if len(gitCommit) > 7 {
		gitCommit = gitCommit[:7]
	}

	// 如果是 dirty build，添加构建日期
	dirtySuffix := ""
	if buildInfo.GitDirty {
		buildDate := time.Now().Format("20060102-150405")
		dirtySuffix = "-" + buildDate
	}

	// 定义要构建的组件
	components := []struct {
		name       string
		dockerfile string
	}{
		{"agent", "./scripts/docker/agent.Dockerfile"},
		{"hub", "./scripts/docker/hub.Dockerfile"},
		{"entry", "./scripts/docker/entry.Dockerfile"},
	}

	// 并行构建所有组件的 docker 镜像
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	for _, component := range components {
		wg.Add(1)
		go func(component struct {
			name       string
			dockerfile string
		}) {
			defer wg.Done()

			tag := "117503445/sshole-" + component.name + ":" + gitCommit + dirtySuffix
			aliyunTag := "registry.cn-hangzhou.aliyuncs.com/117503445/sshole-" + component.name + ":" + gitCommit + dirtySuffix

			// 构建 container 镜像
			cmd := exec.Command(runtime, "build",
				"-t", tag,
				"-t", "117503445/sshole-"+component.name+":latest",
				"-t", aliyunTag,
				"-t", "registry.cn-hangzhou.aliyuncs.com/117503445/sshole-"+component.name+":latest",
				"-f", component.dockerfile, ".")
			cmd.Dir = "../.."
			cmd.Env = os.Environ()

			log.Ctx(ctx).Info().Str("component", component.name).Str("tag", tag).Str("aliyunTag", aliyunTag).Bool("dirty", buildInfo.GitDirty).
				Str("command", cmd.String()).
				Msg("building container image")

			output, err := cmd.CombinedOutput()
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("failed to build docker image for %s: %w, output: %s", component.name, err, string(output))
				}
				errMu.Unlock()
				return
			}

			log.Ctx(ctx).Info().Str("component", component.name).Str("tag", tag).Msg("container image built successfully")
		}(component)
	}

	// 等待所有构建完成
	wg.Wait()

	// 检查构建是否有错误
	if firstErr != nil {
		log.Ctx(ctx).Panic().Err(firstErr).Msg("failed to build docker images")
		return
	}

	log.Ctx(ctx).Info().Msg("all container images built successfully")

	// 如果需要推送
	if cli.BuildDocker.Push {
		log.Ctx(ctx).Info().Msg("pushing container images")

		// 定义要推送的镜像列表
		var images []struct {
			imageName string
			registry  string
		}

		for _, component := range components {
			tag := "117503445/sshole-" + component.name + ":" + gitCommit + dirtySuffix
			aliyunTag := "registry.cn-hangzhou.aliyuncs.com/117503445/sshole-" + component.name + ":" + gitCommit + dirtySuffix

			images = append(images,
				struct {
					imageName string
					registry  string
				}{"117503445/sshole-" + component.name + ":latest", "Docker Hub"},
				struct {
					imageName string
					registry  string
				}{tag, "Docker Hub"},
				struct {
					imageName string
					registry  string
				}{"registry.cn-hangzhou.aliyuncs.com/117503445/sshole-" + component.name + ":latest", "Aliyun"},
				struct {
					imageName string
					registry  string
				}{aliyunTag, "Aliyun"},
			)
		}

		// 并行推送所有镜像
		var pushWg sync.WaitGroup
		var pushFirstErr error
		var pushErrMu sync.Mutex

		for _, img := range images {
			pushWg.Add(1)
			go func(imageName, registry string) {
				defer pushWg.Done()
				if err := pushImage(ctx, runtime, imageName, registry); err != nil {
					pushErrMu.Lock()
					if pushFirstErr == nil {
						pushFirstErr = err
					}
					pushErrMu.Unlock()
				}
			}(img.imageName, img.registry)
		}

		// 等待所有推送完成
		pushWg.Wait()

		// 检查是否有错误
		if pushFirstErr != nil {
			log.Ctx(ctx).Panic().Err(pushFirstErr).Msg("failed to push docker images")
			return
		}

		log.Ctx(ctx).Info().Msg("all container images pushed successfully")
	}

	log.Ctx(ctx).Info().Bool("push", cli.BuildDocker.Push).Msg("container build completed")
}
