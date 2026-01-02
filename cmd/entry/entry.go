package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/goutils"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/utils"
	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/ssh"
)

func publicKeyToPEM(publicKey ed25519.PublicKey) string {
	pubBytes, _ := x509.MarshalPKIXPublicKey(publicKey)
	pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return string(pubPem)
}

func privateKeyToPEM(privateKey ed25519.PrivateKey) string {
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	privPem := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	return string(privPem)
}

func publicKeyToSSHFormat(publicKey ed25519.PublicKey) (string, error) {
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to create SSH public key: %w", err)
	}
	return string(ssh.MarshalAuthorizedKey(sshPublicKey)), nil
}

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	if cli.PrivateKey == "" && cli.PublicKey == "" {
		// 1. 生成 ed25519 密钥对
		// 2. 将公钥写入到 PublicKey 路径，私钥写入到 PrivateKey 路径，都用 pam 格式，并设置正确的权限
		logger.Info().Msg("Generating ed25519 key pair...")

		publicKey, privateKey, err := ed25519.GenerateKey(nil)
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to generate ed25519 key pair")
		}

		privateKeyPEM := privateKeyToPEM(ed25519.PrivateKey(privateKey))
		publicKeyPEM := publicKeyToPEM(ed25519.PublicKey(publicKey))

		// 写入私钥文件
		if err := goutils.WriteText(cli.PrivateKey, privateKeyPEM); err != nil {
			logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to write private key")
		}
		if err := os.Chmod(cli.PrivateKey, 0600); err != nil {
			logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to set private key permissions")
		}

		// 写入公钥文件
		if err := goutils.WriteText(cli.PublicKey, publicKeyPEM); err != nil {
			logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to write public key")
		}
		if err := os.Chmod(cli.PublicKey, 0644); err != nil {
			logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to set public key permissions")
		}

		logger.Info().Str("private_key", cli.PrivateKey).Str("public_key", cli.PublicKey).Msg("Key pair generated and saved")
	}

	// 创建 RPC 客户端
	rpcClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, cli.HubServer)

	var agent *rpcv1.Agent

	// 1. 如果 cli.AgentName 为空，则调用 hub 的 list agent。如果列表为 1，则 agent 信息取第一个。否则，打印所有 agent name 并退出。
	if cli.AgentName == "" {
		logger.Info().Msg("AgentName is empty, listing agents...")

		listResp, err := rpcClient.AgentList(ctx, connect.NewRequest(&rpcv1.ApiRequest{
			Auth: cli.Auth,
		}))
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to list agents")
		}
		if listResp.Msg.Code != 0 {
			logger.Panic().Int64("code", listResp.Msg.Code).
				Str("message", listResp.Msg.Message).
				Msg("Failed to list agents")
		}

		agents := listResp.Msg.GetAgentList().GetAgent()
		if len(agents) == 0 {
			logger.Panic().Msg("No agents available")
		}
		if len(agents) == 1 {
			agent = agents[0]
			logger.Info().Str("agent", agent.Name).Uint32("port", agent.Port).Msg("Using the only available agent")
		} else {
			fmt.Println("Available agents:")
			for _, a := range agents {
				fmt.Printf("- %s (port: %d)\n", a.Name, a.Port)
			}
			logger.Info().Msg("Multiple agents available, please specify --agent-name")
			return
		}
	} else {
		// 2. 如果 agent name 不为空，则调用 hub 的 get agent。如果 agent 不存在，则退出。
		logger.Info().Str("agent", cli.AgentName).Msg("Getting agent info...")

		getResp, err := rpcClient.AgentGet(ctx, connect.NewRequest(&rpcv1.ApiRequest{
			Auth: cli.Auth,
			Payload: &rpcv1.ApiRequest_AgentGet{
				AgentGet: &rpcv1.AgentGetRequest{
					Name: cli.AgentName,
				},
			},
		}))
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to get agent")
		}
		if getResp.Msg.Code != 0 {
			logger.Panic().Int64("code", getResp.Msg.Code).
				Str("message", getResp.Msg.Message).
				Msg("Failed to get agent")
		}
		agent = getResp.Msg.GetAgentGet().GetAgent()
	}

	logger.Info().Str("agent", agent.Name).Uint32("port", agent.Port).Int("ssh_port", cli.SshPort).Msg("Agent selected")

	// 3. 将 cli.PublicKey 读取，转为 ssh 格式，写入到 agent 的 authorized_keys 文件中，通过调用 hub 的 AgentAppendPublicKey RPC 实现
	logger.Info().Str("public_key_path", cli.PublicKey).Msg("Reading public key...")

	publicKeyPEM, err := os.ReadFile(cli.PublicKey)
	if err != nil {
		logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to read public key file")
	}

	// 解析PEM格式的公钥
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		logger.Panic().Msg("Failed to decode PEM block")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to parse public key")
	}

	ed25519PublicKey, ok := publicKey.(ed25519.PublicKey)
	if !ok {
		logger.Panic().Msg("Public key is not an ed25519 key")
	}

	// 转换为SSH格式
	sshPublicKey, err := publicKeyToSSHFormat(ed25519PublicKey)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to convert public key to SSH format")
	}

	// 通过RPC调用写入到agent的authorized_keys
	logger.Info().Str("agent", agent.Name).Msg("Appending public key to agent...")
	appendResp, err := rpcClient.AgentAppendPublicKey(ctx, connect.NewRequest(&rpcv1.ApiRequest{
		Auth: cli.Auth,
		Payload: &rpcv1.ApiRequest_AgentAppendPublicKey{
			AgentAppendPublicKey: &rpcv1.AgentAppendPublicKeyRequest{
				Name:      agent.Name,
				PublicKey: sshPublicKey,
			},
		},
	}))
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to append public key to agent")
	}
	if appendResp.Msg.Code != 0 {
		logger.Panic().Int64("code", appendResp.Msg.Code).
			Str("message", appendResp.Msg.Message).
			Msg("Failed to append public key to agent")
	}

	logger.Info().Str("agent", agent.Name).Msg("Public key appended to agent successfully")

	// 4. 建立 chisel 连接，将本地端口映射到 hub 的 agent port
	logger.Info().Msg("Creating chisel connection...")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.HubServer,
		Remotes: []string{fmt.Sprintf("localhost:%d:localhost:%d", cli.SshPort, agent.Port)}, // 将本地 cli.SshPort 端口映射到 hub 的 agent.Port
		Auth:    cli.Auth,
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create chisel client")
	}

	if err := c.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("Failed to start chisel client")
	}

	go func() {
		time.Sleep(3 * time.Second)

		privateKeyPEM, err := os.ReadFile(cli.PrivateKey)
		if err != nil {
			logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to read private key file")
		}

		result, err := utils.SshExecute(ctx, utils.SshExecuteParams{
			Host:    "localhost",
			Port:    cli.SshPort,
			User:    "root",
			Command: "echo 'SSH connection successful'; hostname; whoami",
			PrivateKeyPem: privateKeyPEM,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Failed to execute SSH command")
		}
		logger.Info().Str("output", result.Output).Msg("SSH command executed")
	}()

	// 等待 chisel 连接结束
	if err := c.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Failed to wait chisel client")
	}
}
