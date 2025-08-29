# SSHole E2E Tests

这个目录包含了 SSHole 项目的端到端 (E2E) 测试，用于验证整个系统的完整功能。

## 架构概述

SSHole 系统包含三个主要组件：
- **Agent**: 代理组件，连接到 Hub 并处理 SSH 连接
- **Hub**: 中心组件，管理 WebSocket 连接和消息路由
- **Entry**: 入口组件，提供本地 SSH 服务并转发连接

E2E 测试验证这些组件之间的完整交互流程。

## 测试文件结构

```
test/e2e/
├── main.go              # E2E 测试主程序
├── tools/
│   └── runner.go        # Go 语言测试运行器
└── README.md           # 本文档
```

## 快速开始

### 本地运行测试

```bash
# 运行完整的 E2E 测试套件（推荐）
go run test/e2e/tools/runner.go local

# 或者只构建二进制文件
go run test/e2e/tools/runner.go build

# 清理测试产物
go run test/e2e/tools/runner.go clean
```

## 测试组件

### 独立运行组件

每个组件都可以独立运行用于调试：

```bash
# 构建测试二进制
go run test/e2e/tools/runner.go build

# 运行 Hub
./test/e2e/e2e-test --component=hub

# 运行 Agent
./test/e2e/e2e-test --component=agent --hub=ws://localhost:8080 --token=test-token --id=test-agent

# 运行 Entry
./test/e2e/e2e-test --component=entry --hub=ws://localhost:8080 --token=test-token --id=test-entry --local=localhost:10022

# 运行测试器
./test/e2e/e2e-test --component=test-runner
```

### 组件配置参数

- `--component`: 组件类型 (hub, agent, entry, test-runner)
- `--hub`: Hub WebSocket 地址 (默认: ws://localhost:8080)
- `--token`: 认证令牌 (默认: test-token)
- `--id`: 客户端 ID (默认: test-client)
- `--local`: Entry 本地监听地址 (默认: :10022)

## 测试套件

E2E 测试包含以下测试用例：

1. **健康检查 (Health Check)**: 验证 Hub 的健康检查端点
2. **WebSocket 连接测试**: 验证 WebSocket 连接功能
3. **SSH 连接测试**: 验证 SSH 服务连接
4. **端到端流程测试**: 验证完整的数据流

## 开发和调试

### 查看日志

本地运行时，测试程序会直接输出日志到控制台，无需额外配置。

### 清理测试环境

```bash
go run test/e2e/tools/runner.go clean
```

## 故障排除

### 常见问题

1. **端口冲突**: 确保 8080 和 10022 端口未被占用
2. **构建失败**: 确保 Go 环境正确安装且所有依赖已下载
3. **权限问题**: 确保有权限运行 Go 程序和创建临时文件

### 调试模式

运行测试时会显示详细的日志信息，帮助诊断问题。测试程序会自动启动和停止必要的服务组件。

## 扩展测试

### 添加新的测试用例

在 `main.go` 的 `runTestSuite` 方法中添加新的测试：

```go
tests := []struct {
    name string
    fn   func() error
}{
    {"Existing Test", existingTest},
    {"New Test", newTestFunction},
}
```

### 自定义测试场景

可以通过修改测试程序的配置参数来创建不同的测试场景。例如：

```bash
# 使用不同的端口
go run test/e2e/tools/runner.go local

# 修改测试参数需要在代码中调整
# 可以修改 main.go 中的连接地址和测试参数
```

## CI/CD 集成

可以在 CI/CD 流水线中使用：

```yaml
# GitHub Actions 示例
- name: Run E2E Tests
  run: |
    go run test/e2e/tools/runner.go local
```

## 许可证

本测试代码遵循项目主许可证。
