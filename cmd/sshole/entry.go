package main

import (
	"context"

	"github.com/rs/zerolog/log"

	chclient "github.com/jpillora/chisel/client"
)

func cmdEntry(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting entry")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  "localhost:9000",
		Remotes: []string{"8081:localhost:8000"},
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
