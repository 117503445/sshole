package main

import "script/pkg/it"

var cli struct {
	Build  cmdBuild  `cmd:"" help:"Build"`
	Format cmdFormat `cmd:"" help:"Format and lint code"`
	It     cmdIt     `cmd:"" help:"Integration test"`
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


type cmdIt struct {
}

func (c *cmdIt) Run() error {
	it.It()
	return nil
}