# sshole 详细设计说明书

**设计级别**：详细设计（Detailed Design）

---

## 1. 引言

### 1.1 目的

本文档在概要设计基础上，进一步细化到方法级别，明确每个组件的类结构、方法签名、职责分工及数据流向，为代码实现提供详细的技术指导。

### 1.2 设计目标

* 方法职责明确、接口清晰
* 数据结构完整、类型安全
* 错误处理完善、状态管理清晰
* 并发安全、资源管理合理

---

## 2. 系统架构总览

### 2.1 组件关系

```go
// pkg/common/types.go - 核心类型定义
// Hub: 公网中转节点
type Hub struct {
    tunnelServer  *TunnelServer     // 数据面服务
    portAllocator *PortAllocator    // 端口分配器
}

// Agent: 内网代理节点
type Agent struct {
    config       *AgentConfig       // 配置信息
    client       *ControlClient     // 控制面客户端
    tunnelClient *TunnelClient      // 隧道客户端
    heartbeat    *HeartbeatService  // 心跳服务
}

// Entry: 本地入口增强（可选）
type Entry struct {
    config       *EntryConfig       // 配置信息
    client       *ControlClient     // 控制面客户端
    tunnelClient *TunnelClient      // 隧道客户端
    localServer  *LocalServer       // 本地SSH服务器
}
```

---

## 3. Hub 组件详细设计

### 3.1 Hub 主结构体

```go
// cmd/hub/hub.go - Hub 主结构体定义
type Hub struct {
    // 配置
    config *HubConfig

    // 核心服务
    tunnelServer  *TunnelServer

    // 状态管理
    portAllocator *PortAllocator

    // 并发控制
    mu sync.RWMutex

    // 生命周期
    ctx    context.Context
    cancel context.CancelFunc
}
```

### 3.2 Hub 核心方法

#### 3.2.1 生命周期方法

```go
// cmd/hub/hub.go - Hub 生命周期方法
// NewHub 创建并初始化Hub实例
func NewHub(config *HubConfig) (*Hub, error) {
    // 初始化配置验证
    // 创建Agent注册表
    // 初始化端口分配器
    // 创建控制面服务
    // 创建数据面服务
}

// Start 启动Hub服务
func (h *Hub) Start(ctx context.Context) error {
    // 启动控制面服务
    // 启动数据面服务
    // 启动端口监听
    // 启动健康检查
}
```

#### 3.2.2 Agent管理方法

```go
// cmd/hub/handler.go - Agent 管理方法
// RegisterAgent 处理Agent注册请求
func (h *Hub) RegisterAgent(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
    // 验证鉴权信息
    // 检查Agent是否已存在
    // 分配或恢复hubPort
    // 记录注册信息
    // 启动端口监听
}

// UnregisterAgent 处理Agent注销请求
func (h *Hub) UnregisterAgent(ctx context.Context, req *UnregisterRequest) error {
    // 验证权限
    // 停止对应端口监听
    // 清理注册信息
    // 关闭相关隧道
}

// Heartbeat 处理Agent心跳
func (h *Hub) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
    // 更新Agent最后心跳时间
    // 检查Agent状态
    // 返回当前状态信息
}

// ListAgents 获取在线Agent列表
func (h *Hub) ListAgents(ctx context.Context, req *ListAgentsRequest) (*ListAgentsResponse, error) {
    // 获取所有在线Agent
    // 过滤和排序
    // 返回Agent信息列表
}
```

### 3.3 数据面服务 (TunnelServer)

#### 3.3.1 TunnelServer 结构体

```go
// pkg/tunnel/server.go - 数据面服务
type TunnelServer struct {
    hub          *Hub
    portListeners map[int]*PortListener  // hubPort -> listener
    tunnels       map[string]*Tunnel     // tunnelID -> tunnel
    mu            sync.RWMutex
}
```

#### 3.3.2 核心方法

```go
// Serve 启动WebSocket服务
func (ts *TunnelServer) Serve() http.Handler {
    return http.HandlerFunc(ts.handleWebSocket)
}

// handleWebSocket 处理WebSocket连接
func (ts *TunnelServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    // 升级HTTP连接到WebSocket
    // 验证握手参数
    // 创建隧道实例
    // 启动数据转发
}

// createTunnel 为SSH连接创建数据隧道
func (ts *TunnelServer) createTunnel(
    ctx context.Context,
    agentName string,
    localPort int,
    wsConn *websocket.Conn,
) (*Tunnel, error) {
    // 查找目标Agent
    // 建立到Agent的隧道连接
    // 返回隧道实例
}
```

### 3.4 端口监听器 (PortListener)

```go
// cmd/hub/port.go - 端口监听器（待创建）
type PortListener struct {
    hubPort    int
    agentName  string
    listener   net.Listener
    hub        *Hub
    activeConns map[string]net.Conn  // 活跃连接
    mu         sync.RWMutex
}

// Listen 启动SSH端口监听
func (pl *PortListener) Listen(ctx context.Context) error {
    // 绑定端口
    // 启动accept循环
}

// acceptLoop 接受SSH连接
func (pl *PortListener) acceptLoop(ctx context.Context) {
    for {
        // 接受TCP连接
        // 为每个连接创建隧道
        // 处理连接生命周期
    }
}
```

### 3.5 隧道 (Tunnel)

```go
// pkg/tunnel/tunnel.go - 隧道实现
type Tunnel struct {
    ID         string
    AgentName  string
    LocalPort  int
    SSHConn    net.Conn        // SSH客户端连接
    TunnelConn *websocket.Conn // 到Agent的WebSocket连接
    ctx        context.Context
    cancel     context.CancelFunc
}

// Start 启动隧道数据转发
func (t *Tunnel) Start() error {
    // 启动双向数据转发
    // 处理连接关闭
}

// forward 从SSH到WebSocket
func (t *Tunnel) forwardSSHToWS() {
    // 读取SSH TCP数据
    // 直接通过WebSocket发送原始字节
}

// forward 从WebSocket到SSH
func (t *Tunnel) forwardWSToSSH() {
    // 直接从WebSocket读取原始字节
    // 发送到SSH TCP连接
}
```

---

## 4. Agent 组件详细设计

### 4.1 Agent 主结构体

```go
// cmd/agent/agent.go - Agent 主结构体
type Agent struct {
    config       *AgentConfig
    client       *ControlClient
    tunnelClient *TunnelClient
    heartbeat    *HeartbeatService

    // 状态
    hubPort      int
    registered   bool
    lastHeartbeat time.Time

    // 并发控制
    mu sync.RWMutex

    // 生命周期
    ctx    context.Context
    cancel context.CancelFunc
}
```

### 4.2 Agent 核心方法

#### 4.2.1 生命周期方法

```go
// cmd/agent/agent.go - Agent 生命周期方法
// NewAgent 创建Agent实例
func NewAgent(config *AgentConfig) (*Agent, error) {
    // 验证配置
    // 创建客户端
    // 初始化心跳服务
}

// Start 启动Agent
func (a *Agent) Start(ctx context.Context) error {
    // 连接到Hub
    // 注册Agent
    // 启动心跳
    // 启动隧道监听
}
```

#### 4.2.2 注册与心跳

```go
// cmd/agent/agent.go - Agent 注册与心跳方法
// Register 向Hub注册
func (a *Agent) Register(ctx context.Context) error {
    // 构建注册请求
    // 调用Hub注册接口
    // 保存分配的hubPort
    // 标记为已注册
}

// sendHeartbeat 发送心跳
func (a *Agent) sendHeartbeat(ctx context.Context) error {
    // 检查注册状态
    // 发送心跳请求
    // 更新最后心跳时间
}
```

### 4.3 隧道客户端 (TunnelClient)

```go
// pkg/tunnel/client.go - 隧道客户端
type TunnelClient struct {
    config *AgentConfig
    hub    *websocket.Dialer
}

// Connect 连接到Hub的隧道服务
func (tc *TunnelClient) Connect(
    ctx context.Context,
    tunnelID string,
    localPort int,
) (*TunnelConnection, error) {
    // 建立WebSocket连接
    // 发送握手信息
    // 返回连接实例
}
```

### 4.4 隧道连接 (TunnelConnection)

```go
// pkg/tunnel/client.go - 隧道连接实现
type TunnelConnection struct {
    wsConn    *websocket.Conn
    localConn net.Conn
    ctx       context.Context
    cancel    context.CancelFunc
}

// Start 启动数据转发
func (tc *TunnelConnection) Start() error {
    // 连接到本地SSHD
    // 启动双向转发
}

// forward 从WebSocket到本地
func (tc *TunnelConnection) forwardWSToLocal() {
    // 读取WebSocket数据
    // 转发到本地SSHD
}

// forward 从本地到WebSocket
func (tc *TunnelConnection) forwardLocalToWS() {
    // 读取本地SSHD数据
    // 转发到WebSocket
}
```

---

## 5. Entry 组件详细设计

### 5.1 Entry 主结构体

```go
// cmd/entry/entry.go - Entry 主结构体
type Entry struct {
    config       *EntryConfig
    client       *ControlClient
    tunnelClient *TunnelClient
    localServer  *LocalServer

    // 状态
    connectedAgent string
    localPort      int

    // 并发控制
    mu sync.RWMutex
}
```

### 5.2 Entry 核心方法

#### 5.2.1 生命周期方法

```go
// cmd/entry/entry.go - Entry 生命周期方法
// NewEntry 创建Entry实例
func NewEntry(config *EntryConfig) (*Entry, error) {
    // 验证配置
    // 创建客户端
    // 初始化本地服务器
}

// Start 启动Entry服务
func (e *Entry) Start(ctx context.Context) error {
    // 启动本地SSH服务器
    // 启动管理接口
}
```

#### 5.2.2 连接管理

```go
// cmd/entry/entry.go - Entry 连接管理方法
// Connect 连接到指定Agent
func (e *Entry) Connect(ctx context.Context, agentName string) error {
    // 检查Agent状态
    // 创建隧道连接
    // 启动本地监听
    // 更新连接状态
}

// Disconnect 断开连接
func (e *Entry) Disconnect() error {
    // 关闭本地监听
    // 断开隧道连接
    // 清理状态
}
```

### 5.3 本地服务器 (LocalServer)

```go
// cmd/entry/server.go - 本地服务器（待创建）
type LocalServer struct {
    entry      *Entry
    listener   net.Listener
    activeConns map[string]net.Conn
    mu         sync.RWMutex
}

// Listen 启动本地SSH监听
func (ls *LocalServer) Listen(ctx context.Context, port int) error {
    // 绑定本地端口
    // 启动accept循环
}

// acceptLoop 处理本地SSH连接
func (ls *LocalServer) acceptLoop(ctx context.Context) {
    for {
        // 接受本地连接
        // 创建到Hub的隧道
        // 启动数据转发
    }
}
```

---

## 6. 数据结构设计

### 6.1 配置结构

#### 6.1.1 Hub配置

```go
// cmd/hub/main.go - Hub CLI 配置（使用 kong 库）
var cli struct {
    Auth       string `env:"AUTH"`
    PortRange  string `env:"PORT_RANGE" default:"49152-65535"`
}

// HubConfig 运行时配置（从 CLI 参数转换）
type HubConfig struct {
    // 鉴权配置
    AuthToken string

    // 网络配置（硬编码或从环境变量读取）
    ControlAddr string // 控制面监听地址，默认 ":8080"
    TunnelAddr  string // 数据面监听地址，默认 ":8081"

    // 端口分配
    PortRange string // 端口范围表达式，如 "2000-3000,3004,4000-4001"

    // 超时配置
    AgentTimeout time.Duration

    // 日志配置
    LogLevel string
}

// newHubConfig 从 CLI 参数创建配置
func newHubConfig() *HubConfig {
    return &HubConfig{
        AuthToken:    cli.Auth,
        ControlAddr:  ":8080", // 可以从环境变量读取
        TunnelAddr:   ":8081", // 可以从环境变量读取
        PortRange:    cli.PortRange, // 默认 "49152-65535"
        AgentTimeout: 5 * time.Minute,
        LogLevel:     "info",
    }
}

// 需要导入的包
import (
    "os"
    "time"
)
```

#### 6.1.2 Agent配置

```go
// cmd/agent/main.go - Agent CLI 配置（使用 kong 库）
var cli struct {
    HubServer string `env:"HUB_SERVER"`
    Auth      string `env:"AUTH"`
    Name      string `env:"NAME"`
    SshdPort  int    `env:"SSHD_PORT" default:"22222"`
}

// AgentConfig 运行时配置（从 CLI 参数转换）
type AgentConfig struct {
    // Hub连接
    HubAddr   string
    AuthToken string

    // Agent信息
    Name      string
    LocalPort int

    // 心跳配置
    HeartbeatInterval time.Duration
    MaxRetries        int
}

// newAgentConfig 从 CLI 参数创建配置
func newAgentConfig() *AgentConfig {
    name := cli.Name
    if name == "" {
        // 使用主机名作为默认名称
        hostname, _ := os.Hostname()
        name = hostname
    }

    return &AgentConfig{
        HubAddr:           cli.HubServer,
        AuthToken:         cli.Auth,
        Name:              name,
        LocalPort:         cli.SshdPort,
        HeartbeatInterval: 30 * time.Second,
        MaxRetries:        3,
    }
}

// 需要导入的包
import (
    "os"
    "time"
)
```

#### 6.1.3 Entry配置

```go
// cmd/entry/main.go - Entry CLI 配置（使用 kong 库）
var cli struct {
    HubServer  string `env:"HUB_SERVER"`
    Auth       string `env:"AUTH"`
    AgentName  string `env:"AGENT_NAME"`
    SshPort    int    `env:"SSH_PORT" default:"22222"`
    PrivateKey string `env:"PRIVATE_KEY" description:"The path to the private key pam file"`
    PublicKey  string `env:"PUBLIC_KEY" description:"The path to the public key pam file"`
}

// EntryConfig 运行时配置（从 CLI 参数转换）
type EntryConfig struct {
    // Hub连接
    HubAddr   string
    AuthToken string

    // 本地配置
    LocalPort    int
    DefaultAgent string

    // 密钥配置
    PrivateKeyPath string
    PublicKeyPath  string
}

// newEntryConfig 从 CLI 参数创建配置
func newEntryConfig() *EntryConfig {
    return &EntryConfig{
        HubAddr:        cli.HubServer,
        AuthToken:      cli.Auth,
        LocalPort:      cli.SshPort,
        DefaultAgent:   cli.AgentName,
        PrivateKeyPath: cli.PrivateKey,
        PublicKeyPath:  cli.PublicKey,
    }
}

// 不需要额外的导入
```

### 6.2 协议消息

#### 6.2.1 控制面消息

```go
// pkg/rpc/v1/sshole.pb.go - 控制面消息（Protocol Buffers 生成）
type RegisterRequest struct {
    AgentName string `json:"agent_name"`
    AuthToken string `json:"auth_token"`
}

type RegisterResponse struct {
    HubPort   int    `json:"hub_port"`
    ExpiresAt int64  `json:"expires_at"`
}

// 心跳消息
type HeartbeatRequest struct {
    AgentName string `json:"agent_name"`
}

type HeartbeatResponse struct {
    Status    string `json:"status"`
    Timestamp int64  `json:"timestamp"`
}

// Agent列表
type ListAgentsRequest struct {
    Filter string `json:"filter,omitempty"`
}

type ListAgentsResponse struct {
    Agents []AgentInfo `json:"agents"`
}

type AgentInfo struct {
    Name       string `json:"name"`
    HubPort    int    `json:"hub_port"`
    Status     string `json:"status"`
    LastSeen   int64  `json:"last_seen"`
    ConnectedAt int64 `json:"connected_at"`
}
```

#### 6.2.2 WebSocket隧道协议

WebSocket连接建立后直接传输原始TCP数据，无控制协议：

```go
// WebSocket握手通过URL参数传递元信息
// 例如: ws://hub/tunnel?agent=agent1&port=22

// 之后WebSocket直接传输原始TCP字节流
// 无消息类型、无控制帧、无封装
```

---

## 7. 错误处理设计

### 7.1 错误类型

```go
// pkg/common/errors.go - 错误处理类型
// 控制面错误
type ControlError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
    Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ControlError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// 预定义错误码
const (
    ErrCodeAuthFailed     = "AUTH_FAILED"
    ErrCodeAgentNotFound  = "AGENT_NOT_FOUND"
    ErrCodeAgentOffline   = "AGENT_OFFLINE"
    ErrCodeResourceExhausted = "RESOURCE_EXHAUSTED"
    ErrCodeInternal       = "INTERNAL_ERROR"
)
```

### 7.2 错误处理方法

```go
// pkg/common/errors.go - 错误处理方法
// handleControlError 处理控制面错误
func handleControlError(err error) *ControlError {
    // 转换错误类型
    // 添加上下文信息
    // 记录日志
}

// validateAuth 验证鉴权
func validateAuth(token string) error {
    // 检查token格式
    // 验证token有效性
    // 返回错误或nil
}
```

---

## 8. 资源管理设计

### 8.1 连接池管理

```go
// pkg/common/pool.go - 连接池管理（待创建）
type ConnectionPool struct {
    pool   chan net.Conn
    mu     sync.Mutex
    closed bool
}

// Get 获取连接
func (cp *ConnectionPool) Get(ctx context.Context) (net.Conn, error) {
    select {
    case conn := <-cp.pool:
        return conn, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        return cp.createConnection()
    }
}

// Put 归还连接
func (cp *ConnectionPool) Put(conn net.Conn) {
    cp.mu.Lock()
    defer cp.mu.Unlock()
    if !cp.closed {
        select {
        case cp.pool <- conn:
        default:
            conn.Close()
        }
    } else {
        conn.Close()
    }
}
```

### 8.2 端口分配器

```go
type PortAllocator struct {
    filePath  string           // JSON文件路径
    allocated map[int]string   // port -> agentName
    available []int
    mu        sync.Mutex
}

// NewPortAllocator 创建端口分配器
func NewPortAllocator(filePath string, portRangeExpr string) (*PortAllocator, error) {
    pa := &PortAllocator{
        filePath:  filePath,
        allocated: make(map[int]string),
        available: make([]int, 0),
    }

    // 解析端口范围表达式
    availablePorts, err := parsePortRange(portRangeExpr)
    if err != nil {
        return nil, err
    }
    pa.available = availablePorts

    // 从文件加载已分配端口
    if err := pa.loadFromFile(); err != nil {
        return nil, err
    }

    return pa, nil
}

// parsePortRange 解析端口范围表达式，如 "2000-3000,3004,4000-4001"
func parsePortRange(expr string) ([]int, error) {
    var ports []int
    ranges := strings.Split(expr, ",")

    for _, r := range ranges {
        r = strings.TrimSpace(r)
        if strings.Contains(r, "-") {
            // 处理范围，如 "2000-3000"
            parts := strings.Split(r, "-")
            if len(parts) != 2 {
                return nil, fmt.Errorf("invalid range format: %s", r)
            }
            start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
            if err != nil {
                return nil, err
            }
            end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
            if err != nil {
                return nil, err
            }
            for port := start; port <= end; port++ {
                ports = append(ports, port)
            }
        } else {
            // 处理单个端口，如 "3004"
            port, err := strconv.Atoi(strings.TrimSpace(r))
            if err != nil {
                return nil, err
            }
            ports = append(ports, port)
        }
    }

    return ports, nil
}

// 需要导入的包
import (
    "fmt"
    "strconv"
    "strings"
)

// Allocate 分配端口
func (pa *PortAllocator) Allocate(agentName string) (int, error) {
    pa.mu.Lock()
    defer pa.mu.Unlock()

    // 检查是否已有分配
    for port, name := range pa.allocated {
        if name == agentName {
            return port, nil
        }
    }

    // 分配新端口
    if len(pa.available) == 0 {
        return 0, errors.New("no available ports")
    }

    port := pa.available[0]
    pa.available = pa.available[1:]
    pa.allocated[port] = agentName

    // 持久化到文件
    if err := pa.saveToFile(); err != nil {
        return 0, err
    }

    return port, nil
}

// Release 释放端口
func (pa *PortAllocator) Release(agentName string) error {
    pa.mu.Lock()
    defer pa.mu.Unlock()

    // 查找并释放端口
    for port, name := range pa.allocated {
        if name == agentName {
            delete(pa.allocated, port)
            pa.available = append(pa.available, port)

            // 持久化到文件
            return pa.saveToFile()
        }
    }

    return errors.New("agent not found")
}

// loadFromFile 从JSON文件加载分配状态
func (pa *PortAllocator) loadFromFile() error {
    data, err := os.ReadFile(pa.filePath)
    if os.IsNotExist(err) {
        return nil // 文件不存在，使用默认状态
    }
    if err != nil {
        return err
    }

    var state PortAllocationState
    if err := json.Unmarshal(data, &state); err != nil {
        return err
    }

    // 恢复分配状态
    pa.allocated = state.Allocated

    // 重建可用端口列表
    available := make([]int, 0)
    for port := pa.available[0]; port <= pa.available[len(pa.available)-1]; port++ {
        if _, allocated := pa.allocated[port]; !allocated {
            available = append(available, port)
        }
    }
    pa.available = available

    return nil
}

// saveToFile 保存分配状态到JSON文件
func (pa *PortAllocator) saveToFile() error {
    state := PortAllocationState{
        Allocated: pa.allocated,
    }

    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(pa.filePath, data, 0644)
}

type PortAllocationState struct {
    Allocated map[int]string `json:"allocated"`
}
```

---

## 9. 测试设计

### 9.1 单元测试结构

```go
// Hub_test.go
func TestHub_RegisterAgent(t *testing.T) {
    // 创建测试Hub
    // 模拟注册请求
    // 验证响应
    // 检查状态
}

func TestHub_PortAllocation(t *testing.T) {
    // 测试端口分配逻辑
    // 测试端口回收
    // 测试并发分配
}

// Agent_test.go
func TestAgent_Registration(t *testing.T) {
    // 模拟Hub服务
    // 测试注册流程
    // 验证心跳机制
}
```

### 9.2 集成测试

```go
// hub_agent_integration_test.go
func TestHubAgentIntegration(t *testing.T) {
    // 启动测试Hub
    // 启动测试Agent
    // 执行SSH连接测试
    // 验证数据传输
}
```

---

## 10. 部署与运维设计

### 10.1 配置管理

配置通过 CLI 参数和环境变量管理，使用 kong 库进行解析：

```go
// 各组件使用 kong 库解析 CLI 参数
// cmd/hub/main.go, cmd/agent/main.go, cmd/entry/main.go
import "github.com/alecthomas/kong"

var cli struct {
    // 组件特定的 CLI 参数定义
    // 使用 env 标签绑定环境变量
    // 使用 default 标签设置默认值
}

func main() {
    kong.Parse(&cli)
    // 使用 cli 变量中的配置
}
```

配置转换函数将 CLI 参数转换为内部配置结构：

```go
// pkg/common/config.go - 配置转换工具
// newHubConfig, newAgentConfig, newEntryConfig 函数
// 将 CLI 参数转换为对应的 Config 结构体
```

### 10.2 健康检查

```go
// cmd/hub/health.go - 健康检查服务（待创建）
type HealthChecker struct {
    hub *Hub
}

// CheckHealth 检查系统健康状态
func (hc *HealthChecker) CheckHealth() HealthStatus {
    return HealthStatus{
        Status:  "healthy",
        Checks: map[string]CheckResult{
            "control_service": hc.checkControlService(),
            "tunnel_service":  hc.checkTunnelService(),
        },
    }
}
```

---

## 11. 总结

本详细设计文档在概要设计基础上，进一步明确了：

* 每个组件的类结构和方法职责
* 数据结构和协议格式
* 并发控制和资源管理策略
* 错误处理和测试设计
* 部署运维相关功能

该设计确保了代码实现的可行性和系统的高质量，为后续开发提供了完整的技术指导。
