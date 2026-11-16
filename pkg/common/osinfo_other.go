//go:build !windows

package common

import "runtime"

// osDescription returns the OS name and architecture on non-Windows hosts,
// where the agent is only used for development and CI.
func osDescription() string { return runtime.GOOS + " " + runtime.GOARCH }
