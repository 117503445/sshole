package main

import (
	"context"

	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	// 1. 如果 cli.AgentName 为空，则调用 hub 的 list agent。如果列表为 1，则 agent 信息取第一个。否则，打印所有 agent name 并退出。
	// 2. 如果 agent name 不为空，则调用 hub 的 get agent。如果 agent 不存在，则退出。
	// 3. 建立 chisel 连接，把 agent 的 port 映射到本地的 SshPort 端口

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.HubServer,
		Remotes: []string{"24:localhost:23"}, // 把服务器的 port 23 端口映射到本地的 SshPort 24 端口
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create chisel client")
	}
	if err := c.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("Failed to start chisel client")
	}
	if err := c.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Failed to wait chisel client")
	}
}
