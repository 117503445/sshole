package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

type conn struct {
	Port          int32
	SshPublicKey  string
	SshPrivateKey string
}

func newConn() conn {
	c, err := newConnWithPort(0)
	if err != nil {
		panic(err)
	}
	return c
}

// HoleServer 是 hub 服务器的实现
// 如果 cli.Auth 不为空，则需要验证 auth

// 返回的 code 为 0 表示成功，message 为空字符串
// 每个 RPC 方法的错误码从 1 开始独立编号
type HoleServer struct {
	agents map[string]*rpcv1.Agent // name -> agent
	conns  map[string]conn         // name -> connection info (port, ssh keys)
	mu     sync.RWMutex

	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func newHoleServer() *HoleServer {
	privateKey, publicKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Panic().Err(err).Msg("failed to generate ed25519 key")
		return nil
	}
	return &HoleServer{
		agents:     make(map[string]*rpcv1.Agent),
		conns:      make(map[string]conn),
		privateKey: ed25519.PrivateKey(privateKey),
		publicKey:  ed25519.PublicKey(publicKey),
	}
}

// AgentCreate 注册 agent 到 hub

// 1. hub 分配一个 port 给 agent
// 2. hub 返回 public key 给 agent
func (s *HoleServer) AgentCreate(
	ctx context.Context,
	req *connect.Request[rpcv1.ApiRequest],
) (*connect.Response[rpcv1.ApiResponse], error) {
	log.Info().Msg("AgentCreate RPC")

	agentCreateReq := req.Msg.GetAgentCreate()
	if agentCreateReq == nil {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    1,
			Message: "Invalid request: missing agent_create payload",
		}), nil
	}

	if agentCreateReq.Name == "" {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    2,
			Message: "Invalid request: agent name cannot be empty",
		}), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查 agent 是否已存在
	if _, exists := s.agents[agentCreateReq.Name]; exists {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    3,
			Message: "Agent already exists",
		}), nil
	}

	// 为 agent 创建连接信息（包含 SSH 密钥对）
	connInfo, err := newConnWithPort(0)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create connection info")
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    4,
			Message: "Failed to create connection info",
		}), nil
	}

	// 创建 agent
	agent := &rpcv1.Agent{
		Name: agentCreateReq.Name,
		Port: uint32(connInfo.Port),
	}

	// 存储 agent 和连接信息
	s.agents[agentCreateReq.Name] = agent
	s.conns[agentCreateReq.Name] = connInfo

	log.Info().Str("name", agent.Name).Uint32("port", agent.Port).Msg("Agent created successfully")

	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    0,
		Message: "",
		Payload: &rpcv1.ApiResponse_AgentCreate{
			AgentCreate: &rpcv1.AgentCreateResponse{
				Agent:     agent,
				PublicKey: string(s.publicKey),
			},
		},
	}), nil
}

// AgentList 获取 agent 列表
func (s *HoleServer) AgentList(
	ctx context.Context,
	req *connect.Request[rpcv1.ApiRequest],
) (*connect.Response[rpcv1.ApiResponse], error) {
	log.Info().Msg("AgentList RPC")

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 构建 agent 列表
	agents := make([]*rpcv1.Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}

	log.Info().Int("count", len(agents)).Msg("Agent list retrieved successfully")

	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    0,
		Message: "",
		Payload: &rpcv1.ApiResponse_AgentList{
			AgentList: &rpcv1.AgentListResponse{
				Agent: agents,
			},
		},
	}), nil
}

// AgentGet 获取 agent 信息
func (s *HoleServer) AgentGet(
	ctx context.Context,
	req *connect.Request[rpcv1.ApiRequest],
) (*connect.Response[rpcv1.ApiResponse], error) {
	log.Info().Msg("AgentGet RPC")

	agentGetReq := req.Msg.GetAgentGet()
	if agentGetReq == nil {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    1,
			Message: "Invalid request: missing agent_get payload",
		}), nil
	}

	if agentGetReq.Name == "" {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    2,
			Message: "Invalid request: agent name cannot be empty",
		}), nil
	}

	s.mu.RLock()
	agent, exists := s.agents[agentGetReq.Name]
	s.mu.RUnlock()

	if !exists {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    3,
			Message: "Agent not found",
		}), nil
	}

	log.Info().Str("name", agent.Name).Uint32("port", agent.Port).Msg("Agent retrieved successfully")

	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    0,
		Message: "",
		Payload: &rpcv1.ApiResponse_AgentGet{
			AgentGet: &rpcv1.AgentGetResponse{
				Agent: agent,
			},
		},
	}), nil
}

// AgentAppendPublicKey 追加 public key 到 agent

// 1. hub 连接 agent 的 ssh
// 2. hub 追加 public key 到 agent 的 /tmp/sshole_agent/authorized_keys

func (s *HoleServer) AgentAppendPublicKey(
	ctx context.Context,
	req *connect.Request[rpcv1.ApiRequest],
) (*connect.Response[rpcv1.ApiResponse], error) {
	log.Info().Msg("AgentAppendPublicKey RPC")

	agentAppendReq := req.Msg.GetAgentAppendPublicKey()
	if agentAppendReq == nil {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    1,
			Message: "Invalid request: missing agent_append_public_key payload",
		}), nil
	}

	if agentAppendReq.Name == "" {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    2,
			Message: "Invalid request: agent name cannot be empty",
		}), nil
	}

	if agentAppendReq.PublicKey == "" {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    3,
			Message: "Invalid request: public key cannot be empty",
		}), nil
	}

	s.mu.RLock()
	agent, agentExists := s.agents[agentAppendReq.Name]
	connInfo, connExists := s.conns[agentAppendReq.Name]
	s.mu.RUnlock()

	if !agentExists || !connExists {
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    4,
			Message: "Agent not found",
		}), nil
	}

	// SSH 连接到 agent 并追加公钥
	err := s.appendPublicKeyToAgent(connInfo, agentAppendReq.PublicKey)
	if err != nil {
		log.Error().Err(err).Str("agent", agent.Name).Msg("Failed to append public key to agent")
		return connect.NewResponse(&rpcv1.ApiResponse{
			Code:    5,
			Message: "Failed to append public key to agent",
		}), nil
	}

	log.Info().Str("agent", agent.Name).Msg("Public key appended successfully")

	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    0,
		Message: "",
		Payload: &rpcv1.ApiResponse_AgentAppendPublicKey{
			AgentAppendPublicKey: &rpcv1.AgentAppendPublicKeyResponse{},
		},
	}), nil
}

// appendPublicKeyToAgent SSH 连接到 agent 并追加公钥到 authorized_keys
func (s *HoleServer) appendPublicKeyToAgent(connInfo conn, publicKey string) error {
	// 解析私钥
	privateKey, err := ssh.ParsePrivateKey([]byte(connInfo.SshPrivateKey))
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// 创建 SSH 配置
	config := &ssh.ClientConfig{
		User: "root", // 假设使用 root 用户
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 在生产环境中应该验证主机密钥
		Timeout:         10 * time.Second,
	}

	// 连接到 agent
	client, err := ssh.Dial("tcp", fmt.Sprintf("localhost:%d", connInfo.Port), config)
	if err != nil {
		return fmt.Errorf("failed to connect to agent: %w", err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 执行命令：创建目录并追加公钥
	// 使用 tee -a 来追加内容到文件，如果文件不存在会创建
	cmd := fmt.Sprintf(`mkdir -p /tmp/sshole_agent && echo "%s" | tee -a /tmp/sshole_agent/authorized_keys`, strings.TrimSpace(publicKey))

	// 执行命令
	var stderr io.Reader
	if stderr, err = session.StderrPipe(); err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := session.Run(cmd); err != nil {
		// 读取 stderr 来获取更详细的错误信息
		stderrBytes, _ := io.ReadAll(stderr)
		return fmt.Errorf("failed to execute command: %w, stderr: %s", err, string(stderrBytes))
	}

	return nil
}
