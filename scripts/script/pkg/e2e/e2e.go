package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"

	"github.com/rs/zerolog/log"
)

const (
	e2eMappingPath = "/workspace/data/e2e/port_mapping_e2e.json"
	e2eKeyPath     = "/workspace/data/e2e/sshole_e2e_id_ed25519"
	e2ePubKeyPath  = "/workspace/data/e2e/sshole_e2e_id_ed25519.pub"
)

type portMapping struct {
	Agents map[string]int `json:"agents"`
}

type caseParams struct {
	MappingFile    string
	PublicKeyPath  string
	PrivateKeyPath string
}

func ensureE2EMapping() string {
	// Ensure the directory exists
	if err := os.MkdirAll("/workspace/data/e2e", 0o755); err != nil {
		log.Panic().Err(err).Msg("Failed to create E2E directory")
	}

	mapping := portMapping{
		Agents: map[string]int{
			"test-agent":      10022,
			"test-agent-auth": 10023,
		},
	}
	data, err := json.MarshalIndent(mapping, "", "  ")
	if err != nil {
		log.Panic().Err(err).Msg("Failed to marshal E2E port mapping")
	}
	if err := os.WriteFile(e2eMappingPath, data, 0o644); err != nil {
		log.Panic().Err(err).Msg("Failed to write E2E port mapping")
	}
	return e2eMappingPath
}

func ensureE2EKeypair() string {
	// Ensure the directory exists
	if err := os.MkdirAll("/workspace/data/e2e", 0o755); err != nil {
		log.Panic().Err(err).Msg("Failed to create E2E directory")
	}

	if _, err := os.Stat(e2ePubKeyPath); err == nil {
		return e2ePubKeyPath
	}

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", e2eKeyPath, "-N", "", "-C", "sshole-e2e")
	if err := cmd.Run(); err != nil {
		log.Warn().Err(err).Msg("ssh-keygen failed, using fallback public key")
		fallback := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMa+aWUpQZQR5CQQe/Z9ydCqs1+cLPNYMmowGiPrvRGS sshole-e2e"
		if writeErr := os.WriteFile(e2ePubKeyPath, []byte(fallback+"\n"), 0o644); writeErr != nil {
			log.Panic().Err(writeErr).Msg("Failed to write fallback public key")
		}
		return e2ePubKeyPath
	}
	return e2ePubKeyPath
}

func E2e(testCase string) {
	log.Info().Str("test_case", testCase).Msg("Starting E2E test...")

	// If cwd is not /workspace, panic
	if cwd, err := os.Getwd(); err != nil {
		log.Panic().Err(err).Msg("Failed to get current working directory")
	} else if cwd != "/workspace/scripts/script" {
		log.Panic().
			Str("cwd", cwd).
			Msg("Current working directory is not /workspace/scripts/script. Please run the script in dev container.")
	}

	mappingFile := ensureE2EMapping()
	publicKeyPath := ensureE2EKeypair()

	params := caseParams{
		MappingFile:    mappingFile,
		PublicKeyPath:  publicKeyPath,
		PrivateKeyPath: publicKeyPath[:len(publicKeyPath)-4], // Remove .pub extension
	}

	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)

	switch testCase {
	case "":
		// Run all test cases when no case is specified
		log.Info().Msg("Running all test cases...")
		runBasicTest(ctx, params)
		runAuthTest(ctx, params)
	case "basic":
		runBasicTest(ctx, params)
	case "auth":
		runAuthTest(ctx, params)
	default:
		log.Panic().Str("test_case", testCase).Msg("Unknown test case. Available cases: basic, auth")
	}
}
