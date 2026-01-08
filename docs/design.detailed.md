# sshole 详细设计说明书

**设计级别**：详细设计（Detailed Design）

---

## 1. 引言

### 1.1 目的

本文档在概要设计基础上，细化到方法级别，明确组件结构、关键数据结构、并发模型、状态机与超时策略、协议帧格式与关键边界条件，为代码实现提供直接指导。

### 1.2 设计目标

* 方法职责明确、接口清晰
* 数据结构完整、类型安全
* 状态机显式、超时策略确定
* 并发安全、资源释放可靠
* 控制面极简、数据面透明

---

## 2. 系统架构总览

### 2.1 组件关系与包结构建议

```text
cmd/
  hub/        # Hub 主程序
  agent/      # Agent 主程序
  entry/      # Entry（可选）
pkg/
  common/     # 通用类型、错误、工具
  hub/        # Hub 核心实现
  agent/      # Agent 核心实现
  proto/      # 控制消息 JSON 结构定义（不是 proto 文件）
  tunnel/     # /tunnel 握手帧与转发逻辑
```

### 2.2 核心类型定义

```go
// pkg/common/types.go
type AgentName = string
type SessionID = string

type ProtocolVersion struct{} // 全局单一版本：仅作为常量存在
const SSHOLE_VERSION = 1
```

---

## 3. 协议与数据流（实现级）

## 3.1 WebSocket 端点

* `/agent`：控制通道（**Text JSON only**，不允许 Binary）
* `/tunnel`：数据通道（Binary），**第一帧必须是握手 header**，之后为纯 SSH 字节流

---

## 3.2 控制通道消息（Text JSON）

### 3.2.1 OPEN（Hub → Agent）

```go
// pkg/proto/control.go
type ControlMessage struct {
    Type      string `json:"type"`
    SessionID string `json:"session_id"`
}
```

示例：

```json
{"type":"OPEN","session_id":"abc123"}
```

**约束：**

* Hub → Agent 仅发送 `OPEN`
* Agent 不需要 ACK（保持最小；实现中可打印日志即可）

---

## 3.3 /tunnel 轻量握手帧（固定长度 Binary Header）

### 3.3.1 帧格式（固定长度）

```go
// pkg/tunnel/handshake.go
const (
    HandshakeMagicSize = 8
    HandshakeSIDSize   = 16
    HandshakeSize      = HandshakeMagicSize + HandshakeSIDSize
)

var HandshakeMagic = [8]byte{'S','S','H','O','L','E','0','1'}

type HandshakeHeader struct {
    Magic    [8]byte
    Session  [16]byte // sessionId 的 16 字节表示（实现可用 UUID bytes 或 hash 截断）
}
```

**语义：**

* `/tunnel` 建连后，Agent 必须先发送 `HandshakeHeader`（一次性 24 bytes）
* Hub 校验 Magic + session 对应 pending 状态
* 校验通过后，Hub 才开始转发 SSH 字节流

> 说明：你在概要里写了“sessionId raw/hash”，详细设计里固定为 16 bytes，具体生成策略在 6.3 说明。

---

## 4. Hub 组件详细设计

## 4.1 Hub 主结构体

```go
// pkg/hub/hub.go
type Hub struct {
    cfg *HubConfig

    // agentName -> agent runtime state（仅在线态信息）
    agents map[string]*AgentState

    // hubPort -> agentName（固定映射，从持久化加载，不改变）
    ports map[int]string

    // hubPort -> listener
    listeners map[int]net.Listener

    // sessionId -> pending
    pending map[string]*PendingSession

    mu sync.RWMutex

    ctx    context.Context
    cancel context.CancelFunc
}

type AgentState struct {
    Name AgentName

    // /agent 控制连接（在线态）
    Control *WSConn

    // 固定映射端口
    HubPort int

    ConnectedAt time.Time
}
```

---

## 4.2 Pending 会话结构与状态机

### 4.2.1 状态机

```text
INIT -> OPEN_SENT -> BOUND -> CLOSED
              \-> TIMEOUT
```

### 4.2.2 结构定义

```go
// pkg/hub/session.go
type PendingState int

const (
    PendingINIT PendingState = iota
    PendingOPEN_SENT
    PendingBOUND
    PendingTIMEOUT
    PendingCLOSED
)

type PendingSession struct {
    SessionID string
    AgentName AgentName

    SSHConn net.Conn // Hub 接入的 TCP(SSH client)

    State PendingState

    CreatedAt time.Time
    Deadline time.Time

    // tunnel ws 绑定后填充
    Tunnel *WSConn
}
```

---

## 4.3 Hub 生命周期

```go
// pkg/hub/hub.go
func NewHub(cfg *HubConfig) (*Hub, error)

func (h *Hub) Start(ctx context.Context) error
func (h *Hub) Stop() error
```

### 4.3.1 Start 具体步骤

1. `LoadPortMapping()`：加载 `agentName -> hubPort` 固定映射
2. 启动 HTTP server：

   * `GET /agent`：升级 WS，接收 Agent 控制连接
   * `GET /tunnel`：升级 WS，处理隧道连接与握手
   * RPC：`ListAgents`
3. 对每个已映射 port 启动 SSH listener（`listen :hubPort`）

   * 端口不可监听：**Hub 启动失败**（符合“端口分配不变”的约束）

---

## 4.4 /agent 控制连接处理（Hub 侧）

```go
// pkg/hub/agent_ws.go
func (h *Hub) handleAgentWS(w http.ResponseWriter, r *http.Request)
```

**行为：**

1. 校验 token（细节可在 `Auth` 模块）
2. 读取 headers：

   * `X-Agent`
3. 升级为 WebSocket
4. 注册到 `h.agents[agentName].Control`
5. 连接断开时：

   * 清理 `Control`
   * online=false

**控制通道限制：**

* 只允许 Text：

  * 收到 Binary frame：立即关闭连接（protocol violation）
* Hub 不需要从 Agent 接收任何业务消息（可忽略/丢弃，但仍需按 Text 处理）

---

## 4.5 SSH 入口监听（hubPort）与 OPEN 触发

### 4.5.1 PortListener

```go
// pkg/hub/port_listener.go
type PortListener struct {
    Hub *Hub
    HubPort int
    AgentName AgentName
    ln net.Listener
}

func (pl *PortListener) Serve(ctx context.Context) error
```

### 4.5.2 accept 处理逻辑

```go
// pkg/hub/ssh_accept.go
func (h *Hub) onSSHConn(agentName AgentName, sshConn net.Conn)
```

**步骤：**

1. 生成 `sessionId`
2. 创建 `PendingSession{State: INIT}`，写入 `h.pending[sessionId]`
3. 更新为 `OPEN_SENT`，并通过 agent 控制连接发送：

```json
{"type":"OPEN","session_id":"..."}
```

4. 等待 /tunnel 绑定（见 4.6），超时则：

   * `State = TIMEOUT`
   * 关闭 sshConn
   * 从 map 清理

---

## 4.6 /tunnel 连接处理（Hub 侧）

```go
// pkg/hub/tunnel_ws.go
func (h *Hub) handleTunnelWS(w http.ResponseWriter, r *http.Request)
```

### 4.6.1 处理步骤

1. 校验 token
2. 读取 headers：

   * `X-Agent`
   * `X-Session`
3. 升级 WS（Binary）
4. 读取**第一帧**，必须满足：

   * 固定长度 `HandshakeSize`
   * Magic 匹配
   * Session bytes 与 `X-Session` 对应（见 6.3 的转换函数）
5. Pending 匹配：

   * `pending[sessionId]` 必须存在
   * `pending.AgentName` 必须与 `X-Agent` 一致
   * `pending.State` 必须是 `OPEN_SENT`
   * `pending.Tunnel` 必须为空（否则为重复 dial-back）
6. 绑定成功：

   * `pending.Tunnel = ws`
   * `pending.State = BOUND`
   * 启动转发：`sshConn <-> ws`
7. 绑定失败（包括重复 dial-back）：

   * 立即关闭 ws，返回原因写日志

### 4.6.2 重复 dial-back

当 `pending.Tunnel != nil` 或 state 不是可绑定态：

* 视为重复 dial-back 或非法连接
* Hub **直接 close** 当前 `/tunnel` WS
* 不影响已绑定会话

---

## 4.7 数据转发实现（Hub 侧）

```go
// pkg/hub/forward.go
func (h *Hub) startForwarding(p *PendingSession) error
```

**模型：**

* 双向 copy：

  * SSH TCP → WS (binary frames)
  * WS → SSH TCP

**结束条件：**

* 任一方向出错或 EOF：

  * 关闭两端连接
  * `State = CLOSED`
  * 从 pending map 删除

---

## 4.8 ListAgents RPC（Hub）

```go
// pkg/hub/rpc.go
type AgentInfo struct {
    AgentName string `json:"agent_name"`
    HubPort   int    `json:"hub_port"`
    Online    bool   `json:"online"`
}

func (h *Hub) ListAgents(ctx context.Context) ([]AgentInfo, error)
```

Online 判断：`h.agents[name].Control != nil`

---

## 5. Agent 组件详细设计

## 5.1 Agent 主结构体

```go
// pkg/agent/agent.go
type Agent struct {
    cfg *AgentConfig

    // /agent 长连接（控制）
    control *ControlClient

    // dial /tunnel 的客户端
    tunnel *TunnelClient

    ctx    context.Context
    cancel context.CancelFunc
}
```

---

## 5.2 Agent 生命周期

```go
func NewAgent(cfg *AgentConfig) (*Agent, error)
func (a *Agent) Start(ctx context.Context) error
```

### 5.2.1 Start 行为

1. 建立 `/agent` WS 长连接
2. 进入 read loop，持续读取 Text JSON
3. 收到 `OPEN(sessionId)`：

   * 启动 goroutine：`handleOpen(sessionId)`
4. 控制连接断开：

   * 按重试策略重连
   * **重试发现无法建连 → 记录错误并退出进程**

---

## 5.3 ControlClient（Agent 侧）

```go
// pkg/agent/control_client.go
type ControlClient struct {
    cfg *AgentConfig
    ws  *websocket.Conn
}

func (cc *ControlClient) Connect(ctx context.Context) error
func (cc *ControlClient) ReadLoop(ctx context.Context, onOpen func(sessionID string)) error
```

**强约束：**

* 收到 Binary frame：视为协议错误（可直接断开并重连）
* 只解析 JSON Text

---

## 5.4 TunnelClient（Agent 侧）

```go
// pkg/agent/tunnel_client.go
type TunnelClient struct {
    cfg *AgentConfig
}

func (tc *TunnelClient) Dial(ctx context.Context, sessionID string) (*TunnelConn, error)
```

Dial 步骤：

1. `wss://hub/tunnel` 建连，带 headers：

   * `Authorization`
   * `X-Agent`
   * `X-Session`
2. 发送第一帧握手 header（固定 24 bytes）
3. 连接本地 `127.0.0.1:localPort`
4. 返回 TunnelConn 进入转发

---

## 5.5 TunnelConn（Agent 侧转发）

```go
// pkg/agent/tunnel_conn.go
type TunnelConn struct {
    ws   *websocket.Conn
    local net.Conn
}

func (tc *TunnelConn) Start(ctx context.Context) error
```

转发模型同 Hub：

* local TCP ↔ WS binary

结束条件：

* 任一方向错误：关闭两端

---

## 5.6 Agent 重连失败即退出（明确策略）

```go
// pkg/agent/retry.go
type RetryPolicy struct {
    MaxRetries int           // 建议 3
    Backoff    time.Duration // 建议 1s，失败递增到上限
}

func (a *Agent) runWithReconnect(ctx context.Context) error
```

规则：

* `/agent` 断线后重连
* 重连达到 MaxRetries 仍失败：

  * 打印 error（含 hub 地址、agentName）
  * **os.Exit(1)** 或返回致命错误由 main 退出

---

## 6. 数据结构与配置

## 6.1 Hub 配置

```go
// pkg/hub/config.go
type HubConfig struct {
    AuthToken string

    HTTPAddr string // 默认 ":8080"

    PendingTimeout time.Duration // 10s
    TunnelDialTimeout time.Duration // 5s

    MappingFile string // 端口映射持久化文件
}
```

---

## 6.2 Hub 端口映射（固定，不变）

**原则：**

* Hub 启动时从 `MappingFile` 加载 `agentName -> hubPort`
* 运行中不重新分配、不迁移、不回收
* Hub 启动时无法监听某个已分配端口：**启动失败**（保证“不变”）

```go
// pkg/hub/port_mapping.go
type PortMapping struct {
    Agents map[string]int `json:"agents"` // agentName -> hubPort
}

func LoadMapping(path string) (*PortMapping, error)
```

---

## 6.3 sessionId 与 16-byte session bytes 的转换

你要求握手 header 的 session 字段是固定 16 bytes，因此需要一个确定性转换：

```go
// pkg/tunnel/session_bytes.go
func SessionIDTo16Bytes(sessionID string) [16]byte {
    // 允许实现策略：
    // 1) 若 sessionID 是 UUID 字符串 -> 解析为 16 bytes
    // 2) 否则 hash(sessionID) 截断 16 bytes（如 sha256）
}
```

约束：

* Hub 与 Agent 必须使用同一转换函数（同一包复用）

---

## 6.4 超时参数（全局显式）

```go
// pkg/common/timeouts.go
type Timeouts struct {
    PendingTimeout    time.Duration // 10s
    TunnelDialTimeout time.Duration // 5s
    AgentReconnectMaxRetries int    // 3
    AgentReconnectBackoff    time.Duration // 1s（递增到上限可选）
}
```

---

## 7. 错误处理设计

## 7.1 错误码（最小集）

```go
// pkg/common/errors.go
type ErrCode string

const (
    ErrAuthFailed      ErrCode = "AUTH_FAILED"
    ErrAgentOffline    ErrCode = "AGENT_OFFLINE"
    ErrSessionNotFound ErrCode = "SESSION_NOT_FOUND"
    ErrSessionMismatch ErrCode = "SESSION_MISMATCH"
    ErrDuplicateTunnel ErrCode = "DUPLICATE_TUNNEL"
    ErrHandshakeFailed ErrCode = "HANDSHAKE_FAILED"
    ErrTimeout         ErrCode = "TIMEOUT"
)
```

Hub 对外（SSH）失败策略：

* 直接断开 SSH TCP（SSH 客户端得到连接断开即可）

---

## 8. 并发与资源管理

### 8.1 Hub 并发模型

* 每个 `hubPort` 一个 accept loop
* 每个 SSH 连接一个 goroutine
* 每个 `/tunnel` WS 一个 goroutine（绑定后进入转发）
* `pending`、`agents`、`ports` 访问均通过 `mu` 保护

### 8.2 资源释放顺序（强制）

会话结束时（任一侧断开）：

1. close SSH TCP
2. close WS tunnel
3. 从 pending map 删除
4. 记录关闭原因/时长（日志）

---

## 9. Entry（可选）详细设计

Entry 仅做 TCP↔TCP 转发，不参与 WS：

```go
// pkg/entry/entry.go
type Entry struct {
    cfg *EntryConfig
    hub HubClient
}

type EntryConfig struct {
    HubAddr string
    AuthToken string
    EntryPort int
    AgentName string
}
```

流程：

1. 调用 `ListAgents` 获取 hubPort
2. `listen :entryPort`
3. accept 本地 SSH conn
4. dial `hub:hubPort`
5. io.Copy 双向转发

---

## 10. 测试设计（与新模型一致）

### 10.1 单元测试

* `SessionIDTo16Bytes` 一致性测试
* `HandshakeHeader` 编解码测试
* Pending 状态机转换测试：

  * OPEN_SENT -> BOUND
  * OPEN_SENT -> TIMEOUT
  * BOUND 后重复 dial-back 拒绝

### 10.2 集成测试（最关键）

* 启动 Hub（含固定 mapping）
* 启动 Agent（连接 /agent）
* 用本地 `netcat` 或最小 TCP 客户端模拟 SSH 连接到 hubPort
* 验证：

  * Hub 发 OPEN
  * Agent dial-back /tunnel
  * 握手第一帧正确
  * 双向字节可转发
  * 关闭任一侧能正确清理 pending

---

## 11. 部署与运维

### 11.1 Hub 端口映射文件

* 必须与 Hub 一起持久化（磁盘/卷）
* Hub 重启后加载同一文件，保证 hubPort 不变

### 11.2 Agent 重连失败退出

* 由 systemd / supervisor 拉起
* 日志必须包含：

  * hub 地址
  * agentName
  * 错误原因（DNS/TLS/401/连接拒绝）

---

## 12. 总结

本详细设计以“控制连接复用 + 会话独立隧道”为中心，完成了实现级落地：

* `/agent` **Text JSON only**，最小控制消息
* `/tunnel` **第一帧固定长度 binary 握手 header**
* 明确 **重复 dial-back** 的拒绝策略
* Hub **端口映射固定不变**，启动即验证可监听
* Agent **重连失败即退出**
* Pending 状态机与超时参数 **显式化**

---
