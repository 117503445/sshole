# sshole

基于 Go 开发的 SSH 连接代理系统，通过 WebSocket 实现内网穿透，允许从开发机 SSH 连接到内网容器。

## 🚀 特性

- **零配置内网穿透**: 无需公网 IP 或复杂网络配置
- **标准 SSH 协议**: 完全兼容标准 SSH 客户端
- **WebSocket 传输**: 基于 WebSocket 的安全数据传输
- **轻量级架构**: 控制连接复用 + 会话级独立隧道
- **容器友好**: 专为容器化环境设计
- **高性能**: 基于 `github.com/coder/websocket` 的零分配实现

## 📋 系统架构

本系统由三个核心组件组成：

### 🖥️ Agent（内网代理）
- 部署在内网容器中
- 自动启动内置 OpenSSH 服务
- 建立与 Hub 的长期 WebSocket 控制连接
- 按需建立数据隧道转发 SSH 流量

### 🌐 Hub（中转服务器）
- 部署在公网服务器
- 管理 Agent 注册和端口映射
- 提供 SSH 入口和 WebSocket 端点
- 处理数据转发和连接管理

### 💻 Entry（可选本地转发器）
- 运行在开发机上
- 提供本地端口转发体验
- 自动管理 SSH 公钥分发
- 简化用户连接流程

## 🔧 快速开始

### 前置要求

- Go 1.25.5+
- Docker (用于开发环境)

### 构建

```bash
# 构建所有组件
go-task build:all

# 构建单个组件
go-task build:hub
go-task build:agent
go-task build:entry
```

### 运行开发环境

```bash
# 启动开发容器
go-task base:dev

# 运行端到端测试
go-task base:e2e
```

### 部署示例

```bash
# 启动 Hub
docker run -d --name hub -p 9000:9000 -p 2222:2222 117503445/sshole-hub

# 启动 Agent (内网容器)
docker run -d --name agent 117503445/sshole-agent --hub-server http://hub:9000

# 连接到内网容器
ssh user@localhost -p 2222
```

## 📚 详细文档

- [构建与测试](docs/build-test.md) - 构建命令和测试指南
- [项目架构](docs/architecture.md) - 系统架构和组件设计
- [配置说明](docs/configuration.md) - 环境变量和命令行参数
- [开发指南](docs/development.md) - 开发注意事项和代码规范
- [概要设计说明书](docs/design.high-level.md) - 系统整体架构和设计理念
- [详细设计说明书](docs/design.detailed.md) - 实现级技术细节和接口规范

## 🔒 安全特性

- **Token 认证**: 所有组件间通信使用 Bearer Token
- **TLS 加密**: 生产环境建议配置 TLS
- **最小权限**: Agent SSHD 仅监听本地回环地址
- **会话隔离**: 每个 SSH 会话使用独立 WebSocket 隧道

## 🏗️ 技术栈

- **语言**: Go 1.25.5
- **WebSocket**: [github.com/coder/websocket](https://github.com/coder/websocket)
- **RPC**: [connectrpc.com/connect](https://connectrpc.com/)
- **日志**: [github.com/rs/zerolog](https://github.com/rs/zerolog)
- **配置**: [github.com/alecthomas/kong](https://github.com/alecthomas/kong)

## 📄 许可证

本项目采用 [GNU Affero General Public License v3.0](LICENSE) 许可证。

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

### 开发环境设置

```bash
# 克隆项目
git clone https://github.com/117503445/sshole.git
cd sshole

# 安装依赖
go mod download

# 运行测试
go test ./...

# 格式化代码
go-task format:all
```
