package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/117503445/sshdev/pkg/sshlib"
	"github.com/rs/zerolog/log"
)

// ensureSSHD starts an SSH server listening on 127.0.0.1:<port> using sshdev.
func (a *Agent) ensureSSHD(ctx context.Context, port int) (func(), error) {
	if isPortListening(port) {
		log.Info().Int("port", port).Msg("sshd already listening")
		return func() {}, nil
	}

	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	cfg := &sshlib.Config{
		ListenAddr:     listenAddr,
		HostKeyContent: hostPrivateKey,
		// HostKeyBuiltin: true,
	}

	// Force public key authentication
	authorizedKeysPath, err := ensureAuthorizedKeysFile()
	if err != nil {
		return nil, fmt.Errorf("ensure authorized_keys: %w", err)
	}
	cfg.AuthorizedKeysFiles = authorizedKeysPath
	log.Info().Str("path", authorizedKeysPath).Msg("using authorized_keys for authentication")

	server, err := sshlib.NewServer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create ssh server: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	// Wait for server to start listening
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if isPortListening(port) {
			break
		}
		select {
		case err := <-errCh:
			return nil, fmt.Errorf("ssh server failed: %w", err)
		case <-waitCtx.Done():
			server.Stop()
			return nil, fmt.Errorf("ssh server failed to listen on %d", port)
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Info().Int("port", port).Msg("sshd started using sshdev")
	return func() {
		server.Stop()
	}, nil
}

func isPortListening(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(home)
}

func ensureAuthorizedKeysFile() (string, error) {
	home := userHomeDir()
	if home == "" {
		return "", fmt.Errorf("failed to get user home directory")
	}
	ssholeDir := filepath.Join(home, ".sshole")
	if err := os.MkdirAll(ssholeDir, 0o700); err != nil {
		return "", fmt.Errorf("create .sshole dir: %w", err)
	}
	authKeys := filepath.Join(ssholeDir, "authorized_keys")
	if _, err := os.Stat(authKeys); err != nil {
		// create empty authorized_keys to keep sshd happy; operator must populate.
		if err := os.WriteFile(authKeys, []byte{}, 0o600); err != nil {
			log.Warn().Err(err).Msg("init authorized_keys failed")
		}
	}
	return authKeys, nil
}
