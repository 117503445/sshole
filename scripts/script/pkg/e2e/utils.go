package e2e

import (
	"os/exec"

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

// checkDockerImages checks if the specified container images exist
func checkDockerImages(images []string) {
	runtime := detectContainerRuntime()
	log.Info().Str("runtime", runtime).Msg("Checking if container images exist...")

	for _, image := range images {
		cmd := exec.Command(runtime, "images", image, "--format", "{{.Repository}}:{{.Tag}}")
		output, err := cmd.Output()
		if err != nil {
			log.Panic().Err(err).Str("image", image).Str("runtime", runtime).Msg("Failed to check container image")
		}
		if len(output) == 0 {
			log.Panic().Str("image", image).Str("runtime", runtime).Msg("Container image not found. Please build or pull the image first.")
		}
		// log.Info().Str("image", string(output[:len(output)-1])).Msg("Container image found")
	}
}