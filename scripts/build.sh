#!/bin/bash

# sshole 项目构建脚本

set -e

echo "🏗️  Building sshole project..."

# 创建输出目录
mkdir -p bin

echo "📦 Building Agent..."
go build -o bin/agent ./cmd/agent

echo "📦 Building Hub..."
go build -o bin/hub ./cmd/hub

echo "📦 Building Entry..."
go build -o bin/entry ./cmd/entry

echo "✅ Build complete!"
echo ""
echo "📋 Generated binaries:"
ls -la bin/
echo ""
echo "🚀 Usage:"
echo "  ./bin/hub -addr ':8080'          # Start Hub server"
echo "  ./bin/agent -hub 'ws://localhost:8080'  # Start Agent"
echo "  ./bin/entry -hub 'ws://localhost:8080' -local ':10022'  # Start Entry"
