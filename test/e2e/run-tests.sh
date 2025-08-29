#!/bin/bash

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 is not installed or not in PATH"
        exit 1
    fi
}

# 编译测试二进制文件
build_test_binary() {
    log_info "Building e2e test binary..."

    if [ ! -f "go.mod" ]; then
        log_error "go.mod not found. Please run this script from the project root."
        exit 1
    fi

    # 编译 e2e 测试
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o test/e2e/e2e-test ./test/e2e

    if [ $? -eq 0 ]; then
        log_success "E2E test binary built successfully"
    else
        log_error "Failed to build e2e test binary"
        exit 1
    fi
}

# 使用 Docker 运行测试
run_with_docker() {
    log_info "Running e2e tests with Docker..."

    check_command "docker"

    # 切换到 e2e 目录
    cd test/e2e

    # 构建和运行
    docker-compose up --build --abort-on-container-exit --exit-code-from test-runner

    # 检查退出码
    if [ $? -eq 0 ]; then
        log_success "E2E tests passed with Docker"
    else
        log_error "E2E tests failed with Docker"
        exit 1
    fi

    # 清理
    docker-compose down -v
}

# 使用 Podman 运行测试
run_with_podman() {
    log_info "Running e2e tests with Podman..."

    check_command "podman"
    check_command "podman-compose"

    # 切换到 e2e 目录
    cd test/e2e

    # 构建和运行
    podman-compose up --build --abort-on-container-exit --exit-code-from test-runner

    # 检查退出码
    if [ $? -eq 0 ]; then
        log_success "E2E tests passed with Podman"
    else
        log_error "E2E tests failed with Podman"
        exit 1
    fi

    # 清理
    podman-compose down -v
}

# 本地运行测试（用于开发）
run_locally() {
    log_info "Running e2e tests locally..."

    # 构建二进制文件
    build_test_binary

    # 运行测试
    ./test/e2e/e2e-test

    if [ $? -eq 0 ]; then
        log_success "E2E tests passed locally"
    else
        log_error "E2E tests failed locally"
        exit 1
    fi
}

# 显示使用说明
show_usage() {
    echo "Usage: $0 [OPTION]"
    echo ""
    echo "Run e2e tests for sshole project"
    echo ""
    echo "Options:"
    echo "  docker     Run tests using Docker"
    echo "  podman     Run tests using Podman (default)"
    echo "  local      Run tests locally (for development)"
    echo "  build      Only build the test binary"
    echo "  clean      Clean up test artifacts"
    echo "  help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 podman    # Run tests with Podman"
    echo "  $0 docker    # Run tests with Docker"
    echo "  $0 local     # Run tests locally"
    echo "  $0 build     # Only build binary"
}

# 清理测试产物
cleanup() {
    log_info "Cleaning up test artifacts..."

    # 清理二进制文件
    if [ -f "test/e2e/e2e-test" ]; then
        rm test/e2e/e2e-test
        log_info "Removed test binary"
    fi

    # 清理 Docker 容器
    if command -v docker &> /dev/null; then
        cd test/e2e
        docker-compose down -v 2>/dev/null || true
        cd -
    fi

    # 清理 Podman 容器
    if command -v podman &> /dev/null; then
        cd test/e2e
        podman-compose down -v 2>/dev/null || true
        cd -
    fi

    log_success "Cleanup completed"
}

# 主函数
main() {
    case "${1:-podman}" in
        "docker")
            build_test_binary
            run_with_docker
            ;;
        "podman")
            build_test_binary
            run_with_podman
            ;;
        "local")
            run_locally
            ;;
        "build")
            build_test_binary
            ;;
        "clean")
            cleanup
            ;;
        "help"|"-h"|"--help")
            show_usage
            ;;
        *)
            log_error "Unknown option: $1"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"
