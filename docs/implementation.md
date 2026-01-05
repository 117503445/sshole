# sshole - SSH 内网穿透系统实现文档

## 系统架构实现

### Hub (中转服务器)
- **端口**: 9000 (HTTP/2 over cleartext)
- **技术栈**: Connect RPC, WebSocket, HTTP/2
- **路由**:
  - `/bin`: 二进制文件下载服务
  - `/healthz`: 健康检查服务

### Agent (内网代理)
- **SSH服务器**: 默认端口22222
- **访问控制**: 只允许localhost (127.0.0.1)访问
- **认证方式**: 公钥认证，禁用密码登录
- **密钥类型**: ed25519

### Entry (开发机客户端)
- **SSH密钥**: ed25519密钥对
- **默认密钥路径**:
  - 私钥: `~/.ssh/id_ed25519`
  - 公钥: `~/.ssh/id_ed25519.pub`
- **监听端口**: 默认22222

## 配置参数实现

### Agent 配置参数
- `HUB_SERVER`: Hub服务器地址
- `AUTH`: 认证密钥
- `NAME`: Agent名称 (默认为主机名)
- `SSHD_PORT`: SSH服务器端口 (默认22222)

### Entry 配置参数
- `HUB_SERVER`: Hub服务器地址
- `AUTH`: 认证密钥
- `AGENT_NAME`: 目标Agent名称 (可选，不指定时显示列表)
- `SSH_PORT`: 本地SSH监听端口 (默认22222)
- `PRIVATE_KEY`: 私钥文件路径 (默认 `~/.ssh/id_ed25519`)
- `PUBLIC_KEY`: 公钥文件路径 (默认 `~/.ssh/id_ed25519.pub`)

## RPC 接口实现

基于Protocol Buffers和Connect RPC框架的API服务：

### AgentCreate
- **功能**: 注册Agent到Hub
- **输入**: Agent名称
- **输出**: 分配的端口号和Hub公钥

### AgentList
- **功能**: 获取所有注册的Agent列表
- **输入**: 无
- **输出**: Agent列表 (名称和端口)

### AgentGet
- **功能**: 获取指定Agent信息
- **输入**: Agent名称
- **输出**: Agent详细信息

### AgentAppendPublicKey
- **功能**: 将SSH公钥追加到指定Agent的authorized_keys
- **输入**: Agent名称和SSH公钥
- **输出**: 操作结果

## 安全实现

### 认证机制
- **Hub认证**: 通过AUTH环境变量进行身份验证
- **SSH认证**: 基于ed25519密钥对的公钥认证
- **WebSocket认证**: 连接建立时通过auth参数验证

### 安全特性
- SSH服务器只监听localhost (127.0.0.1)
- 禁用密码认证，只允许公钥认证
- 密钥文件权限: 私钥600，公钥644
- 防止非授权用户使用系统

## 部署实现

### Docker 部署
- **镜像命名**: `117503445/sshole-*`
- **网络模式**: 使用Docker网络进行容器间通信
- **数据持久化**: 支持缓存和配置持久化

### 端到端测试 (E2E)
- **测试流程**:
  1. 启动Hub容器
  2. 等待1秒，启动Agent容器
  3. 等待3秒，启动Entry容器
  4. 运行10秒后停止测试
- **验证标准**: 标准输出中无错误信息

### 开发环境
- **构建工具**: Task (Go Task)
- **代码生成**: Protocol Buffers生成
- **测试框架**: 标准Go测试框架

## Tunnel 模块实现

### 核心功能
基于 `github.com/coder/websocket` 实现双向端口转发：
- **本地端口映射到远程端口** (localToRemote)
- **远程端口映射到本地端口** (remoteToLocal)

### TunnelManager
- **唯一性**: 每个进程拥有唯一的TunnelManager实例，全局唯一ID
- **认证**: 每个TunnelManager持有一个auth字符串，用于身份验证
- **连接管理**: 支持多个WebSocket连接，每个连接可承载多个Tunnel

### 连接特性 (Conn)
- **心跳机制**: 每5秒发送心跳包，12秒超时标记为断开
- **多路复用**: 一个WebSocket连接可承载多个端口映射
- **状态监控**: 实时跟踪连接状态 (connected/disconnected)

### 端口映射类型
- **LocalToRemote**: 本地监听端口，流量转发至远程指定端口
- **RemoteToLocal**: 远程监听端口，流量转发至本地指定端口

## 技术依赖

### 核心依赖
- `github.com/coder/websocket`: WebSocket通信
- `connectrpc.com/connect`: RPC框架
- `github.com/google/uuid`: 唯一标识生成
- `github.com/rs/zerolog`: 结构化日志
- `golang.org/x/crypto/ssh`: SSH功能支持

## 系统总结

这个系统为开发者和运维人员提供了一种安全、便捷的方式来访问内网SSH资源，特别适用于云原生环境中的容器调试和运维场景。
