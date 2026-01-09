下面是**按你给定的约束，整理后的「完整概要设计说明书」**。
我**只做了你明确列出的事项**，其余地方保持你原有的设计取向与克制边界，没有引入额外复杂度。

---

# sshole 概要设计说明书

**设计级别**：概要设计（High-Level Design）

**核心原则**：

* 系统整体仅使用 **一个全局协议版本**
* **Agent⇄Hub 保持一条长期在线的控制连接**
* **每个 SSH 会话使用一条独立 WS 隧道（1 WS ↔ 1 SSH）**
* 不做 WS 多路复用、不做隧道复用
* 控制面极简，数据面透明

---

## 1. 引言

### 1.1 目的

本文档描述 **sshole** 系统的总体架构、核心运行模型、数据流、组件职责、协议约定、状态机与超时策略，为系统实现、评审、部署与运维提供统一、稳定的设计依据。

### 1.2 设计目标

* 提供一种 **无需 VPN、对 SSH 完全透明** 的内网访问方案
* 支持 **多个 SSH 客户端并发访问同一个内网 Agent**
* Hub 无需主动连接内网，全部连接由 Agent dial-back
* 实现复杂度低、行为可预测、故障可定位

---

## 2. 系统概述

### 2.1 系统定义

sshole 是一个基于 **内网 Agent 主动反向连接（dial-back）** 的 SSH 穿透系统，由以下组件组成：

* **Hub**：公网中转节点，对外暴露 SSH 入口端口
* **Agent**：部署在内网，主动连接 Hub，负责与本地 SSHD 建立连接
* **Entry（可选）**：运行在用户机器上的本地转发器，仅用于体验增强

用户使用标准 SSH 客户端即可访问内网主机。

---

### 2.2 核心设计要点

* **控制连接与数据隧道分离**

  * `/agent`：控制通道，仅用于最小协调
  * `/tunnel`：数据通道，仅承载 SSH 字节流
* **控制连接复用**

  * 每个 Agent 与 Hub 之间始终只有一条 `/agent` 长连接
* **会话级隧道**

  * 每个 SSH 会话独立建立一条 `/tunnel` WebSocket
* **严格职责边界**

  * `/agent` 不承载任何 SSH 字节
  * `/tunnel` 不承载任何控制语义

---

## 3. 系统架构

### 3.1 组件职责

#### Hub（公网）

* 接收 Agent 建立的 `/agent` 控制连接
* 为每个 Agent 分配并维护一个固定的 `hubPort`
* 在 `hubPort` 上监听 SSH TCP 连接
* 提供 RPC：`ListAgents`、`AppendKnownHost`（将用户公钥转发到 Agent known_hosts）
* 当收到 SSH 会话时：

  * 创建会话状态（Pending）
  * 通过 `/agent` 通知 Agent 建立隧道
  * 等待 `/tunnel` dial-back 并完成匹配
  * 建立字节流转发
* 提供最小查询接口（RPC）：`ListAgents`

---

#### Agent（内网）

* 启动即建立到 Hub 的 `/agent` 长连接
* 启动阶段自动拉起内置 OpenSSH（绑定 `localPort`，仅监听 127.0.0.1），使用固定 HostKey 和 `~/.sshole/authorized_keys`
* 监听 Hub 发来的 OPEN 请求
* 对每个 SSH 会话：

  * 主动 dial-back `/tunnel`
  * 连接本地 SSHD
  * 执行双向字节转发
* 控制连接断开时自动重连
* **Hub 重启后若无法重新建连，Agent 直接报错并退出**

---

#### Entry（可选）

* 查询 Hub 的 Agent 映射信息
* 读取本机公钥，调用 Hub `AppendKnownHost` 将其写入 Agent 的 known_hosts
* 在本地监听端口
* 启动时将 Agent 的固定 HostKey 追加到本机 `known_hosts`（目标 `[localhost]:entryPort`）
* 将本地 SSH TCP 连接转发到 Hub 的 `hubPort`
* 不参与 WebSocket 逻辑

---

### 3.2 连接与端口模型

| 名称           | 归属        | 说明                         |
| ------------ | --------- | -------------------------- |
| `localPort`  | Agent     | 本地 SSHD 端口，仅监听 `127.0.0.1` |
| `hubPort`    | Hub       | 对外暴露的 SSH 入口端口（固定映射）       |
| `/agent` WS  | Hub⇄Agent | 控制通道（长连接，JSON）             |
| `/tunnel` WS | Hub⇄Agent | 数据隧道（短连接，binary）           |
| `entryPort`  | Entry     | 本地监听端口（可选）                 |

---

## 4. 核心运行模型与数据流

### 4.1 主路径数据流

```
SSH Client
  → Hub(hubPort)
    → WS /tunnel
      → Agent(localPort)
        → SSHD
```

---

### 4.2 Agent 上线流程（控制连接）

1. Agent 启动，读取配置：

   * `agentName`
   * `localPort`
   * `token`
   * `hubURL`

2. Agent 建立 WebSocket 长连接：

   ```
   GET wss://hub/agent
   ```

   Headers：

   * `Authorization: Bearer <token>`
   * `X-Agent: <agentName>`

3. Hub 校验 token 与 agentName

4. Hub 加载已持久化的 `hubPort`。如果没有对应的 `hubPort`，则分配并持久化。

   * **不重新分配**
   * 若端口不可用 → Hub 启动失败

5. Hub 启动 `hubPort` SSH 监听

6. Agent 保持 `/agent` 长连接在线

**约束**

* 不引入心跳机制
* Agent 在线状态 = `/agent` 连接存在

---

### 4.3 SSH 会话触发隧道创建

当用户执行：

```bash
ssh user@hub -p <hubPort>
```

Hub 行为：

1. `accept()` SSH TCP 连接
2. 根据 `hubPort` 查找 `agentName`
3. 生成唯一 `sessionId`
4. 创建 Pending 会话状态
5. 通过 `/agent` 发送 OPEN 消息：

```json
{
  "type": "OPEN",
  "session_id": "abc123"
}
```

6. 等待 Agent dial-back `/tunnel`
7. 匹配成功后，建立转发
8. 任意一侧关闭即清理会话

---

### 4.4 Agent dial-back /tunnel（短连接）

1. Agent 收到 OPEN 请求
2. 启动独立 goroutine
3. 建立 WebSocket：

```
GET wss://hub/tunnel
```

Headers：

* `Authorization: Bearer <token>`
* `X-Agent: <agentName>`
* `X-Session: <sessionId>`

4. 同时连接本地：

```
127.0.0.1:<localPort>
```

5. 进入隧道握手阶段（见 4.5）

---

### 4.5 /tunnel 轻量握手帧（binary）

**目的**

* 确认 Hub 与 Agent 已就同一 `sessionId` 达成一致
* 防止错误匹配与竞态污染
* 为转发开始建立清晰边界

**规则**

* `/tunnel` 建立后
* **第一帧必须是固定长度 binary header**
* 握手完成后才允许转发 SSH 字节流

**示例结构（固定长度）**

```
[ 8 bytes ]  magic = "SSHOLE01"
[16 bytes ]  sessionId (raw / hash)
```

**握手流程**

1. Agent → Hub 发送握手帧
2. Hub 校验：

   * sessionId 是否存在
   * 是否尚未绑定
3. 校验成功：

   * Hub 标记会话为 BOUND
   * 开始转发
4. 校验失败：

   * 立即关闭 /tunnel

---

### 4.6 重复 dial-back 处理

* 同一个 `sessionId` **只允许绑定一次**
* 若收到重复 `/tunnel`：

  * Hub 立即关闭连接
  * 不影响已绑定会话
* Hub 记录重复事件用于日志与监控

---

## 5. 状态机与超时模型（显式定义）

### 5.1 Pending 会话状态机（Hub）

```
INIT
 ↓
OPEN_SENT
 ↓
BOUND ─────────→ CLOSED
 ↓
TIMEOUT
```

**状态说明**

| 状态        | 含义            |
| --------- | ------------- |
| INIT      | SSH 连接已建立     |
| OPEN_SENT | 已通知 Agent     |
| BOUND     | /tunnel 已绑定   |
| TIMEOUT   | 等待 /tunnel 超时 |
| CLOSED    | 会话结束          |

---

### 5.2 超时参数（建议默认值）

| 参数                            | 默认值      |
| ----------------------------- | -------- |
| `pending_timeout_ms`          | 10s      |
| `tunnel_dial_timeout_ms`      | 5s       |
| `agent_reconnect_backoff`     | 1s → 30s |
| `max_pending_per_agent`       | 100      |
| `max_active_tunnel_per_agent` | 200      |

---

## 6. 外部接口

### 6.1 用户接口

```bash
ssh user@hub -p <hubPort>
```

---

### 6.2 Hub RPC（最小）

```protobuf
rpc ListAgents(ListAgentsRequest) returns (ListAgentsResponse);
rpc AppendKnownHost(AppendKnownHostRequest) returns (AppendKnownHostResponse);

message AgentInfo {
  string agent_name = 1;
  int32 hub_port = 2;
  bool online = 3;
}

message AppendKnownHostRequest {
  string agent_name = 1;
  string public_key = 2;
}
```

---

## 7. 错误与失败模型

### 7.1 Agent 无法建连

* Hub 重启后端口不可用
* TLS / token 校验失败
  → Agent **直接报错并退出**

---

### 7.2 SSH 会话失败

* Agent 离线
* Pending 超时
* /tunnel 握手失败

→ Hub 断开 SSH 连接

---

## 8. 安全设计

* 所有连接使用 TLS。代码中不涉及 TLS，生产部署时由网关实现。
* Token 鉴权
* Agent SSHD 仅监听本地
* Hub 为唯一公网入口

---

## 9. 明确不包含

* WS 多路复用
* 隧道连接池
* Hub 多活 / HA
* 复杂控制面

---

## 10. 总结

sshole 采用 **“单控制连接 + 会话级独立隧道”** 的克制架构：

* 行为清晰
* 状态有限
* 竞态可控
* 实现难度与运维成本可预测
