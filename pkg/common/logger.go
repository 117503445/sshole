package common

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
)

// callerMarshalRegex matches paths in Go module cache, extracting module path with version
// e.g., ../root/go/pkg/mod/github.com/117503445/sshdev@v0.0.0-xxx/internal/server/server.go
// -> github.com/117503445/sshdev@v0.0.0-xxx/internal/server/server.go
var callerMarshalRegex = regexp.MustCompile(`(github\.com/[^\@]+@[^\}/]+)(.*)`)

// callerMarshalFunc formats file path for display in logs
// For Go module cache paths, extracts module path with version
func callerMarshalFunc(pc uintptr, file string, line int) string {
	// Try to match Go module cache path pattern (github.com/user/repo@version)
	if matches := callerMarshalRegex.FindStringSubmatch(file); len(matches) >= 3 {
		return matches[1] + matches[2] + ":" + strconv.Itoa(line)
	}

	// Fallback: try to extract from pkg/mod/ path
	if _, after, ok := strings.Cut(file, "pkg/mod/"); ok {
		return after + ":" + strconv.Itoa(line)
	}

	// Default: use basename only
	return filepath.Base(file) + ":" + strconv.Itoa(line)
}

// SetCallerMarshalFunc sets the global CallerMarshalFunc for zerolog.
// Must be called after glog.InitZeroLog() to override its default setting.
func SetCallerMarshalFunc() {
	zerolog.CallerMarshalFunc = callerMarshalFunc
}