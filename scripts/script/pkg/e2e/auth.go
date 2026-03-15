package e2e

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
)

// runAuthTest runs the authentication test with auth set to 123456
func runAuthTest(ctx context.Context, params caseParams) {
	log.Info().Msg("Starting auth E2E test...")

	runtime := detectContainerRuntime()

	// Check if container images exist
	checkDockerImages([]string{
		"117503445/sshole-hub",
		"117503445/sshole-agent",
		"117503445/sshole-entry",
	})

	// Create a container network for the containers to communicate
	log.Info().Msg("Creating container network...")
	networkCmd := exec.Command(runtime, "network", "create", "sshole-auth-test")
	networkCmd.Stdout = os.Stdout
	networkCmd.Stderr = os.Stderr
	if err := networkCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to create network (might already exist)")
	}

	// Start hub container with auth
	log.Info().Msg("Starting hub container with auth...")
	hubCmd := exec.Command(runtime, "run", "--name", "hub-auth", "--rm", "--network", "sshole-auth-test", "-v", params.MappingFile+":/tmp/port_mapping.json", "-e", "AUTH=123456", "-e", "MAPPING_FILE=/tmp/port_mapping.json", "117503445/sshole-hub")
	hubCmd.Stdout = os.Stdout
	hubCmd.Stderr = os.Stderr

	if err := hubCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start hub container")
	}

	// Wait 1 second
	log.Info().Msg("Waiting 1 second before starting agent...")
	time.Sleep(1 * time.Second)

	// Start agent container with auth
	log.Info().Msg("Starting agent container with auth...")
	agentCmd := exec.Command(runtime, "run", "-e", "AUTH=123456", "-e", "NAME=test-agent-auth", "--name", "agent-auth", "--rm", "--network", "sshole-auth-test", "117503445/sshole-agent", "--hub-server", "http://hub-auth:9000")
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start agent container")
	}

	// Wait 3 seconds before starting entry
	log.Info().Msg("Waiting 3 seconds before starting entry...")
	time.Sleep(3 * time.Second)

	// Start entry container with auth
	log.Info().Msg("Starting entry container with auth...")
	entryCmd := exec.Command(runtime, "run", "--name", "entry-auth", "--rm", "--network", "sshole-auth-test", "-v", params.PublicKeyPath+":/tmp/public_key.pub", "-e", "AUTH=123456", "-e", "AGENT_NAME=test-agent-auth", "-e", "PUBLIC_KEY=/tmp/public_key.pub", "117503445/sshole-entry", "--hub-server", "http://hub-auth:9000")
	entryCmd.Stdout = os.Stdout
	entryCmd.Stderr = os.Stderr

	if err := entryCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start entry container")
	}

	// Wait 10 seconds before stopping test
	log.Info().Msg("Waiting 10 seconds for auth test...")
	time.Sleep(10 * time.Second)

	// Stop containers (they will be automatically removed due to --rm flag)
	log.Info().Msg("Stopping containers...")
	stopHubCmd := exec.Command(runtime, "stop", "hub-auth")
	if err := stopHubCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop hub container")
	}

	stopAgentCmd := exec.Command(runtime, "stop", "agent-auth")
	if err := stopAgentCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop agent container")
	}

	stopEntryCmd := exec.Command(runtime, "stop", "entry-auth")
	if err := stopEntryCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to stop entry container")
	}

	// Clean up network
	log.Info().Msg("Cleaning up network...")
	cleanupCmd := exec.Command(runtime, "network", "rm", "sshole-auth-test")
	if err := cleanupCmd.Run(); err != nil {
		log.Warn().Err(err).Msg("Failed to remove network")
	}

	log.Info().Msg("Auth E2E test completed.")
}
