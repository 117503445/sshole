package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/sshole/pkg/proto"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
)

const testToken = "test-token"

// freePort reserves an ephemeral port and releases it for the hub to bind.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release free port: %v", err)
	}
	return port
}

// startTestHub starts a Hub in-process with the given static agent mapping.
func startTestHub(t *testing.T, agents map[string]int) (*Hub, string) {
	t.Helper()
	return startTestHubTimeout(t, agents, 5*time.Second)
}

// startTestHubTimeout is startTestHub with a configurable pending timeout.
func startTestHubTimeout(t *testing.T, agents map[string]int, pendingTimeout time.Duration) (*Hub, string) {
	t.Helper()
	mappingFile := filepath.Join(t.TempDir(), "port_mapping.json")
	if agents != nil {
		if err := SaveMapping(mappingFile, &PortMapping{Agents: agents}); err != nil {
			t.Fatalf("seed mapping: %v", err)
		}
	}
	httpPort := freePort(t)
	h, err := NewHub(HubConfig{
		AuthToken:      testToken,
		HTTPAddr:       fmt.Sprintf("127.0.0.1:%d", httpPort),
		MappingFile:    mappingFile,
		PendingTimeout: pendingTimeout,
	})
	if err != nil {
		t.Fatalf("NewHub: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		// Release hubPort listeners so later tests can reuse the ports.
		h.mu.Lock()
		for _, ln := range h.listeners {
			ln.Close()
		}
		h.mu.Unlock()
	})
	go func() { _ = h.Start(ctx) }()

	addr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	// Wait until the HTTP server accepts connections.
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hub http server did not come up: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return h, addr
}

// dialRaw dials a websocket endpoint and returns the response on failure for status assertions.
func dialRaw(t *testing.T, url string, header http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
}

func authHeader(agent string) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+testToken)
	if agent != "" {
		h.Set("X-Agent", agent)
	}
	return h
}

// connectAgent attaches a fake agent control channel.
func connectAgent(t *testing.T, httpAddr, name string) *websocket.Conn {
	t.Helper()
	ws, resp, err := dialRaw(t, "ws://"+httpAddr+"/agent", authHeader(name))
	if err != nil {
		body := ""
		if resp != nil && resp.Body != nil {
			b, _ := io.ReadAll(resp.Body)
			body = string(b)
		}
		t.Fatalf("agent control dial: %v %s", err, body)
	}
	t.Cleanup(func() { ws.Close(websocket.StatusNormalClosure, "") })
	return ws
}

// waitAgentOnline polls ListAgents until the agent reports online.
func waitAgentOnline(t *testing.T, httpAddr, name string) {
	t.Helper()
	client := rpcv1connect.NewHoleServiceClient(http.DefaultClient, "http://"+httpAddr)
	deadline := time.Now().Add(5 * time.Second)
	for {
		req := connect.NewRequest(&rpcv1.ListAgentsRequest{})
		req.Header().Set("Authorization", "Bearer "+testToken)
		resp, err := client.ListAgents(context.Background(), req)
		if err == nil {
			for _, a := range resp.Msg.Agents {
				if a.AgentName == name && a.Online {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("agent %s did not come online", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// readOpen reads the next OPEN control message and returns its session ID.
func readOpen(t *testing.T, control *websocket.Conn) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	typ, data, err := control.Read(ctx)
	if err != nil {
		t.Fatalf("read OPEN: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected text control message, got %v", typ)
	}
	var msg proto.ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal control message: %v", err)
	}
	if msg.Type != "OPEN" || msg.SessionID == "" {
		t.Fatalf("expected OPEN with session id, got %s", string(data))
	}
	return msg.SessionID
}

// dialTunnel attaches a tunnel websocket for the given session and completes the handshake.
func dialTunnel(t *testing.T, httpAddr, agent, sessionID string) *websocket.Conn {
	t.Helper()
	header := authHeader(agent)
	header.Set("X-Session", sessionID)
	ws, resp, err := dialRaw(t, "ws://"+httpAddr+"/tunnel", header)
	if err != nil {
		body := ""
		if resp != nil && resp.Body != nil {
			b, _ := io.ReadAll(resp.Body)
			body = string(b)
		}
		t.Fatalf("tunnel dial: %v %s", err, body)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tunnel.SendHandshake(ctx, ws, sessionID); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
	t.Cleanup(func() { ws.Close(websocket.StatusNormalClosure, "") })
	return ws
}

// assertEcho writes payload to conn and requires the exact bytes back.
func assertEcho(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if string(buf) != string(payload) {
		t.Fatalf("echo mismatch: got %q want %q", buf, payload)
	}
}

func TestHealthz(t *testing.T) {
	_, addr := startTestHub(t, nil)
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("get healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Fatalf("healthz body = %q", body)
	}
}

func TestAgentControlAuth(t *testing.T) {
	_, addr := startTestHub(t, nil)

	// Missing token -> 401.
	h := http.Header{}
	h.Set("X-Agent", "a")
	_, resp, err := dialRaw(t, "ws://"+addr+"/agent", h)
	if err == nil {
		t.Fatal("expected dial error without token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}

	// Wrong token -> 401.
	h.Set("Authorization", "Bearer wrong")
	_, resp, err = dialRaw(t, "ws://"+addr+"/agent", h)
	if err == nil {
		t.Fatal("expected dial error with wrong token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}

	// Missing X-Agent -> 400.
	h = http.Header{}
	h.Set("Authorization", "Bearer "+testToken)
	_, resp, err = dialRaw(t, "ws://"+addr+"/agent", h)
	if err == nil {
		t.Fatal("expected dial error without X-Agent")
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", resp)
	}
}

func TestAgentRegistersPersistsAndGoesOffline(t *testing.T) {
	h, addr := startTestHub(t, nil)

	control := connectAgent(t, addr, "dyn")
	waitAgentOnline(t, addr, "dyn")

	// Dynamically registered agent gets a persisted port mapping.
	h.mu.RLock()
	state := h.agents["dyn"]
	h.mu.RUnlock()
	if state.HubPort < 10000 {
		t.Fatalf("unexpected hub port %d", state.HubPort)
	}
	pm, err := LoadMapping(h.cfg.MappingFile)
	if err != nil {
		t.Fatalf("load mapping: %v", err)
	}
	if pm.Agents["dyn"] != state.HubPort {
		t.Fatalf("mapping not persisted: %+v", pm.Agents)
	}

	// After the control channel closes, the agent must report offline.
	control.Close(websocket.StatusNormalClosure, "")
	client := rpcv1connect.NewHoleServiceClient(http.DefaultClient, "http://"+addr)
	deadline := time.Now().Add(5 * time.Second)
	for {
		req := connect.NewRequest(&rpcv1.ListAgentsRequest{})
		req.Header().Set("Authorization", "Bearer "+testToken)
		resp, err := client.ListAgents(context.Background(), req)
		if err == nil {
			for _, a := range resp.Msg.Agents {
				if a.AgentName == "dyn" && !a.Online {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("agent did not go offline after control close")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestRPCAuth(t *testing.T) {
	_, addr := startTestHub(t, nil)
	client := rpcv1connect.NewHoleServiceClient(http.DefaultClient, "http://"+addr)

	_, err := client.ListAgents(context.Background(), connect.NewRequest(&rpcv1.ListAgentsRequest{}))
	if err == nil {
		t.Fatal("expected unauthenticated error")
	}
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Fatalf("expected CodeUnauthenticated, got %v", err)
	}

	req := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	req.Header().Set("Authorization", "Bearer "+testToken)
	if _, err := client.ListAgents(context.Background(), req); err != nil {
		t.Fatalf("list agents with token: %v", err)
	}
}

func TestSSHConnClosedWhenAgentOffline(t *testing.T) {
	port := freePort(t)
	_, addr := startTestHub(t, map[string]int{"off": port})
	_ = addr

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial hub port: %v", err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected connection to be closed for offline agent")
	}
}

// TestSSHInitiatedForwarding is the regression test for the duplicate
// startForwarding bug: an SSH-initiated session must be forwarded exactly
// once, otherwise the tunnel websocket is read concurrently and the
// connection dies immediately.
func TestSSHInitiatedForwarding(t *testing.T) {
	port := freePort(t)
	_, addr := startTestHub(t, map[string]int{"a": port})

	control := connectAgent(t, addr, "a")
	waitAgentOnline(t, addr, "a")

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial hub port: %v", err)
	}
	defer conn.Close()

	sessionID := readOpen(t, control)
	ws := dialTunnel(t, addr, "a", sessionID)

	// Agent side echoes everything back.
	nc := tunnel.NetConn(context.Background(), ws)
	go func() { _, _ = io.Copy(nc, nc) }()

	assertEcho(t, conn, []byte("SSH-2.0-OpenSSH_10.5\r\n"))
	assertEcho(t, conn, []byte("ping"))

	// Larger payload exercises stream framing across websocket messages.
	big := make([]byte, 64*1024)
	for i := range big {
		big[i] = byte(i)
	}
	assertEcho(t, conn, big)

	// The connection must survive; with duplicate forwarding it is closed
	// as soon as the second forwarding pair touches the websocket.
	time.Sleep(300 * time.Millisecond)
	assertEcho(t, conn, []byte("still-alive"))
}

// TestEntryInitiatedForwarding covers the entry flow: entry opens the tunnel
// first, the hub asks the agent for a tunnel, then entry<->agent forwarding starts.
func TestEntryInitiatedForwarding(t *testing.T) {
	_, addr := startTestHub(t, nil)
	control := connectAgent(t, addr, "a")
	waitAgentOnline(t, addr, "a")

	sessionID := "entry-session-1"
	header := authHeader("a")
	header.Set("X-Session", sessionID)
	entryWS, _, err := dialRaw(t, "ws://"+addr+"/tunnel", header)
	if err != nil {
		t.Fatalf("entry tunnel dial: %v", err)
	}
	defer entryWS.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tunnel.SendHandshake(ctx, entryWS, sessionID); err != nil {
		t.Fatalf("entry handshake: %v", err)
	}

	// Hub should now ask the agent to open a tunnel for this session.
	got := readOpen(t, control)
	if got != sessionID {
		t.Fatalf("OPEN session = %q, want %q", got, sessionID)
	}
	agentWS := dialTunnel(t, addr, "a", sessionID)
	nc := tunnel.NetConn(context.Background(), agentWS)
	go func() { _, _ = io.Copy(nc, nc) }()

	// Entry side exchanges binary messages with the agent echo.
	if err := entryWS.Write(context.Background(), websocket.MessageBinary, []byte("hello-entry")); err != nil {
		t.Fatalf("entry write: %v", err)
	}
	readCtx, readCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readCancel()
	typ, data, err := entryWS.Read(readCtx)
	if err != nil {
		t.Fatalf("entry read: %v", err)
	}
	if typ != websocket.MessageBinary || string(data) != "hello-entry" {
		t.Fatalf("entry echo mismatch: %v %q", typ, data)
	}
}

func TestTunnelWSRejected(t *testing.T) {
	_, addr := startTestHub(t, nil)
	connectAgent(t, addr, "a")
	waitAgentOnline(t, addr, "a")

	// Wrong token -> 401.
	h := authHeader("a")
	h.Set("Authorization", "Bearer wrong")
	h.Set("X-Session", "s")
	_, resp, err := dialRaw(t, "ws://"+addr+"/tunnel", h)
	if err == nil {
		t.Fatal("expected dial error with wrong token")
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", resp)
	}

	// Unknown agent -> 403.
	h = authHeader("ghost")
	h.Set("X-Session", "s")
	_, resp, err = dialRaw(t, "ws://"+addr+"/tunnel", h)
	if err == nil {
		t.Fatal("expected dial error for unknown agent")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", resp)
	}

	// Missing X-Agent -> 400.
	h = http.Header{}
	h.Set("Authorization", "Bearer "+testToken)
	h.Set("X-Session", "s")
	_, resp, err = dialRaw(t, "ws://"+addr+"/tunnel", h)
	if err == nil {
		t.Fatal("expected dial error without X-Agent")
	}
	if resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", resp)
	}
}

// TestTunnelAgentMismatch: a tunnel for another agent's session must be rejected.
func TestTunnelAgentMismatch(t *testing.T) {
	port := freePort(t)
	_, addr := startTestHub(t, map[string]int{"a": port})
	control := connectAgent(t, addr, "a")
	connectAgent(t, addr, "b")
	waitAgentOnline(t, addr, "a")
	waitAgentOnline(t, addr, "b")

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial hub port: %v", err)
	}
	defer conn.Close()
	sessionID := readOpen(t, control)

	// Agent "b" tries to bind agent "a"'s session.
	h := authHeader("b")
	h.Set("X-Session", sessionID)
	ws, _, err := dialRaw(t, "ws://"+addr+"/tunnel", h)
	if err != nil {
		t.Fatalf("tunnel dial: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tunnel.SendHandshake(ctx, ws, sessionID); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
	_, _, err = ws.Read(ctx)
	if err == nil {
		t.Fatal("expected tunnel to be closed on agent mismatch")
	}
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected policy violation close, got %v", err)
	}
}

// TestPendingSessionTimeout: if the agent never binds a tunnel, the SSH
// connection must be closed after PendingTimeout.
func TestPendingSessionTimeout(t *testing.T) {
	port := freePort(t)
	_, addr := startTestHubTimeout(t, map[string]int{"a": port}, 300*time.Millisecond)
	control := connectAgent(t, addr, "a")
	waitAgentOnline(t, addr, "a")

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		t.Fatalf("dial hub port: %v", err)
	}
	defer conn.Close()
	_ = readOpen(t, control) // session opened, but agent never dials the tunnel

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	start := time.Now()
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("expected connection to be closed after pending timeout")
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("timeout close took too long: %v", elapsed)
	}
}

func TestFindAvailablePort(t *testing.T) {
	h := &Hub{ports: map[int]string{10000: "a", 10001: "b"}}
	if got := h.findAvailablePort(); got != 10002 {
		t.Fatalf("findAvailablePort = %d, want 10002", got)
	}
}

func TestHubConfigDefaults(t *testing.T) {
	cfg := (&HubConfig{}).withDefaults()
	if cfg.HTTPAddr != ":9000" {
		t.Fatalf("default HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.PendingTimeout <= 0 || cfg.TunnelDialTimeout <= 0 {
		t.Fatalf("default timeouts must be positive: %+v", cfg)
	}

	custom := (&HubConfig{HTTPAddr: ":1", PendingTimeout: time.Second, TunnelDialTimeout: 2 * time.Second}).withDefaults()
	if custom.HTTPAddr != ":1" || custom.PendingTimeout != time.Second || custom.TunnelDialTimeout != 2*time.Second {
		t.Fatalf("explicit config must be preserved: %+v", custom)
	}
}
