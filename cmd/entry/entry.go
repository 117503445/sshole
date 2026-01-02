package main

import (
	"context"

	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.HubServer,
		Remotes: []string{"24:localhost:23"}, // 把服务器的 23 端口映射到本地的 24 端口
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
