# 构建与测试

本文档描述 sshole 项目的构建和测试命令。

## 前置要求

- Go 1.25.5+
- Docker（用于开发环境和 E2E 测试）
- [go-task](https://taskfile.dev/) 任务运行器

> **注意**: 本项目使用 `go-task` 而不是 `make`。

## 构建命令

### 构建所有二进制文件

```bash
go-task build:bin
```

生成的二进制文件位于 `./data/` 目录：
- `./data/agent/sshole_agent`
- `./data/entry/sshole_entry`
- `./data/hub/sshole_hub`

### 构建 Docker 镜像

```bash
# 构建所有镜像
go-task build:docker

# 构建并推送
go-task build:docker -- --push
```

### 构建发布版本

```bash
go-task build:release
```

发布文件位于 `./data/release/` 目录。

## 测试命令

### 运行 E2E 测试

```bash
go-task test:e2e
```

E2E 测试会：
1. 构建二进制文件
2. 构建 Docker 镜像
3. 启动完整的环境（Hub、Agent、Entry）
4. 执行端到端连接测试

### 运行单元测试

```bash
go test ./...
```

### 运行特定包的测试

```bash
go test ./pkg/tunnel/...
```

## 开发环境

### 启动开发环境

```bash
go-task base:dev
```

默认任务会运行 E2E 测试。

### 清理

```bash
# 清理构建产物
rm -rf ./data/
```

## 代码生成

```bash
go-task gen:gen
```

用于生成 RPC 相关代码（protobuf）。

## 格式化

```bash
go-task format:all
```

## 快速参考

| 命令 | 说明 |
|------|------|
| `go-task build:bin` | 构建所有二进制文件 |
| `go-task build:docker` | 构建 Docker 镜像 |
| `go-task build:release` | 构建发布版本 |
| `go-task test:e2e` | 运行 E2E 测试 |
| `go-task gen:gen` | 生成代码 |
| `go-task format:all` | 格式化代码 |
| `go-task base:dev` | 启动开发环境（默认任务） |