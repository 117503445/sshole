# sshole

基于 Go 开发的 SSH 连接代理系统，通过 WebSocket 实现内网穿透，允许连接内网机器。

## 架构

分为三个组件

- Agent: 部署在内网机器上，启动 SSH Server，并建立与 Hub 的连接
- Hub: 部署在公网机器上，提供 Agent SSH 服务
- Entry: 可选，部署在开发机器上。连接到 Hub Websocket 后，在本地提供 Agent SSH 服务

## 使用方式

从 [Latest Release](https://github.com/117503445/sshole/releases/latest) 下载预编译二进制。

### Agent

部署在内网机器上。启动后：

1. 启动内置 SSH 服务器，监听 `127.0.0.1:local-port`
2. 建立与 Hub 的 WebSocket 控制连接
3. 收到 Hub 的 OPEN 消息时，建立隧道连接本地 SSHD

```bash
./sshole-agent \
  --hub-server "http://hub:9000" \
  --auth "my-secret-token" \
  --name "agent-1"
```

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|----------|------|--------|------|
| `--hub-server` | `HUB_SERVER` | string | - | Hub 服务器地址（必填） |
| `--auth` | `AUTH` | string | - | 认证 Token（必填） |
| `--name` | `NAME` | string | hostname | Agent 名称 |
| `--local-port` | `LOCAL_PORT` | int | `22222` | 本地 SSHD 监听端口 |
| `--skip-sshd` | `SKIP_SSHD` | bool | `false` | 跳过启动内置 SSHD |
| `--tunnel-dial` | `TUNNEL_DIAL_TIMEOUT` | duration | `5s` | 隧道拨号超时时间 |

### Hub

部署在公网机器上。启动后：

1. 加载端口映射文件，记录 agentName -> hubPort 的映射关系
2. 启动 HTTP 服务（WebSocket 端点、RPC 接口）
3. 为每个 Agent 启动 SSH 端口监听
4. 收到 SSH 连接时，通知 Agent 建立隧道并转发数据

```bash
./sshole-hub \
  --auth-token "my-secret-token" \
  --http-addr ":9000"
```

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|----------|------|--------|------|
| `--auth-token` | `AUTH` | string | - | 认证 Token（必填） |
| `--http-addr` | `HTTP_ADDR` | string | `:9000` | HTTP 服务监听地址 |
| `--mapping-file` | `MAPPING_FILE` | string | `data/port_mapping.json` | 端口映射持久化文件路径 |
| `--pending` | `PENDING_TIMEOUT` | duration | `10s` | 等待隧道建立的超时时间 |
| `--tunnel-dial` | `TUNNEL_DIAL_TIMEOUT` | duration | `5s` | 隧道拨号超时时间 |

### Entry

可选，部署在开发机器上。启动后：

1. 查询 Hub 获取目标 Agent 信息
2. 将本地 SSH 公钥推送到 Agent 的 `authorized_keys`
3. 在本地监听 SSH 端口
4. 收到连接时，通过 Hub 建立到 Agent 的隧道

```bash
./sshole-entry \
  --hub-server "http://hub:9000" \
  --auth "my-secret-token" \
  --agent-name "agent-1" \
  --entry-port 2222
```

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|----------|------|--------|------|
| `--hub-server` | `HUB_SERVER` | string | - | Hub 服务器地址（必填） |
| `--auth` | `AUTH` | string | - | 认证 Token（必填） |
| `--agent-name` | `AGENT_NAME` | string | - | 目标 Agent 名称（必填） |
| `--entry-port` | `ENTRY_PORT` | int | `22222` | 本地监听端口 |
| `--public-key` | `PUBLIC_KEY` | string | `~/.ssh/id_ed25519.pub` | SSH 公钥路径 |
| `--private-key` | `PRIVATE_KEY` | string | `~/.ssh/id_ed25519` | SSH 私钥路径 |

## 示例

### 示例一：Agent + Hub

最简部署，适合 Hub 有独立公网 IP、可以直接暴露 TCP SSH 端口 的场景。

```
┌─────────────┐         ┌─────────────┐
│   Agent     │◀──WS────│     Hub     │
│ (内网机器)   │         │ (公网机器)   │
│  SSHD       │         │  :2222      │
└─────────────┘         └─────────────┘
                              ▲
                              │ SSH
                              │
                        ┌─────────────┐
                        │   Client    │
                        └─────────────┘
```

**公网机器 - Hub**

```bash
./sshole-hub --auth-token "secret"
```

**内网机器 - Agent**

```bash
./sshole-agent \
  --hub-server "http://hub.example.com:9000" \
  --auth "secret" \
  --name "agent-1"
```

**SSH 连接**

查看 Hub 的端口映射文件或日志，找到 agent-1 的 hubPort

```bash
ssh root@hub.example.com -p <hubPort>
```

### 示例二：Agent + Hub + Entry

完整部署，适合 Hub 只能接收 HTTP/WS 连接，不能暴露 TCP SSH 端口的场景。

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Agent     │◀──WS────│     Hub     │◀──WS────│    Entry    │
│ (内网机器)   │         │ (中转服务器) │         │ (开发机器)   │
│  SSHD       │         │             │         │  :22222     │
└─────────────┘         └─────────────┘         └─────────────┘
                                                     ▲
                                                     │ SSH
                                                     │
                                               ┌─────────────┐
                                               │   Client    │
                                               └─────────────┘
```

**中转服务器 - Hub**

```bash
./sshole-hub --auth-token "secret"
```

**内网机器 - Agent**

```bash
./sshole-agent \
  --hub-server "http://hub.example.com:9000" \
  --auth "secret" \
  --name "agent-1"
```

**开发机器 - Entry**

```bash
./sshole-entry \
  --hub-server "http://hub.example.com:9000" \
  --auth "secret" \
  --agent-name "agent-1" \
  --entry-port 22222
```

**SSH 连接**

```bash
ssh root@localhost -p 22222
```
