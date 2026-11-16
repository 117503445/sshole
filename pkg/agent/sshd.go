package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
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

	// Windows 没有 /bin/sh，sshlib 默认会校验该路径并直接启动失败。
	if runtime.GOOS == "windows" && cfg.Shell == "" {
		cfg.Shell = "powershell.exe"
	}

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

// sshdWatchInterval 是内嵌 sshd 的巡检周期；监听消失说明服务协程已经退出。
const sshdWatchInterval = 15 * time.Second

// setSSHCleanup 记录当前内嵌 sshd 的关闭函数，并清掉上一次的失败原因。
// 参数 cleanup：winio/sshlib 服务返回的停止函数。
func (a *Agent) setSSHCleanup(cleanup func()) {
	a.sshMu.Lock()
	defer a.sshMu.Unlock()
	a.sshCleanup = cleanup
	a.sshErr = ""
}

// closeSSHCleanup 关闭当前内嵌 sshd，供 Agent 退出或重启前调用。
func (a *Agent) closeSSHCleanup() {
	a.sshMu.Lock()
	cleanup := a.sshCleanup
	a.sshCleanup = nil
	a.sshMu.Unlock()
	if cleanup != nil {
		cleanup()
	}
}

// sshProbe 返回本机 SSH 端口此刻是否可连接，以及最近一次失败原因，供状态帧使用。
func (a *Agent) sshProbe() (bool, string) {
	if isPortListening(a.cfg.LocalPort) {
		return true, ""
	}
	a.sshMu.Lock()
	reason := a.sshErr
	a.sshMu.Unlock()
	if reason == "" {
		reason = fmt.Sprintf("127.0.0.1:%d 未监听", a.cfg.LocalPort)
	}
	return false, reason
}

// watchSSHD 后台看护内嵌 sshd：监听消失就重启，失败原因留在状态帧里让 Hub 和 Entry 看见。
//
// SkipSSHD 模式下 SSH 服务由现场自己提供，只探测不接管，避免误杀别人的 sshd。
func (a *Agent) watchSSHD(ctx context.Context, port int) {
	if a.cfg.SkipSSHD {
		return
	}
	go func() {
		ticker := time.NewTicker(sshdWatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			if isPortListening(port) {
				continue
			}
			log.Warn().Int("port", port).Msg("embedded sshd listener gone, restarting")
			a.restartSSHD(ctx, port)
		}
	}()
}

// restartSSHD 在本机端口上重新创建一个内嵌 sshd；失败时记录原因，等下一轮巡检再试。
//
// 这里刻意不对已经死掉的旧服务调 Stop：sshdev 的 Server.Start 写 s.listener、
// Server.Stop 读 s.listener，两者之间没有任何同步（CI -race 实测），跨协程 Stop
// 必然构成数据竞争。监听消失说明旧服务的 accept 循环已经退出，直接重建就够；
// 旧服务对象留给进程退出回收，不额外持有可写状态。
// 参数 ctx：Agent 生命周期；参数 port：本机 SSH 端口。
func (a *Agent) restartSSHD(ctx context.Context, port int) {
	cleanup, err := a.ensureSSHD(ctx, port)
	if err != nil {
		a.sshMu.Lock()
		a.sshErr = err.Error()
		a.sshMu.Unlock()
		log.Warn().Err(err).Int("port", port).Msg("embedded sshd restart failed, will retry")
		return
	}
	a.setSSHCleanup(cleanup)
	log.Info().Int("port", port).Msg("embedded sshd restarted")
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
