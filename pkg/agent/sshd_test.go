package agent

import (
	"context"
	"net"
	"testing"
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

// TestRestartSSHDRestoresListener 验证监听消失后看护能在同一端口重新把内嵌 sshd 拉起来。
//
// 这是"Hub 显示在线但连不上"的真正修复点：本机 SSH 掉线必须能自愈并如实上报，
// 而不是让控制连接继续假装可用。测试不调用 Stop：sshdev 的 Start/Stop 对 listener
// 字段没有同步，跨协程 Stop 会被 -race 判为数据竞争。
func TestRestartSSHDRestoresListener(t *testing.T) {
	port := freeTCPPort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a := &Agent{cfg: AgentConfig{LocalPort: port}}
	if ready, _ := a.sshProbe(); ready {
		t.Fatalf("port %d must be free before restart", port)
	}

	a.restartSSHD(ctx, port)
	if ready, reason := a.sshProbe(); !ready {
		t.Fatalf("restartSSHD should leave a listening sshd, got %q", reason)
	}
	if _, reason := a.sshProbe(); reason != "" {
		t.Fatalf("ready probe must clear the reason, got %q", reason)
	}
}

// TestSSHProbeSurfacesRecordedFailure 验证重启失败原因会进入状态帧，而不是只剩"未监听"。
//
// 现场最需要的是"为什么连不上"，只报 false 等于没报。
func TestSSHProbeSurfacesRecordedFailure(t *testing.T) {
	a := &Agent{cfg: AgentConfig{LocalPort: freeTCPPort(t)}}
	a.sshMu.Lock()
	a.sshErr = "boom"
	a.sshMu.Unlock()
	if ready, reason := a.sshProbe(); ready || reason != "boom" {
		t.Fatalf("probe must surface the recorded failure, got ready=%v reason=%q", ready, reason)
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
