package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"
	"sync"

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

func newConnWithPort(port int32) (conn, error) {
	// port: 随机一个本地可用的 tcp port 或使用指定端口
	// ssh key: 随机生成一个 ed25519 key

	// 检查指定端口是否可用
	var listener net.Listener
	var err error

	if port != 0 {
		// 尝试监听指定端口
		listener, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			// 指定端口不可用，回退到随机端口
			listener, err = net.Listen("tcp", "localhost:0")
			if err != nil {
				return conn{}, err
			}
		}
	} else {
		// 使用随机端口
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return conn{}, err
		}
	}

	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port

	// 生成 ed25519 SSH 密钥对
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return conn{}, err
	}

	// 将密钥转换为 SSH 格式
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return conn{}, err
	}

	// 将私钥转换为OpenSSH格式
	sshPrivKey, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return conn{}, err
	}

	return conn{
		Port:          int32(actualPort),
		SshPublicKey:  string(ssh.MarshalAuthorizedKey(sshPubKey)),
		SshPrivateKey: string(pem.EncodeToMemory(sshPrivKey)),
	}, nil
}

func newConn() conn {
	c, err := newConnWithPort(0)
	if err != nil {
		panic(err)
	}
	return c
}

type HoleServer struct {
	// id -> conn
	conns map[string]conn
	mu    sync.RWMutex
}

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
