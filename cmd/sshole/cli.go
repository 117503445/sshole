package main

import (
	"context"
	"sshole/pkg/common"
)

type commandHub struct {
}

func (c *commandHub) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "hub",
	})
	
	cmdHub(ctx)
	return nil
}

type commandAgent struct {
}

func (c *commandAgent) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "agent",
	})
	
	cmdAgent(ctx)
	return nil
}

type commandEntry struct {
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
	Hub   commandHub   `cmd:"" help:"run hub server"`
	Agent commandAgent `cmd:"" help:"run agent client"`
	Entry commandEntry `cmd:"" help:"run entry client"`
}