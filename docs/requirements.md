# sshole - SSH 内网穿透系统需求文档

## 项目概述

sshole是一个基于Go开发的SSH连接代理系统，通过WebSocket实现内网穿透，允许从开发机安全地SSH连接到内网容器。该系统采用分布式架构，由三个核心组件组成：Hub（中转服务器）、Agent（内网代理）和Entry（开发机客户端）。

## 功能需求

### 1. Hub (中转服务器)
- **位置**: 部署在公网服务器上
- **功能**:
  - 作为WebSocket中转服务器处理Agent和Entry之间的数据转发
  - 提供RPC API管理Agent注册和查询
  - 提供二进制文件下载服务 (`/bin` 路由)
  - 提供健康检查服务 (`/healthz` 路由)
  - 实现认证机制防止非授权访问

### 2. Agent (内网代理)
- **位置**: 运行在目标内网容器中
- **功能**:
  - 建立与Hub的WebSocket连接
  - 启动嵌入式OpenSSH服务器 (默认端口22222)
  - 只允许localhost访问SSH服务 (127.0.0.1)
  - 使用公钥认证禁用密码登录
  - 自动注册到Hub并获取分配的端口
  - 通过TunnelManager实现远程端口到本地SSH端口的映射

### 3. Entry (开发机客户端)
- **位置**: 运行在开发机上
- **功能**:
  - 自动生成或使用现有的ed25519密钥对
  - 连接到Hub查询可用Agent
  - 将开发机的SSH公钥追加到目标Agent
  - 通过TunnelManager建立本地端口到Hub的端口映射
  - 在本地端口监听SSH连接并转发到目标Agent

## 配置需求

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

## RPC 接口需求

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

## 安全需求

### 认证机制
- **Hub认证**: 通过AUTH环境变量进行身份验证
- **SSH认证**: 基于ed25519密钥对的公钥认证
- **WebSocket认证**: 连接建立时通过auth参数验证

### 安全特性
- SSH服务器只监听localhost (127.0.0.1)
- 禁用密码认证，只允许公钥认证
- 所有密钥文件权限设置为600 (私钥) 或644 (公钥)
- 防止非授权用户使用系统

## 部署需求

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

## 待完成功能 (TODO)

根据代码注释中的TODO列表：

- [x] 认证，防止非授权用户使用
- [ ] agent 追加 SSH 公钥
- [ ] 单实例多 conn，防止 SSHD 冲突
- [ ] sshd 临时文件夹
- [ ] 多版本 cli 的 fc 函数冲突
- [ ] fc3 instance

## 项目特色

1. **轻量级**: 基于Go原生WebSocket，无额外代理依赖
2. **安全**: 多重认证机制，严格的访问控制
3. **易用**: 自动密钥生成，一键连接内网资源
4. **可扩展**: 模块化设计，支持多Agent并发管理
5. **云原生**: 完整Docker容器化支持
