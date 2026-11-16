//go:build windows

package common

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/windows"
)

// osDescription returns the real Windows version and build number.
// RtlGetVersion is not affected by manifest shims, so it can tell machines
// apart when a binary fails to start on one Windows release but not another.
func osDescription() string {
	info := windows.RtlGetVersion()
	return fmt.Sprintf("windows %d.%d build %d %s", info.MajorVersion, info.MinorVersion, info.BuildNumber, runtime.GOARCH)
}
