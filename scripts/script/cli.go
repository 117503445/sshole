package main

import "script/pkg/e2e"

var cli struct {
	Build  cmdBuild  `cmd:"" help:"Build"`
	Format cmdFormat `cmd:"" help:"Format and lint code"`
	E2E    cmdE2e    `cmd:"" aliases:"e2e" help:"e2e test"`
}

type cmdBuild struct {
}

func (c *cmdBuild) Run() error {
	build()
	return nil
}

type cmdFormat struct {
}

func (c *cmdFormat) Run() error {
	format()
	return nil
}

type cmdE2e struct {
}

func (c *cmdE2e) Run() error {
	e2e.E2e()
	return nil
}
