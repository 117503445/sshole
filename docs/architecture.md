# 项目架构

本文档描述 sshole 项目的整体架构和组件设计。

## 系统概述

sshole 是一个基于 Go 开发的 SSH 连接代理系统，通过 WebSocket 实现内网穿透，允许从开发机 SSH 连接到内网容器。

### 核心设计原则

- **单控制连接 + 会话级独立隧道**: 每个 Agent 与 Hub 之间保持一条长期在线的控制连接，每个 SSH 会话使用独立的 WebSocket 隧道
- **控制面极简，数据面透明**: `/agent` 仅用于最小协调，`/tunnel` 仅承载 SSH 字节流
- **不做隧道复用**: 不引入 WebSocket 多路复用，行为清晰、状态有限

## 组件架构

```
┌─────────────┐                      ┌─────────────┐
│  SSH Client │                      │    Hub      │
│  (开发机)    │◄────── SSH ─────────►│  (公网服务器) │
└─────────────┘                      └──────┬──────┘
                                           │
                                    ┌──────┴──────┐
                                    │  WebSocket  │
                                    │  /agent     │
                                    │  /tunnel    │
                                    └──────┬──────┘
                                           │
                                     ┌─────┴─────┐
                                     │   Agent   │
                                     │  (内网)    │
                                     └─────┬─────┘
                                           │
                                     ┌─────┴─────┐
                                     │   SSHD    │
                                     │ (本地监听) │
                                     └───────────┘
```

## 三大核心组件

### Hub（公网中转服务器）

- 接收 Agent 建立的 `/agent` 控制连接
- 为每个 Agent 分配并维护固定的 `hubPort`
- 在 `hubPort` 上监听 SSH TCP 连接
- 提供 RPC 接口：`ListAgents`、`AppendKnownHost`
- 处理数据转发和连接管理

### Agent（内网代理）

- 部署在内网容器中
- 自动启动内置 OpenSSH 服务（仅监听 127.0.0.1）
- 建立与 Hub 的长期 WebSocket 控制连接
- 按需建立数据隧道转发 SSH 流量
- 控制连接断开时自动重连

### Entry（可选本地转发器）

- 运行在开发机上
- 提供本地端口转发体验
- 自动管理 SSH 公钥分发
- 简化用户连接流程

## 连接与端口模型

| 名称 | 归属 | 说明 |
|------|------|------|
| `localPort` | Agent | 本地 SSHD 端口，仅监听 127.0.0.1 |
| `hubPort` | Hub | 对外暴露的 SSH 入口端口（固定映射） |
| `/agent` WS | Hub⇄Agent | 控制通道（长连接，JSON Text） |
| `/tunnel` WS | Hub⇄Agent | 数据隧道（短连接，Binary） |
| `entryPort` | Entry | 本地监听端口（可选） |

## 数据流

### 主路径数据流

```
SSH Client → Hub(hubPort) → WS /tunnel → Agent(localPort) → SSHD
```

### Agent 上线流程

1. Agent 启动，读取配置
2. 建立 WebSocket 长连接到 `/agent`
3. Hub 校验 token 与 agentName
4. Hub 加载/分配固定的 `hubPort`
5. Hub 启动 `hubPort` SSH 监听

### SSH 会话流程

1. 用户执行 `ssh user@hub -p <hubPort>`
2. Hub accept SSH TCP 连接
3. Hub 通过 `/agent` 发送 OPEN 消息
4. Agent dial-back `/tunnel`
5. 隧道握手完成后建立双向转发

## 目录结构

```
sshole/
├── cmd/
│   ├── hub/          # Hub 主程序入口
│   ├── agent/        # Agent 主程序入口
│   └── entry/        # Entry 主程序入口
├── pkg/
│   ├── common/       # 通用类型、错误、超时配置
│   ├── hub/          # Hub 核心实现
│   ├── agent/        # Agent 核心实现
│   ├── entry/        # Entry 核心实现
│   ├── proto/        # 控制消息 JSON 结构定义
│   ├── tunnel/       # /tunnel 握手帧与转发逻辑
│   ├── rpc/          # RPC 定义（protobuf）
│   └── utils/        # 工具函数
├── internal/
│   └── buildinfo/    # 构建信息
├── scripts/
│   ├── tasks/        # Taskfile 任务定义
│   ├── script/       # 构建脚本
│   └── docker/       # Dockerfile
├── docs/             # 文档
└── data/             # 构建产物
```

## 核心包说明

### pkg/common

通用类型和配置：
- `types.go`: 基础类型定义（AgentName, SessionID）
- `errors.go`: 错误码定义
- `timeouts.go`: 超时配置

### pkg/hub

Hub 核心实现：
- `hub.go`: 主结构体和生命周期管理
- `config.go`: 配置定义
- `agent_ws.go`: `/agent` WebSocket 处理
- `tunnel_ws.go`: `/tunnel` WebSocket 处理
- `session.go`: 会话状态管理
- `mapping.go`: 端口映射管理
- `rpc.go`: RPC 服务实现

### pkg/agent

Agent 核心实现：
- `agent.go`: 主结构体和生命周期管理
- `config.go`: 配置定义
- `sshd.go`: 内置 SSHD 管理
- `known_hosts.go`: known_hosts 文件管理
- `hostkey.go`: 主机密钥管理

### pkg/tunnel

隧道相关：
- `handshake.go`: 握手帧处理
- `netconn.go`: 网络连接封装

### pkg/proto

控制消息定义：
- `control.go`: 控制消息 JSON 结构（OPEN, ADD_KNOWN_HOST）

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.25.5 |
| WebSocket | [github.com/coder/websocket](https://github.com/coder/websocket) |
| RPC | [connectrpc.com/connect](https://connectrpc.com/) |
| 日志 | [github.com/rs/zerolog](https://github.com/rs/zerolog) |
| 配置 | [github.com/alecthomas/kong](https://github.com/alecthomas/kong) |

## 更多信息

- [概要设计说明书](design.high-level.md) - 系统整体架构和设计理念
- [详细设计说明书](design.detailed.md) - 实现级技术细节和接口规范