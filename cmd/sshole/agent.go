package main

import (
	"context"
	"sshole/pkg/utils"

	"github.com/rs/zerolog/log"
)

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	utils.Execute(ctx, utils.ExecuteParams{
		Cmd: utils.Command("/opt/openssh/bin/ssh-keygen -A"),
	})

	utils.Execute(ctx, utils.ExecuteParams{
		Cmd: utils.Command("/opt/openssh/sbin/sshd -D -e"),
	})
}
