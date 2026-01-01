package main

import (
	"context"

	"sshole/pkg/common"
)

type commandHub struct {
	Auth string `env:"AUTH"`
}

func (c *commandHub) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "hub",
	})

	cmdHub(ctx)
	return nil
}

var cli struct {
	Hub commandHub `cmd:"" help:"run hub server"`
}
