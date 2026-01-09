package main

import (
	"context"

	"github.com/117503445/sshole/pkg/entry"
	"github.com/rs/zerolog/log"
)

func runEntry(ctx context.Context, cfg entry.EntryConfig) {
	e := entry.New(cfg)
	if err := e.Start(ctx); err != nil {
		log.Panic().Err(err).Msg("entry exited")
	}
}
