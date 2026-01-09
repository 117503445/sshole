package main

import (
	"context"

	"github.com/117503445/sshole/pkg/agent"
	"github.com/rs/zerolog/log"
)

func runAgent(ctx context.Context, cfg agent.AgentConfig) {
	a := agent.New(cfg)
	if err := a.Start(ctx); err != nil {
		log.Panic().Err(err).Msg("agent exited")
	}
}
