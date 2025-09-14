package main

import (
	"context"
	"sshole/pkg/common"

	"github.com/rs/zerolog/log"
)

func cmdHub(ctx context.Context) {
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "hub",
	})

	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")
}
