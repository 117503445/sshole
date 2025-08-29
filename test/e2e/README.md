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
├── Dockerfile           # Docker 构建文件
├── Containerfile        # Podman 构建文件
├── docker-compose.yml   # Docker Compose 配置
├── podman-compose.yml   # Podman Compose 配置
├── run-tests.sh         # 测试运行脚本
└── README.md           # 本文档
```

## 快速开始

### 使用 Podman（推荐）

```bash
# 运行完整的 E2E 测试套件
./test/e2e/run-tests.sh podman

# 或者直接使用 podman-compose
cd test/e2e
podman-compose up --build
```

### 使用 Docker

```bash
# 运行完整的 E2E 测试套件
./test/e2e/run-tests.sh docker

# 或者直接使用 docker-compose
cd test/e2e
docker-compose up --build
```

### 本地开发测试

```bash
# 本地运行测试（需要先启动各个组件）
./test/e2e/run-tests.sh local

# 或者只构建二进制文件
./test/e2e/run-tests.sh build
```

## 测试组件

### 独立运行组件

每个组件都可以独立运行用于调试：

```bash
# 构建测试二进制
./test/e2e/run-tests.sh build

# 运行 Hub
./test/e2e/e2e-test --component=hub

# 运行 Agent
./test/e2e/e2e-test --component=agent --hub=ws://localhost:8080 --token=test-token --id=test-agent

# 运行 Entry
./test/e2e/e2e-test --component=entry --hub=ws://localhost:8080 --token=test-token --id=test-entry --local=:10022

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

## Docker/Podman 配置

### 服务架构

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Agent     │◄──►│     Hub     │◄──►│    Entry    │
│             │    │             │    │             │
│ ws://hub:8080   │    :8080     │    :10022      │
└─────────────┘    └─────────────┘    └─────────────┘
                      ▲
                      │
               ┌─────────────┐
               │ Test Runner │
               │             │
               └─────────────┘
```

### 网络配置

所有服务运行在 `sshole-test` 网络中，支持服务发现：
- `hub`: Hub 服务
- `agent`: Agent 服务
- `entry`: Entry 服务
- `test-runner`: 测试运行器

## 开发和调试

### 查看日志

```bash
# Docker
docker-compose logs -f

# Podman
podman-compose logs -f
```

### 进入容器

```bash
# Docker
docker-compose exec test-runner sh

# Podman
podman-compose exec test-runner sh
```

### 清理测试环境

```bash
./test/e2e/run-tests.sh clean
```

## 故障排除

### 常见问题

1. **端口冲突**: 确保 8080 和 10022 端口未被占用
2. **网络问题**: 检查 Docker/Podman 网络配置
3. **权限问题**: 确保有权限运行容器

### 调试模式

启用详细日志：

```bash
export LOG_LEVEL=debug
./test/e2e/run-tests.sh podman
```

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

可以通过修改 compose 文件来创建不同的测试场景：

```yaml
# 自定义场景示例
services:
  custom-agent:
    # 自定义配置
  custom-hub:
    # 自定义配置
```

## CI/CD 集成

可以在 CI/CD 流水线中使用：

```yaml
# GitHub Actions 示例
- name: Run E2E Tests
  run: |
    cd test/e2e
    chmod +x run-tests.sh
    ./run-tests.sh podman
```

## 许可证

本测试代码遵循项目主许可证。
