package main

import "script/pkg/e2e"

var cli struct {
	Build       cmdBuild       `cmd:"" help:"Build"`
	BuildDocker cmdBuildDocker `cmd:"" help:"Build Docker image"`
	Release     cmdRelease     `cmd:"" help:"Build release binaries for multiple platforms"`
	Format      cmdFormat      `cmd:"" help:"Format and lint code"`
	E2E         cmdE2e         `cmd:"" aliases:"e2e" help:"e2e test"`
}

type cmdBuild struct {
}

func (c *cmdBuild) Run() error {
	build()
	return nil
}

type cmdBuildDocker struct {
	Push bool `help:"Push image to registry after build" default:"false"`
}

func (c *cmdBuildDocker) Run() error {
	buildDocker()
	return nil
}

type cmdRelease struct {
}

func (c *cmdRelease) Run() error {
	release()
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
