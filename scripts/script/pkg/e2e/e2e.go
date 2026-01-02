package e2e

import (
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
)

func E2e() {
	log.Info().Msg("Starting E2E test...")

	// Check if Docker images exist
	log.Info().Msg("Checking if Docker images exist...")
	checkHubImageCmd := exec.Command("docker", "images", "117503445/sshole-hub", "--format", "{{.Repository}}:{{.Tag}}")
	hubOutput, err := checkHubImageCmd.Output()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to check hub Docker image")
	}
	if len(hubOutput) == 0 {
		log.Panic().Msg("Docker image '117503445/sshole-hub' not found. Please build or pull the image first.")
	}
	log.Info().Str("hub_image", string(hubOutput[:len(hubOutput)-1])).Msg("Hub Docker image found")

	checkAgentImageCmd := exec.Command("docker", "images", "117503445/sshole-agent", "--format", "{{.Repository}}:{{.Tag}}")
	agentOutput, err := checkAgentImageCmd.Output()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to check agent Docker image")
	}
	if len(agentOutput) == 0 {
		log.Panic().Msg("Docker image '117503445/sshole-agent' not found. Please build or pull the image first.")
	}
	log.Info().Str("agent_image", string(agentOutput[:len(agentOutput)-1])).Msg("Agent Docker image found")

	// Create a Docker network for the containers to communicate
	log.Info().Msg("Creating Docker network...")
	networkCmd := exec.Command("docker", "network", "create", "sshole-test")
	networkCmd.Stdout = os.Stdout
	networkCmd.Stderr = os.Stderr
	if err := networkCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to create network (might already exist)")
	}

	// Start hub container
	log.Info().Msg("Starting hub container...")
	hubCmd := exec.Command("docker", "run", "--name", "hub", "--rm", "--network", "sshole-test", "117503445/sshole-hub")
	hubCmd.Stdout = os.Stdout
	hubCmd.Stderr = os.Stderr

	if err := hubCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start hub container")
	}

	// Wait 1 second
	log.Info().Msg("Waiting 1 second before starting agent...")
	time.Sleep(1 * time.Second)

	// Start agent container
	log.Info().Msg("Starting agent container...")
	agentCmd := exec.Command("docker", "run", "--name", "agent", "--rm", "--network", "sshole-test", "117503445/sshole-agent", "--hub-server", "http://hub:9000")
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start agent container")
	}

	// Wait 3 seconds before starting entry
	log.Info().Msg("Waiting 3 seconds before starting entry...")
	time.Sleep(3 * time.Second)

	// Start entry container
	log.Info().Msg("Starting entry container...")
	entryCmd := exec.Command("docker", "run", "--name", "entry", "--rm", "--network", "sshole-test", "117503445/sshole-entry", "--hub-server", "http://hub:9000")
	entryCmd.Stdout = os.Stdout
	entryCmd.Stderr = os.Stderr

	if err := entryCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start entry container")
	}

	// Wait 10 seconds before stopping test
	log.Info().Msg("Waiting 10 seconds before stopping test...")
	time.Sleep(10 * time.Second)

	// Stop containers (they will be automatically removed due to --rm flag)
	log.Info().Msg("Stopping containers...")
	stopHubCmd := exec.Command("docker", "stop", "hub")
	if err := stopHubCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop hub container")
	}

	stopAgentCmd := exec.Command("docker", "stop", "agent")
	if err := stopAgentCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop agent container")
	}

	stopEntryCmd := exec.Command("docker", "stop", "entry")
	if err := stopEntryCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop entry container")
	}

	// Clean up network
	log.Info().Msg("Cleaning up network...")
	cleanupCmd := exec.Command("docker", "network", "rm", "sshole-test")
	if err := cleanupCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to remove network")
	}

	log.Info().Msg("E2E test completed.")
}
