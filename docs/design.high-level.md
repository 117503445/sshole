# sshole 概要设计说明书

**设计级别**：概要设计（High-Level Design）

---

## 1. 引言

### 1.1 目的

本文档描述 **sshole** 系统的总体架构、核心设计原则、组件职责、外部接口、协议约定及运行模型，为系统实现、评审与后续演进提供统一、稳定的设计依据。

### 1.2 设计目标

* 提供一种 **无需 VPN、对 SSH 完全透明** 的内网访问方案
* 支持 **多个 SSH 客户端并发访问同一个内网 Agent**
* 架构清晰、协议极简、实现复杂度可控
* 易于部署、运维与问题定位

---

## 2. 系统概述

### 2.1 系统定义

sshole 是一个基于 **反向连接 + 中转转发** 的 SSH 内网穿透系统，由三类组件组成：

* **Hub**：公网可访问的中转节点，对外提供 SSH 入口
* **Agent**：部署在内网环境中的代理节点，暴露本地 SSH 服务
* **Entry（可选）**：运行在开发机上的本地入口增强组件

用户可以 **直接使用标准 SSH 客户端连接 Hub**，访问位于内网中的 Agent。

---

### 2.2 核心设计原则

#### 2.2.1 控制面与数据面分离

* **控制面**

  * Agent 注册与心跳
  * Agent 发现与状态查询
  * hubPort 分配
  * 鉴权与授权

* **数据面**

  * 仅承载 SSH TCP 字节流
  * 不承载任何业务语义
  * 不维护业务状态

---

#### 2.2.2 最小隧道模型

* **1 SSH TCP 连接 = 1 数据隧道**
* 每个 SSH 会话 **动态创建隧道**
* 会话结束即销毁隧道
* 不做隧道复用
* 不做多路复用
* 不在数据面引入自定义协议

该模型在支持并发访问的前提下，保持了实现复杂度的最小化，是 v1 的基础设计。

---

## 3. 系统架构

### 3.1 组件职责

#### Hub（公网）

* 提供控制面服务（注册、发现、鉴权）
* 在固定 `hubPort` 上监听公网 SSH 入口
* 为每个 SSH 会话动态创建并管理数据隧道
* 将数据流转发至目标 Agent

#### Agent（内网）

* 启动后向 Hub 注册并保持在线
* 接收 Hub 发起的隧道连接
* 将隧道数据转发至本地 SSHD
* 本地 SSHD **仅监听 localhost**

#### Entry（可选，开发机）

* 提供本地 SSH 入口（如 `localhost:port`）
* 简化 Hub 访问方式
* 提供 Agent 选择、状态显示、公钥管理等辅助能力
* **不是系统必需组件**

---

### 3.2 端口与连接模型（关键定义）

| 名称               | 归属    | 含义                            |
| ---------------- | ----- | ----------------------------- |
| Agent Local Port | Agent | 内网 SSHD 端口，仅监听 `127.0.0.1`    |
| Hub Port         | Hub   | Hub 对外监听的 SSH 入口端口，绑定某个 Agent |
| Entry Local Port | Entry | 本地监听端口（可选），作为 SSH 入口          |

---

### 3.3 数据流（主路径）

```
SSH Client → Hub(hubPort) → Agent(localPort) → SSHD
```

> Entry 为可选增强路径，不影响主数据流。

---

## 4. 核心流程设计（最终模型）

sshole 采用 **“入口长期存在、隧道按会话创建”** 的运行模型，
支持多个 SSH 客户端并发访问同一个 Hub 端口。

---

### 4.1 Agent 上线与入口准备（控制面，长期存在）

1. Agent 启动并读取配置（`agentName`、`localPort`、鉴权信息）
2. Agent 通过控制面向 Hub 发起注册请求
3. Hub 对请求进行鉴权
4. Hub 为该 `agentName` 分配或恢复一个固定的 `hubPort`
5. Hub 记录 `hubPort → agentName` 的映射关系
6. **Hub 开始在该 `hubPort` 上对公网监听**
7. Agent 通过心跳机制维持在线状态

**说明**：

* 此阶段 **不建立任何具体 SSH 会话**
* Agent 上线意味着 Hub 已具备为其 **创建 SSH 隧道的能力**
* `hubPort` 在 Agent 在线期间保持稳定

---

### 4.2 用户访问：并发 SSH 到同一个 Hub 端口（数据面）

当一个或多个用户执行：

```bash
ssh user@hub -p <hubPort>
```

系统对 **每一个 SSH TCP 连接** 独立处理：

1. Hub 在 `hubPort` 上 `accept()` 到一个新的 TCP 连接
2. Hub 根据 `hubPort` 映射确定目标 `agentName`
3. Hub 为该 TCP 连接 **动态创建一条新的数据隧道**（WebSocket）
4. Agent 接收该隧道连接，并建立到本地 SSHD 的 TCP 连接：

   ```
   127.0.0.1:<localPort>
   ```
5. Hub 在以下两端之间进行双向字节转发：

   ```
   TCP(client ↔ hubPort)
     ↔ WebSocket Tunnel
       ↔ TCP(agent ↔ local SSHD)
   ```
6. 任意一侧连接关闭：

   * 当前 SSH 会话结束
   * 对应的数据隧道立即销毁

**关键约束**：

* **1 SSH 会话 = 1 数据隧道**
* 隧道为短生命周期资源
* 支持多个 SSH 客户端并发访问同一 `hubPort`

---

### 4.3 Entry（可选）：本地入口增强

Entry 不是系统必需组件，其职责包括：

* 将

  ```
  ssh user@hub -p <hubPort>
  ```

  简化为

  ```
  ssh user@localhost -p <entryPort>
  ```
* 提供 Agent 选择、状态展示
* 管理和分发用户 SSH 公钥

Entry 的存在 **不改变 Hub 作为直接 SSH 入口的核心模型**。

---

## 5. 外部接口设计

### 5.1 用户接口

#### SSH

```bash
ssh user@hub -p <hubPort>
```

#### CLI（示例）

```bash
sshole list
sshole connect <agentName> [--port <entry_local_port>]
sshole disconnect
sshole status
```

---

### 5.2 控制面接口（概要）

控制面通过 RPC 提供以下能力：

* Agent 注册与心跳
* Agent 列表与状态查询
* 查询 Agent 对应的 `hubPort`
* SSH 公钥分发（可选）

#### 语义约定

* Agent 注册对 `agentName` 幂等
* 仅返回在线 Agent
* 所有控制面接口均需鉴权

---

### 5.3 数据面接口（WebSocket Tunnel）

#### 协议约定

* 路径：`/tunnel`
* 帧类型：binary
* Payload：原始 TCP 字节流
* 模式：**1 SSH 会话 = 1 隧道**

#### Header 约定

* `Authorization: Bearer <token>`
* `X-Role: agent | entry`
* `X-Agent: <agentName>`
* `X-LocalPort: <int>`（仅 agent 必填）

---

### Go 外部接口（最简）

#### Hub

```go
type TunnelServer struct {
    ResolveHubPort func(agentName string) (hubPort int, ok bool)
}

func (ts *TunnelServer) Serve() http.Handler
```

#### Agent

```go
func (c *TunnelRClient) Connect(
    ctx context.Context,
    host string,
    auth string,
    agentName string,
    localPort int,
) (hubPort int, err error)
```

#### Entry

```go
func (c *TunnelClient) Connect(
    ctx context.Context,
    host string,
    auth string,
    agentName string,
    localPort int,
) (listenAddr net.Addr, err error)
```

---

## 6. 错误模型与失败场景

### 6.1 控制面错误（摘要）

| 场景        | 语义            |
| --------- | ------------- |
| 鉴权失败      | token 无效或缺失   |
| Agent 不存在 | agentName 未注册 |
| Agent 离线  | 注册存在但当前不可用    |
| 资源耗尽      | hubPort 无可用资源 |
| 内部错误      | Hub 内部异常      |

---

### 6.2 数据面错误（摘要）

#### 握手阶段

* 401：鉴权失败
* 404：Agent 不存在
* 412：Agent 离线或不可路由
* 429：限流
* 503：Hub 过载

#### 隧道阶段

* 正常关闭
* 网络异常
* Hub 主动关闭
* 空闲超时

---

## 7. 配置项设计（概要）

### 7.1 Hub

* 控制面 / 数据面监听地址
* 鉴权配置
* hubPort 分配范围
* Agent 超时阈值
* 日志级别

### 7.2 Agent

* Hub 地址
* 鉴权 token
* agentName
* 本地 SSHD 端口
* 心跳与重试参数

### 7.3 Entry

* Hub 地址
* 鉴权 token
* 默认 agentName
* 本地监听端口
* 用户密钥路径

---

## 8. 安全设计

### 8.1 鉴权

* 控制面与数据面均需鉴权
* 数据面在 WebSocket 握手阶段完成鉴权

### 8.2 最小暴露面

* Agent SSHD 仅监听 localhost
* Agent 不开放任何公网端口
* Hub 是唯一公网入口
* Entry 仅监听本机端口（如启用）

---

## 9. 可靠性与恢复

### 9.1 状态模型

* Hub：维护 Agent 注册表（可选持久化）
* Agent / Entry：无状态，通过重连恢复

### 9.2 故障处理

* Hub 重启：Agent 自动重注册，hubPort 可恢复
* Agent 离线：Hub 拒绝新的 SSH 连接
* 网络中断：对应 SSH 会话中断，不影响其他会话

---

## 10. 技术选型（简要声明）

* 编程语言：Go 1.25.5
* 控制面：connectrpc
* 数据面：WebSocket
* CLI：kong
* 日志：zerolog
* 通用工具：goutils

---

## 11. 设计边界与演进

### 11.1 v1 不包含

* 隧道复用或多路复用
* SSH 会话恢复
* Hub 多活高可用

### 11.2 演进方向

* 一次性 Tunnel 凭证
* 多租户权限模型
* Hub 高可用与状态同步

---

## 12. 总结

sshole v1 采用 **入口长期存在、隧道按会话创建** 的设计：

* 对 SSH 客户端完全透明
* 支持多客户端并发访问
* 数据面协议极简、实现成本低
* 为后续功能扩展预留清晰边界

该概要设计具备 **可实现性、可评审性和长期演进稳定性**。

---
