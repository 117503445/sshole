# 配置说明

本文档描述 sshole 项目各组件的配置方式。

## 配置方式

所有组件通过命令行参数进行配置，使用 [kong](https://github.com/alecthomas/kong) 库解析。

## Hub 配置

### 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--auth-token` | string | - | 认证 Token（必填） |
| `--http-addr` | string | `:9000` | HTTP 服务监听地址 |
| `--mapping-file` | string | - | 端口映射持久化文件路径 |
| `--pending-timeout` | duration | `10s` | 等待隧道建立的超时时间 |
| `--tunnel-dial-timeout` | duration | `5s` | 隧道拨号超时时间 |

### 示例

```bash
./sshole_hub \
  --auth-token "my-secret-token" \
  --http-addr ":9000" \
  --mapping-file "/data/hub/mapping.json"
```

### 端口映射文件

端口映射文件用于持久化 Agent 与 hubPort 的对应关系：

```json
{
  "agents": {
    "agent-1": 22001,
    "agent-2": 22002
  }
}
```

> **重要**: Hub 启动时加载映射文件，运行中不重新分配端口。若端口不可用，Hub 启动失败。

## Agent 配置

### 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--hub-url` | string | - | Hub 服务器地址（必填） |
| `--token` | string | - | 认证 Token（必填） |
| `--agent-name` | string | - | Agent 名称（必填） |
| `--local-port` | int | `22222` | 本地 SSHD 监听端口 |
| `--pending-timeout` | duration | `10s` | 等待隧道建立的超时时间 |
| `--tunnel-dial-timeout` | duration | `5s` | 隧道拨号超时时间 |
| `--reconnect-max-retries` | int | `3` | 重连最大重试次数 |
| `--reconnect-backoff` | duration | `1s` | 重连退避基础时间 |

### 示例

```bash
./sshole_agent \
  --hub-url "ws://hub:9000" \
  --token "my-secret-token" \
  --agent-name "agent-1" \
  --local-port 22222
```

### 重连策略

Agent 在控制连接断开时会自动重连：

- 初始退避时间：`1s`
- 最大退避时间：`30s`
- 最大重试次数：`3` 次
- 重试失败后 Agent 进程退出

### 内置 SSHD

Agent 启动时会自动启动内置 OpenSSH 服务：

- 监听地址：`127.0.0.1:<local-port>`
- HostKey：使用固定内置 ed25519 密钥
- AuthorizedKeys：`~/.sshole/authorized_keys`

## Entry 配置

### 命令行参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--hub-addr` | string | - | Hub 服务器地址（必填） |
| `--token` | string | - | 认证 Token（必填） |
| `--agent-name` | string | - | 目标 Agent 名称（必填） |
| `--entry-port` | int | - | 本地监听端口（必填） |

### 示例

```bash
./sshole_entry \
  --hub-addr "http://hub:9000" \
  --token "my-secret-token" \
  --agent-name "agent-1" \
  --entry-port 2222
```

### 自动公钥分发

Entry 启动时会：

1. 读取本机 SSH 公钥（`~/.ssh/id_ed25519.pub` 或 `~/.ssh/id_rsa.pub`）
2. 调用 Hub 的 `AppendKnownHost` RPC 将公钥添加到 Agent 的 `authorized_keys`
3. 将 Agent 的固定 HostKey 添加到本机 `known_hosts`（避免首次连接提示）

## 超时配置

### 默认超时值

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `PendingTimeout` | `10s` | 等待隧道建立的超时 |
| `TunnelDialTimeout` | `5s` | 隧道拨号超时 |
| `AgentReconnectMaxRetries` | `3` | 重连最大重试次数 |
| `AgentReconnectBackoff` | `1s` | 重连退避基础时间 |

### 超时配置建议

根据网络环境调整：

- **局域网**: 可适当减小超时值
- **公网/高延迟**: 建议增大 `PendingTimeout` 到 `15-20s`
- **不稳定网络**: 增大 `AgentReconnectMaxRetries` 到 `5-10`

## Docker 环境变量

在 Docker 容器中，配置通过环境变量传递：

### Hub

```yaml
environment:
  - AUTH_TOKEN=my-secret-token
```

### Agent

```yaml
environment:
  - HUB_SERVER=ws://hub:9000
  - AUTH=my-secret-token
```

## 安全建议

1. **Token 管理**: 使用强随机 Token，定期轮换
2. **TLS 加密**: 生产环境建议配置 TLS（由反向代理实现）
3. **端口规划**: Hub 的 SSH 端口范围应提前规划，避免冲突
4. **文件权限**: 端口映射文件应设置适当的访问权限