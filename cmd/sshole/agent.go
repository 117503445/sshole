package main

import (
	"context"

	"github.com/rs/zerolog/log"
)

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")
}