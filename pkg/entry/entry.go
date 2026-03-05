package entry

import (
	"bytes"
	"context"
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
			log.Warn().Err(err).Msg("SSH connectivity test failed")
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
		log.Error().Err(err).Str("url", tunnelURL).Msg("dial websocket failed")
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
