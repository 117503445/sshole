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
2. 建立与 Hub 的 WebSocket 控制连接，之后每 20 秒 ping 一次保活，链路断开后无限重连
3. 收到 Hub 的 OPEN 消息时，建立隧道连接本地 SSHD

```bash
./sshole-agent \
  --hub-server "http://hub:9000" \
  --auth "my-secret-token" \
  --name "agent-1"
```

| 参数 | 环境变量 | 类型 | 默认值 | 说明 |
|------|----------|------|--------|------|
| `--hub-server` | `SSHOLE_AGENT_HUB_SERVER` | string | `https://sshole.cloud.117503445.top` | Hub 服务器地址 |
| `--auth` | `SSHOLE_AGENT_AUTH` | string | 内置生产 Token | 认证 Token |
| `--name` | `SSHOLE_AGENT_NAME` | string | 空（自动生成） | Agent 名称；不指定时按规则自动生成，见下 |
| `--local-port` | `SSHOLE_AGENT_LOCAL_PORT` | int | `22222` | 本地 SSHD 监听端口 |
| `--skip-sshd` | `SSHOLE_AGENT_SKIP_SSHD` | bool | `false` | 跳过启动内置 SSHD |
| `--tunnel-dial` | `SSHOLE_AGENT_TUNNEL_DIAL_TIMEOUT` | duration | `5s` | 隧道拨号超时时间 |

默认值面向“员工双击 `sshole.exe` 就能被管理员连上”的调试场景：不带任何参数即可连上生产 Hub。
Windows 下内置 SSHD 自动使用 `powershell.exe` 作为 shell。

#### 不指定 `--name` 时的名称

未指定名称时生成 `<主机名>-<8位随机hex>`（例如 `win-tax01-3f9a1c2b`）：主机名让管理员认得出是哪台机器，
随机后缀保证克隆镜像里主机名重复的机器也不会互相顶掉同一个 Hub 端口。

生成的名称写入 `$HOME/.sshole/agent-name` 并复用，同一台机器重启后仍是同一个 Agent，不会每次启动都多出一个新条目。
启动日志里的 `no --name given, using generated agent name` 会打印最终名称，现场人员照它上报即可。

管理员侧查名字（`online:true` 表示当前在线）：

```bash
curl -s -X POST https://sshole.cloud.117503445.top/pkg.rpc.v1.HoleService/ListAgents \
  -H "Authorization: Bearer <生产 Token>" -H "Content-Type: application/json" -d '{}'
```

拿到名字后用它连 Entry：`sshole-entry --agent-name <名字>`。

### 启动横幅（版本与本节点信息）

`sshole-agent` 与 `sshole-entry` 启动时在控制台打印一段纯文本横幅，现场人员不必查后台、不必读 JSON 日志
就能确认「这是哪台机器、跑的是哪个构建」：

```text
================================================================
SSHole Agent
版本：v0.3.0
提交：00ca179abcde
构建时间：2026-11-16T13:58:02+08:00
本节点：WIN-TAX01
本机地址：192.168.100.77
系统：windows 10.0 build 22631 amd64
Agent 名称：win-tax01-3f9a1c2b
Hub：https://sshole.cloud.117503445.top
本地 SSH：127.0.0.1:22222（内置 sshd）
================================================================
```

- 版本、提交与构建时间由 CI 通过 ldflags 注入；本地 `go run` 未注入时显示 `unknown`。
- 提交只显示前 12 位，够定位构建，也不会把横幅拉得太宽。
- 本机地址取启用且非回环网卡上的全局单播地址，最多 8 个，链路本地地址不展示。
- Entry 的横幅额外打印目标 Agent 名称，管理员据此确认自己即将连的是哪台设备。

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
| `--auth-token` | `SSHOLE_HUB_AUTH` | string | - | 认证 Token（必填） |
| `--http-addr` | `SSHOLE_HUB_HTTP_ADDR` | string | `:9000` | HTTP 服务监听地址 |
| `--mapping-file` | `SSHOLE_HUB_MAPPING_FILE` | string | `data/port_mapping.json` | 端口映射持久化文件路径 |
| `--pending` | `SSHOLE_HUB_PENDING_TIMEOUT` | duration | `10s` | 等待隧道建立的超时时间 |
| `--tunnel-dial` | `SSHOLE_HUB_TUNNEL_DIAL_TIMEOUT` | duration | `5s` | 隧道拨号超时时间 |
| `--agent-ping` | `SSHOLE_HUB_AGENT_PING_INTERVAL` | duration | `20s` | Hub 主动探测 Agent 控制连接的周期 |
| `--agent-ping-timeout` | `SSHOLE_HUB_AGENT_PING_TIMEOUT` | duration | `10s` | 单次探测等待 pong 的上限，超时即判定离线 |

#### 在线判定与状态查询

机器直接断电时 TCP 不会发 FIN，Hub 只靠读循环会长期以为 Agent 还在，Entry 那边只能拿到
一个无法判断的 `handshake EOF`。现在：

1. Agent 每 20 秒在控制连接上发一帧 `STATUS`（同时承担保活），内容是构建版本、提交、
   构建时间、主机名、本机 SSH 端口与是否可连接、最近一次 SSH 失败原因。
2. Hub 每 `--agent-ping` 主动 ping 一次控制连接，pong 在 `--agent-ping-timeout` 内回不来
   就立刻把该 Agent 标记为离线，并关闭这条连接让 Agent 重连。
3. Agent 内嵌 sshd 由看护协程每 15 秒巡检，监听消失会自动重启，失败原因进入状态帧。
4. Agent 不在线时，`/tunnel` 直接返回 `503` 并带上原因，不再让 Entry 等满 pending 超时。

运维查询入口（Bearer 与 RPC 相同）：

```bash
curl -sS -H "Authorization: Bearer $SSHOLE_HUB_AUTH" https://hub.example.com/api/agents
# {"agents":[{"agent_name":"desktop-8nkqc4t-318ddebf","hub_port":10017,"online":false,
#   "last_seen_ms":1794825990690,"status_reported":true,"hostname":"DESKTOP-8NKQC4T",
#   "version":"37651c0","ssh_port":22222,"ssh_ready":true}]}
```

- `online=false` 且 `last_seen_ms` 停住：机器已关机或掉线。
- `online=true` 但 `ssh_ready=false`：机器在跑，是 Agent 本机 SSH 起不来，看 `ssh_error`。
- `status_reported=false`：现场还是旧构建，不会上报状态，先更新 Agent。

Entry 启动时会打印同一份状态，SSH 测试失败和隧道拨号失败也会把 `hub_view` 一起打出来，
不必再去猜"到底是没开机还是没监听"。

#### 二进制分发

Release 构建的 hub 已内嵌 `sshole_agent-linux-amd64` 与 `sshole_entry-linux-amd64`，
开箱即可通过 `/bins/` 下载（无需任何配置），适合没有 GitHub 访问的环境：

```bash
curl -fsSL https://hub.example.com/bins/sshole_agent-linux-amd64 -o agent.bin && chmod +x agent.bin
# macOS (Apple Silicon) 用 sshole_agent-darwin-arm64；entry 同理
```

`/bins/` 无需认证（二进制与 GitHub Release 公开内容一致），但不会暴露 Auth Token。

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
| `--hub-server` | `SSHOLE_ENTRY_HUB_SERVER` | string | `https://sshole.cloud.117503445.top` | Hub 服务器地址 |
| `--auth` | `SSHOLE_ENTRY_AUTH` | string | 内置生产 Token | 认证 Token |
| `--agent-name` | `SSHOLE_ENTRY_AGENT_NAME` | string | `temp-device` | 目标 Agent 名称 |
| `--entry-port` | `SSHOLE_ENTRY_PORT` | int | `22222` | 本地监听端口 |
| `--public-key` | `SSHOLE_ENTRY_PUBLIC_KEY` | string | `~/.ssh/id_ed25519.pub` | SSH 公钥路径 |
| `--private-key` | `SSHOLE_ENTRY_PRIVATE_KEY` | string | `~/.ssh/id_ed25519` | SSH 私钥路径 |

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
