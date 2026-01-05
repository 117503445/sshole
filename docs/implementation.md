# sshole - SSH 内网穿透系统实现文档

## 系统架构实现

### Hub (中转服务器)
- **端口**: 9000 (HTTP/2 over cleartext)
- **技术栈**: Connect RPC, WebSocket, HTTP/2

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

### 阿里云集成
- `github.com/117503445/fc-go-sdk`: 函数计算SDK
- `github.com/alibabacloud-go/fc-20230330/v4`: 阿里云FC服务

## 系统总结

这个系统为开发者和运维人员提供了一种安全、便捷的方式来访问内网SSH资源，特别适用于云原生环境中的容器调试和运维场景。
