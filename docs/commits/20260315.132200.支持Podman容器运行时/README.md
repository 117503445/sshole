# 支持 Podman 容器运行时

## 主要内容和目的

为构建脚本和 E2E 测试添加容器运行时自动检测功能，支持 Podman 和 Docker 两种运行时，优先使用 Podman。

## 更改内容描述

### 1. 构建脚本 (`scripts/script/build-docker.go`)
- 新增 `detectContainerRuntime()` 函数，自动检测系统可用的容器运行时
- 优先检测 Podman，回退到 Docker
- 将所有硬编码的 `docker` 命令替换为动态运行时变量
- 更新日志信息，将 "docker" 改为 "container"

### 2. E2E 测试 (`scripts/script/pkg/e2e/`)
- `utils.go`: 添加容器运行时检测函数
- `auth.go`: 使用动态运行时执行认证测试
- `basic.go`: 使用动态运行时执行基础测试

### 3. 开发镜像 (`dev.Dockerfile`)
- 简化为基础镜像引用 `FROM 117503445/dev-desktop`

## 验证方法和结果

- 构建脚本可自动检测并使用可用的容器运行时
- E2E 测试支持在 Podman 或 Docker 环境下运行
- 无需修改命令行调用方式，自动适配系统环境