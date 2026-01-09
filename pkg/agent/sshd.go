package agent

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog/log"
)

//go:embed openssh-V_9_9_P2.tar.gz
var embeddedOpenSSH []byte

// ensureSSHD starts a bundled sshd listening on 127.0.0.1:<port> if not already running.
func (a *Agent) ensureSSHD(ctx context.Context, port int) (func(), error) {
	if isPortListening(port) {
		log.Info().Int("port", port).Msg("sshd already listening")
		return func() {}, nil
	}

	baseDir := "/tmp/sshole_agent"
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}

	tarPath := filepath.Join(baseDir, "openssh.tar.gz")
	if _, err := os.Stat(tarPath); err != nil {
		if err := os.WriteFile(tarPath, embeddedOpenSSH, 0o644); err != nil {
			return nil, fmt.Errorf("write openssh tar: %w", err)
		}
	}

	optDir := filepath.Join(baseDir, "opt", "openssh")
	sshdPath := filepath.Join(optDir, "sbin", "sshd")
	if _, err := os.Stat(sshdPath); err != nil {
		if err := os.RemoveAll(optDir); err != nil {
			return nil, fmt.Errorf("clean openssh dir: %w", err)
		}
		if err := goutils.Extract(ctx, tarPath, baseDir); err != nil {
			return nil, fmt.Errorf("extract openssh: %w", err)
		}
	}

	cfgPath := filepath.Join(optDir, "etc", "sshd_config")
	cfg := fmt.Sprintf(`Port %d
ListenAddress 127.0.0.1
PermitRootLogin yes
PasswordAuthentication no
PubkeyAuthentication yes
	AuthorizedKeysFile ~/.sshole/authorized_keys
HostKey %s
Subsystem sftp %s`,
		port,
		filepath.Join(optDir, "etc", "ssh_host_ed25519_key"),
		filepath.Join(optDir, "libexec", "sftp-server"),
	)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		return nil, fmt.Errorf("write sshd_config: %w", err)
	}

	hostKey := hostPrivateKey
	if !strings.HasSuffix(hostKey, "\n") {
		hostKey += "\n"
	}
	if err := os.WriteFile(filepath.Join(optDir, "etc", "ssh_host_ed25519_key"), []byte(hostKey), 0o600); err != nil {
		return nil, fmt.Errorf("write host key: %w", err)
	}

	if err := ensurePrivsepAssets(optDir); err != nil {
		return nil, err
	}

	ssholeDir := filepath.Join(userHomeDir(), ".sshole")
	if err := os.MkdirAll(ssholeDir, 0o700); err != nil {
		log.Warn().Err(err).Msg("create ~/.sshole failed")
	}
	authKeys := filepath.Join(ssholeDir, "authorized_keys")
	if _, err := os.Stat(authKeys); err != nil {
		// create empty authorized_keys to keep sshd happy; operator must populate.
		if err := os.WriteFile(authKeys, []byte{}, 0o600); err != nil {
			log.Warn().Err(err).Msg("init authorized_keys failed")
		}
	}

	cmd := exec.Command(filepath.Join(optDir, "sbin", "sshd"), "-D", "-e", "-f", cfgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start sshd: %w", err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for {
		if isPortListening(port) {
			break
		}
		if waitCtx.Err() != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("sshd failed to listen on %d", port)
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Info().Int("port", port).Msg("sshd started")
	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
			time.Sleep(500 * time.Millisecond)
			_ = cmd.Process.Kill()
		}
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

func ensurePrivsepAssets(optDir string) error {
	privsep := filepath.Join("/opt/openssh", "var", "empty")
	if err := os.MkdirAll(privsep, 0o755); err != nil {
		return fmt.Errorf("create privsep dir: %w", err)
	}

	src := filepath.Join(optDir, "libexec", "sshd-session")
	dst := filepath.Join("/opt/openssh", "libexec", "sshd-session")
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create libexec dir: %w", err)
	}
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cleanup sshd-session: %w", err)
	}
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst, 0o755); err != nil {
		return fmt.Errorf("install sshd-session: %w", err)
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
