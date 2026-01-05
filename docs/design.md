# SSH 内网穿透系统设计文档

## 1. 系统概述

### 1.1 背景
在现代云原生环境中，开发人员经常需要访问部署在私有网络或容器中的服务。由于安全策略，这些服务通常不直接暴露在公网上。传统的解决方案如VPN或端口转发存在配置复杂、管理困难等问题。

### 1.2 目标
设计一个轻量级、安全的SSH内网穿透系统，允许开发人员通过标准SSH客户端安全地访问内网资源，无需复杂的网络配置。

### 1.3 核心特性
- **零配置**: 客户端无需复杂配置，开箱即用
- **安全性**: 多层认证，防止未授权访问
- **轻量级**: 最小化资源占用，适合容器化部署
- **可扩展**: 支持多租户，多Agent并发管理
- **透明性**: 对现有SSH工具完全兼容

## 2. 架构设计

### 2.1 系统架构图
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Development   │     │      Hub        │     │     Agent       │
│   Machine       │◄───►│  (Public)      │◄───►│  (Private)      │
│                 │     │                 │     │                 │
│ • SSH Client    │     │ • WebSocket     │     │ • SSH Server    │
│ • Entry Client  │     │ • RPC API       │     │ • Tunnel Agent  │
│                 │     │ • Auth Service  │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   Control      │
                    │   Plane        │
                    └─────────────────┘
```

### 2.2 组件职责

#### Hub (中转服务器)
**位置**: 公网可访问的服务器
**职责**:
- 作为所有通信的中转点
- 管理Agent注册和发现
- 处理WebSocket隧道连接
- 提供管理API接口
- 执行认证和授权

#### Agent (内网代理)
**位置**: 目标内网环境
**职责**:
- 在内网启动SSH服务器
- 建立到Hub的控制连接
- 接受来自Hub的隧道连接请求
- 执行SSH密钥管理

#### Entry (开发机客户端)
**位置**: 开发人员工作机器
**职责**:
- 发现可用的Agent
- 建立本地SSH端口映射
- 管理SSH密钥对
- 处理用户认证

### 2.3 数据流设计

#### 控制流
```
Entry → Hub: Agent发现请求
Hub → Entry: Agent列表响应

Agent → Hub: 注册请求
Hub → Agent: 注册确认 + 分配端口
```

#### 数据流
```
SSH Client → Entry (本地端口)
Entry → Hub (WebSocket隧道)
Hub → Agent (WebSocket隧道)
Agent → SSH Server (本地连接)
SSH Server → Target Container
```

## 3. 组件详细设计

### 3.1 Hub 设计

#### 核心服务
- **WebSocket服务**: 处理双向隧道连接
- **RPC服务**: 处理Agent管理和配置
- **认证服务**: 验证客户端身份
- **路由服务**: 将请求路由到正确的Agent

#### 状态管理
- Agent注册表: 维护活跃Agent信息
- 连接池: 管理WebSocket连接
- 会话管理: 跟踪活跃的隧道会话

#### 安全措施
- TLS加密所有通信
- 基于令牌的认证
- 请求频率限制
- 连接超时管理

### 3.2 Agent 设计

#### 服务架构
- **控制模块**: 负责与Hub通信
- **SSH模块**: 提供SSH服务
- **隧道模块**: 处理WebSocket隧道

#### 生命周期
1. 启动时生成SSH密钥对
2. 连接到Hub并注册
3. 监听控制指令
4. 接受隧道连接请求
5. 转发SSH流量

#### 资源管理
- 自动端口分配
- 连接池管理
- 内存使用优化
- 优雅关闭处理

### 3.3 Entry 设计

#### 用户交互
- 命令行界面
- 自动密钥管理
- Agent选择界面
- 连接状态显示

#### 连接管理
- 本地端口监听
- WebSocket连接池
- 自动重连机制
- 连接健康检查

## 4. 接口设计

### 4.1 外部接口

#### SSH 接口
```bash
# 标准SSH连接
ssh user@localhost -p <local_port>

# 端口由系统自动分配和管理
```

#### 命令行接口
```bash
# 连接到指定Agent
sshole connect <agent_name>

# 列出可用Agent
sshole list

# 断开连接
sshole disconnect
```

### 4.2 内部接口

#### RPC 接口设计
```protobuf
service TunnelService {
  rpc RegisterAgent(RegisterRequest) returns (RegisterResponse);
  rpc ListAgents(ListRequest) returns (ListResponse);
  rpc CreateTunnel(TunnelRequest) returns (TunnelResponse);
  rpc AppendPublicKey(KeyRequest) returns (KeyResponse);
}

message RegisterRequest {
  string agent_name = 1;
  string auth_token = 2;
}

message RegisterResponse {
  int32 assigned_port = 1;
  string hub_public_key = 2;
  string session_token = 3;
}
```

#### WebSocket 协议
- **控制信道**: JSON格式的控制指令
- **数据信道**: 二进制数据透传
- **心跳机制**: 定期发送ping/pong帧

### 4.3 配置接口

#### 环境变量配置
```bash
# Hub配置
HUB_HOST=hub.example.com
HUB_PORT=9000
AUTH_TOKEN=your_token

# Agent配置
HUB_HOST=hub.example.com
AUTH_TOKEN=your_token
AGENT_NAME=my-service
SSH_PORT=2222

# Entry配置
DEFAULT_AGENT=production-db
```

## 5. 安全设计

### 5.1 认证体系

#### 多层认证
1. **令牌认证**: Hub层面的API访问控制
2. **SSH密钥认证**: 传输层安全
3. **会话认证**: 连接级别的身份验证

#### 密钥管理
- **自动生成**: 系统自动生成ed25519密钥对
- **安全存储**: 密钥存储在安全位置
- **定期轮换**: 支持密钥定期更新

### 5.2 访问控制

#### 网络隔离
- Agent只监听localhost
- 所有外部通信通过Hub
- 防火墙规则限制访问

#### 权限模型
- 基于角色的访问控制
- Agent级别的权限隔离
- 用户级别的连接限制

### 5.3 安全措施

#### 通信安全
- TLS 1.3加密
- 完美前向保密
- 证书固定

#### 防护措施
- SQL注入防护
- XSS防护
- DDoS防护

## 6. 部署设计

### 6.1 部署架构

#### 单Hub多Agent
```
Internet
    │
    ▼
┌─────────┐
│   Hub   │ ←── 公网访问
│         │
└─────────┘
    │
    ├─────────────┬─────────────┐
    ▼             ▼             ▼
┌─────────┐ ┌─────────┐ ┌─────────┐
│ Agent 1 │ │ Agent 2 │ │ Agent N │
│         │ │         │ │         │
└─────────┘ └─────────┘ └─────────┘
```

#### 高可用设计
- 多Hub实例负载均衡
- Agent自动故障转移
- 无状态设计保证可扩展性

### 6.2 容器化部署

#### Docker Compose 示例
```yaml
version: '3.8'
services:
  hub:
    image: sshole/hub:latest
    ports:
      - "9000:9000"
    environment:
      - AUTH_TOKEN=your_secret_token

  agent:
    image: sshole/agent:latest
    environment:
      - HUB_HOST=hub
      - AUTH_TOKEN=your_secret_token
      - AGENT_NAME=web-service
    depends_on:
      - hub
```

### 6.3 配置管理

#### 配置层次
1. **默认配置**: 内置安全默认值
2. **环境配置**: 环境变量覆盖
3. **文件配置**: YAML/JSON配置文件
4. **运行时配置**: API动态配置

## 7. 状态恢复和重试机制设计

### 7.1 设计原则

由于系统采用无状态设计，各模块重启后会丢失所有运行时状态。为保证系统的高可用性和用户体验，需要精心设计自动重试和状态恢复机制。

**核心原则**：
- **零配置恢复**: 重启后无需人工干预即可恢复连接
- **渐进式重试**: 使用指数退避算法避免雪崩效应
- **优雅降级**: 在部分组件不可用时保持基本功能
- **状态一致性**: 确保恢复后的状态与重启前一致

### 7.2 Hub状态恢复

#### Agent注册恢复
```
Agent重启 → 自动重新注册
    ↓
Hub: 重新分配端口，更新注册表
    ↓
Entry: 通过心跳检测到Agent重新上线
```

**实现策略**：
- Agent启动后立即尝试连接Hub
- 使用指数退避重试：1s → 2s → 4s → 8s → 最大60s
- Hub接收注册时检查端口冲突，自动重新分配
- 广播Agent状态变更给所有连接的Entry

#### 连接会话恢复
- 维护连接ID映射表，支持断线重连
- WebSocket断开后保留会话上下文5分钟
- 超时后清理过期会话，释放资源

### 7.3 Agent状态恢复

#### 注册重试机制
```go
// 伪代码示例
func connectWithRetry() {
    for attempt := 0; attempt < maxRetries; attempt++ {
        if connectToHub() {
            registerAgent()
            startHeartbeat()
            return
        }
        sleep(calculateBackoff(attempt))
    }
    log.Fatal("Failed to connect to Hub after max retries")
}
```

**关键特性**：
- **自愈能力**: Agent独立恢复，无需外部干预
- **智能重试**: 区分临时失败和永久失败
- **状态同步**: 重新注册后同步最新配置

#### SSH服务重启
- Agent重启时自动重启SSH服务器
- 保留authorized_keys文件（如果挂载了持久化存储）
- 重新生成SSH主机密钥（如果未持久化）

### 7.4 Entry状态恢复

#### 连接重建流程
```
Entry重启 → 检查上次连接的Agent
    ↓
重新发现可用Agent列表
    ↓
自动重连上次使用的Agent
    ↓
重建本地端口映射
```

**用户体验优化**：
- **无缝恢复**: 用户的SSH连接在Entry重启期间保持可用
- **连接记忆**: 记住最后使用的Agent和端口映射
- **智能重连**: 优先重连健康状态的Agent

#### 密钥管理恢复
- SSH密钥对自动重新生成或从安全存储恢复
- 公钥自动重新分发到目标Agent
- 支持密钥轮换和更新机制

### 7.5 跨组件协调

#### 心跳和健康检查
- **Hub-Agent心跳**: 30秒间隔，检测连接健康
- **Agent-Entry心跳**: 通过WebSocket隧道传递
- **超时处理**: 3次心跳失败后标记为断开

#### 故障检测和恢复
```
检测到连接断开 → 立即尝试重连
    ↓
重连失败 → 指数退避重试
    ↓
多次失败 → 标记服务降级
    ↓
定期重试恢复正常服务
```

#### 级联恢复
1. **Hub重启**: Agent检测到断开，立即重连注册
2. **Agent重启**: Entry检测到服务不可用，显示状态并等待恢复
3. **网络中断**: 所有组件独立重试，恢复后自动同步状态

### 7.6 重试策略配置

#### 可配置参数
```yaml
retry:
  max_attempts: 10
  initial_delay: 1s
  max_delay: 60s
  backoff_multiplier: 2.0
  jitter: true

health_check:
  interval: 30s
  timeout: 10s
  failure_threshold: 3
```

#### 自适应调整
- 根据网络条件动态调整重试间隔
- 检测到持续失败时增加重试间隔
- 恢复正常后重置为初始配置

### 7.7 故障场景处理

#### 常见故障模式
1. **Hub临时不可用**: Agent和Entry独立重试，缓存请求
2. **Agent崩溃**: Entry检测并切换到备用Agent
3. **网络分区**: 各组件在分区内继续工作，恢复后同步
4. **证书过期**: 自动重新生成和分发证书

#### 降级策略
- **Hub不可用**: Agent继续运行SSH服务，本地缓存注册信息
- **部分Agent失败**: Entry自动切换到健康的Agent
- **网络拥塞**: 降低心跳频率，减少重试频率

### 7.8 监控和告警

#### 恢复指标
- 连接恢复成功率
- 平均恢复时间
- 重试次数分布
- 故障持续时间

#### 告警规则
- 恢复失败率超过阈值
- 单个组件重试次数过多
- 级联故障检测

## 8. 可扩展性设计

### 8.1 水平扩展
- Hub集群支持
- Agent自动发现
- 负载均衡机制

### 8.2 功能扩展
- 插件架构设计
- 自定义认证模块
- 第三方集成接口

### 8.3 监控和运维
- 指标收集接口
- 日志聚合设计
- 健康检查端点

## 9. 运维考虑

### 8.1 监控指标
- 连接数统计
- 流量统计
- 错误率监控
- 性能指标

### 8.2 日志设计
- 结构化日志
- 日志级别控制
- 分布式追踪

### 8.3 故障处理
- 自动重连机制
- 优雅降级
- 故障恢复流程

## 10. 总结

本设计提供了一个完整、安全、可扩展的SSH内网穿透解决方案。通过精心设计的架构和接口，该系统能够在保证安全性的同时，提供出色的用户体验和运维便利性。

关键设计原则：
- **安全性优先**: 多层认证和加密
- **简单易用**: 零配置体验
- **可扩展性**: 模块化架构设计
- **运维友好**: 完善的监控和日志

该设计为后续实现提供了清晰的技术路径和架构指导。
