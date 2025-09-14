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

type commandFc struct {
	Region    string `env:"FC_REGION"`
	AccountID string `env:"FC_ACCOUNT_ID"`

	AccessKeyId     string `env:"FC_ACCESS_KEY_ID"`
	AccessKeySecret string `env:"FC_ACCESS_KEY_SECRET"`
	// SecurityToken   string
}

func (c *commandFc) Run() error {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{
		Component: "fc",
	})

	cmdFc(ctx)
	return nil
}

var cli struct {
	Hub   commandHub   `cmd:"" help:"run hub server"`
	Agent commandAgent `cmd:"" help:"run agent client"`
	Entry commandEntry `cmd:"" help:"run entry client"`
	Fc    commandFc    `cmd:"" help:"run fc client"`
}
