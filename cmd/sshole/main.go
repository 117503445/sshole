package main

import (
	"github.com/117503445/goutils"
	"github.com/alecthomas/kong"
	"github.com/rs/zerolog/log"
)

func main() {
	goutils.InitZeroLog()

	kongCtx := kong.Parse(&cli)
	log.Info().Interface("cli", cli).Msg("Starting sshole")
	if err := kongCtx.Run(); err != nil {
		log.Panic().Err(err).Msg("run failed")
	}
}
