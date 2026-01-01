package main

var cli struct {
	Build  cmdBuild  `cmd:"" help:"Build"`
	Format cmdFormat `cmd:"" help:"Format and lint code"`
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
