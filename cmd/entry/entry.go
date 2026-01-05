package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/goutils"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/117503445/sshole/pkg/utils"
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

func privateKeyToSSHFormat(privateKey ed25519.PrivateKey) ([]byte, error) {
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	return pem.EncodeToMemory(block), nil
}

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	{
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Panic().Err(err).Msg("failed to get user home directory")
		}
		if cli.PrivateKey == "" {
			cli.PrivateKey = filepath.Join(homeDir, ".ssh", "id_ed25519")
		}
		if cli.PublicKey == "" {
			cli.PublicKey = filepath.Join(homeDir, ".ssh", "id_ed25519.pub")
		}

		if !goutils.FileExists(cli.PrivateKey) && !goutils.FileExists(cli.PublicKey) {
			logger.Info().Msg("Generating ed25519 key pair...")

			publicKey, privateKey, err := ed25519.GenerateKey(nil)
			if err != nil {
				logger.Panic().Err(err).Msg("Failed to generate ed25519 key pair")
			}

			privateKeySSH, err := privateKeyToSSHFormat(ed25519.PrivateKey(privateKey))
			if err != nil {
				logger.Panic().Err(err).Msg("Failed to convert private key to SSH format")
			}
			publicKeySSH, err := publicKeyToSSHFormat(ed25519.PublicKey(publicKey))
			if err != nil {
				logger.Panic().Err(err).Msg("Failed to convert public key to SSH format")
			}

			// 写入私钥文件
			if err := goutils.WriteText(cli.PrivateKey, string(privateKeySSH)); err != nil {
				logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to write private key")
			}
			if err := os.Chmod(cli.PrivateKey, 0600); err != nil {
				logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to set private key permissions")
			}

			// 写入公钥文件
			if err := goutils.WriteText(cli.PublicKey, publicKeySSH); err != nil {
				logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to write public key")
			}
			if err := os.Chmod(cli.PublicKey, 0644); err != nil {
				logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to set public key permissions")
			}

			logger.Info().Str("private_key", cli.PrivateKey).Str("public_key", cli.PublicKey).Msg("Key pair generated and saved")

		} else if goutils.FileExists(cli.PrivateKey) && !goutils.FileExists(cli.PublicKey) {
			logger.Panic().Msg("private key file exists, but public key file does not exist")
		} else if !goutils.FileExists(cli.PrivateKey) && goutils.FileExists(cli.PublicKey) {
			logger.Panic().Msg("public key file exists, but private key file does not exist")
		}

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

	// 3. 将 cli.PublicKey 读取，写入到 agent 的 authorized_keys 文件中，通过调用 hub 的 AgentAppendPublicKey RPC 实现
	logger.Info().Str("public_key_path", cli.PublicKey).Msg("Reading public key...")

	sshPublicKeyBytes, err := os.ReadFile(cli.PublicKey)
	if err != nil {
		logger.Panic().Err(err).Str("path", cli.PublicKey).Msg("Failed to read public key file")
	}

	sshPublicKey := strings.TrimSpace(string(sshPublicKeyBytes))

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

	// 4. 建立 tunnel 连接，将本地端口映射到 hub 的 agent port
	logger.Info().Msg("Creating tunnel connection...")

	// Create TunnelManager for entry
	tm := tunnel.NewTunnelManager("")
	defer tm.Close()

	// Build tunnel URL from hub server
	tunnelURL := strings.TrimSuffix(cli.HubServer, "/") + "/tunnel"

	// Connect to hub
	conn, err := tm.Connect(tunnelURL, cli.Auth)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to connect to hub")
	}

	logger.Info().
		Str("conn_id", conn.GetID()).
		Msg("Connected to hub")

	// Create LocalToRemote tunnel: local listens on 'cli.SshPort', forwards to hub's 'agent.Port'
	tunnelID, err := tm.AddTunnel(conn.GetID(), "localToRemote", cli.SshPort, int(agent.Port))
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create tunnel")
	}

	logger.Info().
		Str("tunnel_id", tunnelID).
		Int("local_port", cli.SshPort).
		Uint32("hub_port", agent.Port).
		Msg("Tunnel created: local -> hub -> agent SSH")

	go func() {
		time.Sleep(3 * time.Second)

		privateKeyPEM, err := os.ReadFile(cli.PrivateKey)
		if err != nil {
			logger.Panic().Err(err).Str("path", cli.PrivateKey).Msg("Failed to read private key file")
		}

		result, err := utils.SshExecute(ctx, utils.SshExecuteParams{
			Host:          "localhost",
			Port:          cli.SshPort,
			User:          "root",
			Command:       "echo 'SSH connection successful'; hostname; whoami",
			PrivateKeyPem: privateKeyPEM,
		})
		if err != nil {
			logger.Error().Err(err).Msg("Failed to execute SSH command")
		}
		logger.Info().Str("output", result.Output).Msg("SSH command executed")
	}()

	// 等待连接断开
	for conn.IsConnected() {
		time.Sleep(1 * time.Second)
	}

	logger.Info().Msg("Connection closed")
}
