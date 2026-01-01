package main

import (
	"context"
	"crypto/ed25519"
	"sync"

	"connectrpc.com/connect"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/rs/zerolog/log"
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

// 返回的 code 为 0 表示成功，其他表示错误
// message 为错误信息
type HoleServer struct {
	// id -> conn
	// conns map[string]conn

	agents map[string]rpcv1.Agent // name -> agent
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
		agents:     make(map[string]rpcv1.Agent),
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
			Code:    400,
			Message: "Invalid request: missing agent_create payload",
		}), nil
	}

	// TODO: 实现AgentCreate逻辑
	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    200,
		Message: "Agent created successfully",
		Payload: &rpcv1.ApiResponse_AgentCreate{
			AgentCreate: &rpcv1.AgentCreateResponse{
				Agent: &rpcv1.Agent{
					Name: agentCreateReq.Name,
					Port: 22222, // 默认端口
				},
				PublicKey: "dummy-public-key", // TODO: 生成真实的密钥
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

	// TODO: 实现AgentList逻辑
	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    200,
		Message: "Agent list retrieved successfully",
		Payload: &rpcv1.ApiResponse_AgentList{
			AgentList: &rpcv1.AgentListResponse{
				Agent: []*rpcv1.Agent{}, // 空列表
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
			Code:    400,
			Message: "Invalid request: missing agent_get payload",
		}), nil
	}

	// TODO: 实现AgentGet逻辑
	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    200,
		Message: "Agent retrieved successfully",
		Payload: &rpcv1.ApiResponse_AgentGet{
			AgentGet: &rpcv1.AgentGetResponse{
				Agent: &rpcv1.Agent{
					Name: agentGetReq.Name,
					Port: 22222,
				},
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
			Code:    400,
			Message: "Invalid request: missing agent_append_public_key payload",
		}), nil
	}

	// TODO: 实现AgentAppendPublicKey逻辑
	return connect.NewResponse(&rpcv1.ApiResponse{
		Code:    200,
		Message: "Public key appended successfully",
		Payload: &rpcv1.ApiResponse_AgentAppendPublicKey{
			AgentAppendPublicKey: &rpcv1.AgentAppendPublicKeyResponse{},
		},
	}), nil
}
