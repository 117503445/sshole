package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rs/zerolog/log"
)

var (
	projectRoot     string
	e2eDataDir      string
	e2eMappingPath  string
	e2eKeyPath      string
	e2ePubKeyPath   string
)

func init() {
	// Detect project root by finding go.mod
	wd, err := os.Getwd()
	if err != nil {
		log.Panic().Err(err).Msg("Failed to get working directory")
	}

	// Walk up to find go.mod or use current directory structure
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Check if this is the scripts/script directory
			if filepath.Base(dir) == "script" {
				parent := filepath.Dir(dir)
				if filepath.Base(parent) == "scripts" {
					projectRoot = filepath.Dir(parent)
					break
				}
			}
			// Check if this looks like the sshole project root
			if _, err := os.Stat(filepath.Join(dir, "cmd")); err == nil {
				projectRoot = dir
				break
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, use current directory
			projectRoot = wd
			break
		}
		dir = parent
	}

	e2eDataDir = filepath.Join(projectRoot, "data", "e2e")
	e2eMappingPath = filepath.Join(e2eDataDir, "port_mapping_e2e.json")
	e2eKeyPath = filepath.Join(e2eDataDir, "sshole_e2e_id_ed25519")
	e2ePubKeyPath = filepath.Join(e2eDataDir, "sshole_e2e_id_ed25519.pub")

	log.Info().Str("project_root", projectRoot).Str("e2e_data_dir", e2eDataDir).Msg("E2E paths configured")
}

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
	if err := os.MkdirAll(e2eDataDir, 0o755); err != nil {
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
	if err := os.MkdirAll(e2eDataDir, 0o755); err != nil {
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
