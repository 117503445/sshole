package main

import (
	"context"

	"sshole/pkg/common"
)

type commandEntry struct {
	HubServer string `env:"HUB_SERVER" default:"localhost:9000"`
}

func (c *commandEntry) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "entry",
	})

	cmdEntry(ctx)
	return nil
}

var cli struct {
	Entry commandEntry `cmd:"" help:"run entry client"`
}


