package main

import (
	"context"

	"sshole/pkg/common"
)

type commandAgent struct {
	HubServer string `env:"HUB_SERVER" default:"localhost:9000"`
	Auth      string `env:"AUTH"`
	SshdPort  int32  `env:"SSHD_PORT" default:"22222"`
}

func (c *commandAgent) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "agent",
	})

	cmdAgent(ctx)
	return nil
}

var cli struct {
	Agent commandAgent `cmd:"" help:"run agent client"`
}
