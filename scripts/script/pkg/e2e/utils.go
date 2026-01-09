package e2e

import (
	"os/exec"

	"github.com/rs/zerolog/log"
)

// checkDockerImages checks if the specified Docker images exist
func checkDockerImages(images []string) {
	log.Info().Msg("Checking if Docker images exist...")

	for _, image := range images {
		cmd := exec.Command("docker", "images", image, "--format", "{{.Repository}}:{{.Tag}}")
		output, err := cmd.Output()
		if err != nil {
			log.Panic().Err(err).Str("image", image).Msg("Failed to check Docker image")
		}
		if len(output) == 0 {
			log.Panic().Str("image", image).Msg("Docker image not found. Please build or pull the image first.")
		}
		// log.Info().Str("image", string(output[:len(output)-1])).Msg("Docker image found")
	}
}