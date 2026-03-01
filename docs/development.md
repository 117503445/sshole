# 开发指南

本文档描述 sshole 项目的开发注意事项和代码规范。

## 快速开始

```bash
# 克隆项目
git clone https://github.com/117503445/sshole.git
cd sshole

# 安装依赖
go mod download

# 运行 E2E 测试（验证环境）
go-task base:dev
```

## 技术规范

### 构建工具

- 使用 **go-task** 运行构建任务，不是 make
- Taskfile 位于 `Taskfile.yml` 和 `scripts/tasks/` 目录

### 日志库

使用 **zerolog** 作为日志库：

```go
import "github.com/rs/zerolog/log"

// 使用 context 相关的 logger
logger := log.Ctx(ctx)
logger.Info().Str("key", "value").Msg("message")
```

**日志规范**：
- 默认日志级别：Debug
- 使用 context 传递 logger
- 结构化日志，使用 `.Str()`, `.Int()` 等方法添加字段

### Go 版本

- 最低版本：Go 1.25.5
- 使用 Go modules 管理依赖

## 代码规范

### 包结构

```
pkg/
├── common/    # 通用类型、错误、工具（可被所有包导入）
├── hub/       # Hub 组件（仅导入 common, proto, tunnel）
├── agent/     # Agent 组件（仅导入 common, proto, tunnel）
├── entry/     # Entry 组件（仅导入 common）
├── proto/     # 协议定义
├── tunnel/    # 隧道相关逻辑
├── rpc/       # RPC 定义
└── utils/     # 工具函数
```

### 依赖原则

- `common` 包不依赖其他内部包
- `tunnel` 包仅依赖 `common`
- `hub`、`agent`、`entry` 包之间不互相依赖
- 避免循环依赖

### 错误处理

使用自定义错误码（定义在 `pkg/common/errors.go`）：

```go
// pkg/common/errors.go
type ErrCode string

const (
    ErrAuthFailed      ErrCode = "AUTH_FAILED"
    ErrAgentOffline    ErrCode = "AGENT_OFFLINE"
    ErrSessionNotFound ErrCode = "SESSION_NOT_FOUND"
    // ...
)
```

### 并发安全

- 使用 `sync.RWMutex` 保护共享状态
- 明确资源释放顺序：
  1. 关闭 SSH TCP 连接
  2. 关闭 WebSocket 隧道
  3. 从 map 中清理会话
  4. 记录日志

### Context 使用

- 所有长时间运行的操作应接收 `context.Context`
- 使用 `context.WithTimeout` 设置超时
- 在循环中使用 `select` 监听 context 取消信号

## 测试规范

### 单元测试

```bash
# 运行所有单元测试
go test ./...

# 运行特定包的测试
go test ./pkg/tunnel/...

# 运行带覆盖率的测试
go test -cover ./...
```

### 测试文件命名

- 测试文件以 `_test.go` 结尾
- 与被测试文件放在同一目录

### E2E 测试

E2E 测试代码位于 `scripts/script/pkg/e2e/`：

```bash
go-task test:e2e
```

E2E 测试流程：
1. 构建二进制文件和 Docker 镜像
2. 启动 Hub、Agent、Entry 容器
3. 验证 SSH 连接
4. 清理容器

## 代码生成

### RPC 代码生成

```bash
go-task gen:gen
```

生成的内容：
- `pkg/rpc/v1/sshole.pb.go` - Protobuf 消息定义
- `pkg/rpc/v1/rpcv1connect/sshole.connect.go` - ConnectRPC 服务定义

### 依赖

- [buf](https://buf.build/) - Protobuf 工具链
- [connect-go](https://connectrpc.com/docs/go/getting-started) - RPC 框架

## 代码风格

### 格式化

```bash
go-task format:all
```

使用 `gofmt` 和 `goimports` 格式化代码。

### 命名约定

- 包名：小写单词，简洁明了
- 接口：动词或描述性名词，如 `Handler`、`Client`
- 结构体：名词，如 `Hub`、`Agent`
- 常量：驼峰式，如 `HandshakeMagic`
- 错误码：大写下划线，如 `ErrAuthFailed`

### 注释规范

- 导出的函数和类型必须有注释
- 注释以函数/类型名开头
- 复杂逻辑添加行内注释

```go
// NewHub creates a new Hub instance with the given configuration.
func NewHub(cfg *HubConfig) (*Hub, error) {
    // ...
}
```

## 调试技巧

### 日志级别调整

在代码中设置日志级别：

```go
import "github.com/rs/zerolog"

zerolog.SetGlobalLevel(zerolog.DebugLevel)
```

### 本地调试

```bash
# 构建
go-task build:bin

# 启动 Hub
./data/hub/sshole_hub --auth-token test --http-addr :9000

# 启动 Agent（另一个终端）
./data/agent/sshole_agent --hub-url ws://localhost:9000 --token test --agent-name test

# 测试连接
ssh localhost -p <hubPort>
```

## 常见问题

### 端口冲突

如果 Hub 启动失败提示端口不可用：
1. 检查端口映射文件中的端口是否被占用
2. 修改映射文件中的端口
3. 重启 Hub

### Agent 无法连接

1. 检查 Hub URL 是否正确（注意 `ws://` 或 `wss://`）
2. 检查 Token 是否匹配
3. 检查网络连通性

### SSH 连接超时

1. 检查 Agent 是否在线
2. 检查 `pending_timeout` 设置是否合理
3. 查看日志排查问题

## 参考文档

- [构建与测试](build-test.md)
- [项目架构](architecture.md)
- [配置说明](configuration.md)
- [概要设计说明书](design.high-level.md)
- [详细设计说明书](design.detailed.md)