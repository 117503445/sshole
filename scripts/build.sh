#!/usr/bin/env sh

set -e

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 garble build -ldflags="-s -w" -o ./bin/sshole_linux_amd64 ./cmd/sshole
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 garble build -ldflags="-s -w" -o ./bin/sshole_linux_arm64 ./cmd/sshole
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 garble build -ldflags="-s -w" -o ./bin/sshole_darwin_amd64 ./cmd/sshole
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 garble build -ldflags="-s -w" -o ./bin/sshole_darwin_arm64 ./cmd/sshole
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 garble build -ldflags="-s -w" -o ./bin/sshole_windows_amd64.exe ./cmd/sshole
CGO_ENABLED=0 GOOS=windows GOARCH=arm64 garble build -ldflags="-s -w" -o ./bin/sshole_windows_arm64.exe ./cmd/sshole