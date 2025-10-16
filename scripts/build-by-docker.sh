#!/usr/bin/env sh

set -e

docker build -t sshole-builder . && docker run --rm -v $(pwd):/workspace sshole-builder