package main

import (
	"context"
	"net/http"
	rpcv1 "sshole/pkg/rpc/v1"
	"sshole/pkg/rpc/v1/rpcv1connect"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"

	chclient "github.com/jpillora/chisel/client"
)

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	if false {
		client := rpcv1connect.NewHoleServiceClient(
			http.DefaultClient,
			"http://localhost:9000",
		)
		res, err := client.AcquireConnection(
			context.Background(),
			connect.NewRequest(&rpcv1.AcquireConnectionRequest{}),
		)
		if err != nil {
			logger.Panic().Err(err).Msg("Failed to create conn")
		}
		logger.Info().Interface("Msg", res.Msg).Send()
	}

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.Entry.HubServer,
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
