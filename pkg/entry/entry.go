package entry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/sshole/pkg/agent"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

type EntryConfig struct {
	HubAddr        string
	Token          string
	AgentName      string
	EntryPort      int
	PublicKeyPath  string
	PrivateKeyPath string
}

type Entry struct {
	cfg EntryConfig
}

func New(cfg EntryConfig) *Entry {
	if cfg.EntryPort == 0 {
		cfg.EntryPort = 22222
	}
	return &Entry{cfg: cfg}
}

func (e *Entry) Start(ctx context.Context) error {
	agent, err := e.pickAgent(ctx)
	if err != nil {
		return err
	}
	pubKey, err := e.readPublicKey()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read public key")
	} else {
		log.Info().Str("agent", agent.AgentName).Str("pub_key", pubKey).Msg("append known_hosts")
		if err := e.appendKnownHost(ctx, agent.AgentName, pubKey); err != nil {
			log.Warn().Err(err).Msg("failed to push known_hosts to agent")
		}
		if err := ensureKnownHost(e.cfg.EntryPort); err != nil {
			log.Warn().Err(err).Msg("failed to append known_hosts")
		}
	}
	// 启动就把 Agent 的真实状态打出来：是哪台机器、哪个构建、本机 SSH 通不通。
	log.Info().Str("agent", agent.AgentName).Str("state", e.describeAgent(ctx, agent.AgentName)).Msg("agent state")

	listenAddr := fmt.Sprintf(":%d", e.cfg.EntryPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	log.Info().Str("addr", listenAddr).Str("agent", agent.AgentName).Int32("hub_port", agent.HubPort).Msg("entry listening")

	// Test SSH connectivity in background
	go func() {
		time.Sleep(2 * time.Second) // Wait a bit for tunnel to be ready
		if err := e.testSSHConnectivity(); err != nil {
			// 失败时补一句 Hub 侧看到的状态，避免只剩 handshake EOF 这种无法判断的报错。
			log.Warn().Err(err).Str("agent", agent.AgentName).Str("hub_view", e.describeAgent(ctx, agent.AgentName)).Msg("SSH connectivity test failed")
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go e.handleConn(ctx, conn, int(agent.HubPort))
	}
}

func (e *Entry) pickAgent(ctx context.Context) (*rpcv1.AgentInfo, error) {
	client := rpcv1connect.NewHoleServiceClient(httpClient, e.cfg.HubAddr)
	req := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	if e.cfg.Token != "" {
		req.Header().Set("Authorization", "Bearer "+e.cfg.Token)
	}
	resp, err := client.ListAgents(ctx, req)
	if err != nil {
		return nil, err
	}
	var chosen *rpcv1.AgentInfo
	if e.cfg.AgentName == "" {
		if len(resp.Msg.Agents) == 1 {
			chosen = resp.Msg.Agents[0]
		} else {
			return nil, fmt.Errorf("multiple agents available, specify AGENT_NAME")
		}
	} else {
		for _, a := range resp.Msg.Agents {
			if a.AgentName == e.cfg.AgentName {
				chosen = a
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("agent %s not found", e.cfg.AgentName)
		}
	}
	return chosen, nil
}

func (e *Entry) appendKnownHost(ctx context.Context, agentName, pubKey string) error {
	client := rpcv1connect.NewHoleServiceClient(httpClient, e.cfg.HubAddr)
	req := connect.NewRequest(&rpcv1.AppendKnownHostRequest{
		AgentName: agentName,
		PublicKey: pubKey,
	})
	if e.cfg.Token != "" {
		req.Header().Set("Authorization", "Bearer "+e.cfg.Token)
	}
	_, err := client.AppendKnownHost(ctx, req)
	return err
}

func (e *Entry) handleConn(ctx context.Context, local net.Conn, hubPort int) {
	defer local.Close()

	// Get agent info
	agent, err := e.pickAgent(ctx)
	if err != nil {
		log.Error().Err(err).Msg("pick agent failed")
		return
	}

	// Create WebSocket URL for tunnel endpoint
	tunnelURL := fmt.Sprintf("%s/tunnel", e.cfg.HubAddr)
	if !strings.HasPrefix(tunnelURL, "ws") {
		tunnelURL = strings.Replace(tunnelURL, "http", "ws", 1)
	}

	// Generate session ID
	sessionID := uuid.New().String()

	// Prepare headers
	header := http.Header{}
	header.Set("X-Agent", agent.AgentName)
	header.Set("X-Session", sessionID)
	if e.cfg.Token != "" {
		header.Set("Authorization", "Bearer "+e.cfg.Token)
	}

	// Connect to WebSocket
	ws, _, err := websocket.Dial(ctx, tunnelURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		log.Error().Err(err).Str("url", tunnelURL).Str("hub_view", e.describeAgent(ctx, agent.AgentName)).Msg("dial websocket failed")
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "closed")

	// Send handshake
	if err := tunnel.SendHandshake(ctx, ws, sessionID); err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("send handshake failed")
		return
	}

	// Create net.Conn from WebSocket for easy forwarding
	remote := tunnel.NetConn(ctx, ws)
	defer remote.Close()

	// Forward data bidirectionally
	go func() {
		_, _ = ioCopy(remote, local)
		remote.Close()
	}()
	_, _ = ioCopy(local, remote)
}

func hostFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

var httpClient = http.DefaultClient

// agentHealth 是 Hub /api/agents 返回的单条运维视图，只取排查连接问题需要的字段。
type agentHealth struct {
	// AgentName 是 Agent 名称。
	AgentName string `json:"agent_name"`
	// Online 表示 Hub 当前持有该 Agent 的控制连接。
	Online bool `json:"online"`
	// LastSeenMS 是最后一次证明控制连接活着的时间。
	LastSeenMS int64 `json:"last_seen_ms"`
	// StatusReported 表示该 Agent 会上报状态帧。
	StatusReported bool `json:"status_reported"`
	// Hostname 是 Agent 所在机器名。
	Hostname string `json:"hostname"`
	// Version 是 Agent 构建版本。
	Version string `json:"version"`
	// SSHPort 是 Agent 本机 SSH 端口。
	SSHPort int `json:"ssh_port"`
	// SSHReady 表示 Agent 本机 SSH 可连接。
	SSHReady bool `json:"ssh_ready"`
	// SSLError 是最近一次本机 SSH 不可用的原因。
	SSLError string `json:"ssh_error"`
}

// describeAgent 向 Hub 查询目标 Agent 的真实状态，把"连不上"变成一句能读懂的话。
//
// 机器直接关机时 TCP 不会发 FIN，Hub 以前会长期以为在线，Entry 只能报
// handshake EOF；现在能明确区分"机器不在线"和"机器在线但本机 SSH 没起来"。
// 参数 ctx：请求上下文；参数 agentName：目标 Agent 名称。
func (e *Entry) describeAgent(ctx context.Context, agentName string) string {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	endpoint := strings.TrimSuffix(e.cfg.HubAddr, "/") + "/api/agents"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "查询 Hub 状态失败: " + err.Error()
	}
	if e.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.Token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "查询 Hub 状态失败: " + err.Error()
	}
	defer resp.Body.Close()
	var body struct {
		Agents []agentHealth `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "解析 Hub 状态失败: " + err.Error()
	}
	for _, a := range body.Agents {
		if a.AgentName != agentName {
			continue
		}
		if !a.StatusReported {
			return fmt.Sprintf("online=%v，该 Agent 版本过旧不会上报状态，请更新到最新构建", a.Online)
		}
		lastSeen := "从未"
		if a.LastSeenMS > 0 {
			lastSeen = time.UnixMilli(a.LastSeenMS).Format(time.RFC3339)
		}
		sshState := "未监听"
		if a.SSHReady {
			sshState = "就绪"
		} else if a.SSLError != "" {
			sshState = "未监听（" + a.SSLError + "）"
		}
		if !a.Online {
			// 状态帧是断开前最后一次上报的，只能当作"最后已知状态"，不能读成现在还可用。
			return fmt.Sprintf("online=false（Hub 已判定离线，机器多半已关机或断网）最后已知状态 hostname=%s version=%s ssh=%d %s last_seen=%s", a.Hostname, a.Version, a.SSHPort, sshState, lastSeen)
		}
		return fmt.Sprintf("online=true hostname=%s version=%s ssh=%d %s last_seen=%s", a.Hostname, a.Version, a.SSHPort, sshState, lastSeen)
	}
	return "Hub 未登记该 Agent"
}

var ioCopy = func(dst net.Conn, src net.Conn) (int64, error) {
	return io.Copy(dst, src)
}

func (e *Entry) readPublicKey() (string, error) {
	path := e.cfg.PublicKeyPath
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// ensureKnownHost 确保本地 ~/.ssh/known_hosts 包含 Agent 的主机公钥。
// 这样 SSH 客户端首次连接时不会弹出 "The authenticity of host can't be established" 提示。
//
// 原理：Agent 使用固定的内置主机密钥（见 pkg/agent/hostkey.go），Entry 启动时
// 将该公钥追加到本地 known_hosts，实现无感知的 SSH 连接。
func ensureKnownHost(entryPort int) error {
	// 获取当前用户主目录
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	// 确保 ~/.ssh 目录存在，权限 700（SSH 标准要求）
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	// known_hosts 文件路径
	knownHosts := filepath.Join(sshDir, "known_hosts")
	// 构造条目格式: [localhost]:<port> <公钥类型> <公钥内容> <注释>
	entry := fmt.Sprintf("[localhost]:%d %s\n", entryPort, agent.HostPublicKey())
	// 如果已存在则跳过，避免重复写入
	if data, err := os.ReadFile(knownHosts); err == nil && strings.Contains(string(data), entry) {
		return nil
	}
	// 追加到 known_hosts，权限 600（SSH 标准要求）
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}

// testSSHConnectivity 测试 SSH 隧道连通性。
// 如果配置了私钥，使用公钥认证；否则跳过主机密钥验证。
func (e *Entry) testSSHConnectivity() error {
	log.Info().Msg("Testing SSH connectivity...")

	// 构建 SSH 客户端配置
	config := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	setupPrivateKey := func() {
		// 如果配置了私钥，使用公钥认证
		if e.cfg.PrivateKeyPath != "" {
			// 解析私钥路径（处理 ~ 前缀）
			keyPath := e.cfg.PrivateKeyPath
			if strings.HasPrefix(keyPath, "~") {
				home, err := os.UserHomeDir()
				if err != nil {
					log.Error().Err(err).Msg("get home dir failed")
					return
				}
				keyPath = filepath.Join(home, strings.TrimPrefix(keyPath, "~"))
			}

			// 读取并解析私钥
			keyBytes, err := os.ReadFile(keyPath)
			if err != nil {
				log.Error().Err(err).Msg("read private key failed")
				return
			}
			signer, err := ssh.ParsePrivateKey(keyBytes)
			if err != nil {
				log.Error().Err(err).Msg("parse private key failed")
				return
			}
			config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	}
	setupPrivateKey()

	// 连接到本地 Entry 端口
	addr := fmt.Sprintf("localhost:%d", e.cfg.EntryPort)
	log.Info().Str("addr", addr).Msg("Connecting to SSH tunnel...")
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("dial SSH tunnel: %w", err)
	}
	defer client.Close()

	// 创建会话并执行测试命令
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = os.Stderr

	log.Info().Msg("Executing test command...")
	err = session.Run("echo 'SSH tunnel test successful'")
	if err != nil {
		return fmt.Errorf("execute test command: %w", err)
	}

	output := strings.TrimSpace(stdoutBuf.String())
	log.Info().Str("output", output).Msg("SSH connectivity test completed successfully")
	return nil
}
