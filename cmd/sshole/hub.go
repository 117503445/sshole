package main

import (
	"context"

	"github.com/rs/zerolog/log"
)

func cmdHub(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")
}
