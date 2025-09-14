package main

import (
	"context"
	"sshole/pkg/common"

	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "main",
	})

	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting sshole")
}
