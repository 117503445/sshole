package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/utils"
	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

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

	// 3. 建立 chisel 连接，将本地端口映射到 hub 的 agent port
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
		result, err := utils.SshExecute(ctx, utils.SshExecuteParams{
			Host:    "localhost",
			Port:    cli.SshPort,
			User:    "root",
			Command: "echo 'SSH connection successful'; hostname; whoami",
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
