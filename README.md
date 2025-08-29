# sshole

基于 Go 开发的 SSH 连接代理系统，通过 WebSocket 实现内网穿透，允许从开发机 SSH 连接到内网容器。

## 项目概述

本系统由三个核心组件组成：

- **Agent**: 运行在容器内，建立与 Hub 的 WebSocket 连接，转发容器内的 SSH 数据
- **Hub**: 运行在公网服务器，作为中转服务器，处理 Agent 和 Entry 之间的数据转发
- **Entry**: 运行在开发机，监听本地端口，转发 SSH 连接到 Hub

## 快速开始

### 1. 构建项目

```bash
# 构建所有组件（推荐）
go run scripts/build.go

# 或者使用详细输出
go run scripts/build.go -verbose

# 清理并重新构建
go run scripts/build.go -clean

# 交叉编译（例如编译为 Linux amd64）
go run scripts/build.go -os linux -arch amd64
```

这将在 `bin/` 目录下生成三个可执行文件：
- `bin/agent` - SSH Agent 组件
- `bin/hub` - SSH Hub 服务器组件
- `bin/entry` - SSH Entry 组件

### 2. 启动 Hub

```bash
./bin/hub -addr ":8080"
```

### 3. 启动 Agent（在容器内）

```bash
./bin/agent -hub "ws://your-hub-server:8080" -token "your-token"
```

### 4. 启动 Entry（在开发机）

```bash
./bin/entry -hub "ws://your-hub-server:8080" -local ":10022" -token "your-token"
```

### 5. 连接 SSH

```bash
ssh -p 10022 user@localhost
```

## 项目结构

```
.
├── cmd/                    # 主程序入口
│   ├── agent/             # Agent 主程序
│   ├── hub/               # Hub 主程序
│   └── entry/             # Entry 主程序
├── pkg/                   # 可重用代码包
│   ├── protocol/          # 协议定义
│   ├── websocket/         # WebSocket 相关
│   └── ssh/               # SSH 相关
├── internal/              # 内部包
│   ├── agent/             # Agent 内部逻辑
│   ├── hub/               # Hub 内部逻辑
│   └── entry/             # Entry 内部逻辑
├── api/                   # API 定义
├── configs/               # 配置文件
├── scripts/               # 构建和部署脚本
├── docs/                  # 文档
└── test/                  # 测试文件
```

## 技术选型

- **语言**: Go 1.21+
- **WebSocket**: coder/websocket
- **SSH**: golang.org/x/crypto/ssh
- **HTTP**: net/http
- **并发**: goroutine + channel
- **序列化**: JSON

## 开发状态

- [x] 项目结构设计
- [ ] Agent 模块开发
- [ ] Hub 模块开发
- [ ] Entry 模块开发
- [ ] WebSocket 协议设计
- [ ] 安全性考虑
- [ ] 测试和文档

## 许可证

MIT License