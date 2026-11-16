package agent

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestSSHProbeReportsNotListening 验证本机 SSH 没起来时，状态帧能给出明确原因。
func TestSSHProbeReportsNotListening(t *testing.T) {
	a := &Agent{cfg: AgentConfig{LocalPort: freeTCPPort(t)}}
	ready, reason := a.sshProbe()
	if ready {
		t.Fatal("closed port must not report ssh ready")
	}
	if reason == "" {
		t.Fatal("failed probe must carry a reason")
	}
}

// TestRestartSSHDRestoresListener 验证内嵌 sshd 掉线后能被重新拉起来。
//
// 这是 Hub 侧"在线但连不上"的真正修复点：监听消失必须重启，而不是让控制连接继续假装可用。
func TestRestartSSHDRestoresListener(t *testing.T) {
	port := freeTCPPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &Agent{cfg: AgentConfig{LocalPort: port}}
	cleanup, err := a.ensureSSHD(ctx, port)
	if err != nil {
		t.Fatalf("ensure sshd: %v", err)
	}
	a.setSSHCleanup(cleanup)
	if ready, reason := a.sshProbe(); !ready {
		t.Fatalf("sshd should be ready after start, got %q", reason)
	}

	a.closeSSHCleanup()
	if !waitPortClosed(port, 3*time.Second) {
		t.Fatalf("port %d still listening after cleanup", port)
	}
	if ready, _ := a.sshProbe(); ready {
		t.Fatal("probe must report not ready once the listener is gone")
	}

	a.restartSSHD(ctx, port)
	if ready, reason := a.sshProbe(); !ready {
		t.Fatalf("sshd should be listening again, got %q", reason)
	}
	a.closeSSHCleanup()
}

// waitPortClosed 轮询等待端口真正释放。
// 参数 port：目标端口；参数 timeout：最长等待时间。
func waitPortClosed(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !isPortListening(port) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// freeTCPPort 预留一个空闲端口并立即释放，交给被测服务重新绑定。
func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
