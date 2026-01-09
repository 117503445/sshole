package e2e

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

// runBasicTest runs the basic connectivity test between hub, agent, and entry containers
func runBasicTest(ctx context.Context, params caseParams) {
	log.Info().Msg("Starting basic E2E test...")

	// Check if Docker images exist
	checkDockerImages([]string{
		"117503445/sshole-hub",
		"117503445/sshole-agent",
		"117503445/sshole-entry",
	})

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
	hubCmd := exec.Command("docker", "run", "--name", "hub", "--rm", "--network", "sshole-test", "-v", params.MappingFile+":/tmp/port_mapping.json", "-e", "MAPPING_FILE=/tmp/port_mapping.json", "117503445/sshole-hub")
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
	agentCmd := exec.Command("docker", "run", "-e", "NAME=test-agent", "--name", "agent", "--rm", "--network", "sshole-test", "117503445/sshole-agent", "--hub-server", "http://hub:9000")
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start agent container")
	}

	// Wait for agent to connect to hub
	log.Info().Msg("Waiting 3 seconds for agent to connect...")
	time.Sleep(3 * time.Second)

	// Start entry container
	log.Info().Msg("Starting entry container...")
	entryCmd := exec.Command("docker", "run", "--name", "entry", "--rm", "-p", "22222:22222", "--network", "sshole-test", "-v", params.PublicKeyPath+":/tmp/public_key.pub", "-e", "AGENT_NAME=test-agent", "-e", "PUBLIC_KEY=/tmp/public_key.pub", "117503445/sshole-entry", "--hub-server", "http://hub:9000")
	entryCmd.Stdout = os.Stdout
	entryCmd.Stderr = os.Stderr

	if err := entryCmd.Start(); err != nil {
		log.Panic().Err(err).Msg("Failed to start entry container")
	}

	// Wait for entry to establish tunnel
	log.Info().Msg("Waiting 3 seconds for tunnel establishment...")
	time.Sleep(3 * time.Second)

	// Test SSH connectivity to agent container through hub
	log.Info().Msg("Testing SSH connectivity to agent container...")
	testSSHConnectivity(params)

	// Wait 5 seconds before stopping test
	log.Info().Msg("Waiting 5 seconds before stopping test...")
	time.Sleep(5 * time.Second)

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

	log.Info().Msg("Basic E2E test completed.")
}

// testSSHConnectivity tests SSH connectivity to the agent container via the hub
func testSSHConnectivity(params caseParams) {
	// Derive private key path from public key path
	keyPath := params.PublicKeyPath[:len(params.PublicKeyPath)-4] // Remove .pub extension
	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		log.Panic().Err(err).Str("key_path", keyPath).Msg("Failed to read private key")
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to parse private key")
	}

	// Create SSH client config
	config := &ssh.ClientConfig{
		User: "root", // SSH tunnel typically uses root user
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // For testing purposes
		Timeout:         10 * time.Second,
	}

	// Connect to localhost:22222 (entry container listens on this port)
	log.Info().Msg("Connecting to localhost:22222...")
	client, err := ssh.Dial("tcp", "localhost:22222", config)
	if err != nil {
		log.Panic().Err(err).Msg("Failed to connect to SSH tunnel")
	}
	defer client.Close()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to create SSH session")
	}
	defer session.Close()

	// Execute a simple command to test connectivity
	log.Info().Msg("Executing test command on agent container...")
	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = os.Stderr

	err = session.Run("echo 'SSH tunnel test successful'")
	if err != nil {
		log.Panic().Err(err).Msg("Failed to execute test command")
	}

	output := stdoutBuf.String()
	log.Info().Str("output", output).Msg("SSH test completed successfully")
}
