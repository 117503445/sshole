package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"connectrpc.com/connect"
	chserver "github.com/jpillora/chisel/server"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	rpcv1 "sshole/pkg/rpc/v1"
	"sshole/pkg/rpc/v1/rpcv1connect"
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

func cmdHub(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")

	go func() {
		holeServer := &HoleServer{}
		mux := http.NewServeMux()

		// 添加 /bin 路由处理器，用于返回二进制文件
		mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
			// 获取当前执行的二进制文件路径
			execPath, err := os.Executable()
			if err != nil {
				http.Error(w, "Failed to get executable path", http.StatusInternalServerError)
				return
			}

			// 打开二进制文件
			file, err := os.Open(execPath)
			if err != nil {
				http.Error(w, "Failed to open executable", http.StatusInternalServerError)
				return
			}
			defer file.Close()

			// 获取文件信息
			fileInfo, err := file.Stat()
			if err != nil {
				http.Error(w, "Failed to get file info", http.StatusInternalServerError)
				return
			}

			// 设置响应头
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=sshole")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

			// 将文件内容复制到响应中
			_, err = io.Copy(w, file)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to copy file to response")
				return
			}
		})

		// 添加 /healthz 路由处理器，用于健康检查
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		path, handler := rpcv1connect.NewHoleServiceHandler(holeServer)
		mux.Handle(path, handler)

		err := http.ListenAndServe(
			"0.0.0.0:9001",
			// Use h2c so we can serve HTTP/2 without TLS.
			h2c.NewHandler(mux, &http2.Server{}),
		)
		if err != nil {
			logger.Error().Err(err).Send()
		}
	}()

	// 创建服务器配置
	config := chserver.Config{
		Reverse: true, // 启用反向模式
		Proxy:   "http://localhost:9001",
		Auth:    cli.Hub.Auth,
	}

	// 创建服务器实例
	s, err := chserver.NewServer(&config)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create server")
	}

	// 设置服务器参数
	s.Debug = true // 启用调试模式

	// 启动服务器在指定端口
	if err := s.Start("", "9000"); err != nil {
		logger.Panic().Err(err).Msg("Failed to start server")
	}

	// 等待服务器关闭
	if err := s.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Server closed unexpectedly")
	}
}
