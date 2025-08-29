package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ANSI 颜色代码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[0;31m"
	ColorGreen  = "\033[0;32m"
	ColorYellow = "\033[1;33m"
	ColorBlue   = "\033[0;34m"
)

// Logger 结构体用于彩色日志输出
type Logger struct{}

// Info 记录信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	fmt.Printf(ColorBlue+"[INFO]"+ColorReset+" "+format+"\n", args...)
}

// Warn 记录警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	fmt.Printf(ColorYellow+"[WARN]"+ColorReset+" "+format+"\n", args...)
}

// Error 记录错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	fmt.Printf(ColorRed+"[ERROR]"+ColorReset+" "+format+"\n", args...)
}

// Success 记录成功日志
func (l *Logger) Success(format string, args ...interface{}) {
	fmt.Printf(ColorGreen+"[SUCCESS]"+ColorReset+" "+format+"\n", args...)
}

// 全局日志实例
var logger = &Logger{}

// Runner 运行器结构体
type Runner struct {
	projectRoot string
	e2eDir      string
}

// NewRunner 创建新的运行器
func NewRunner() (*Runner, error) {
	// 获取项目根目录
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %v", err)
	}

	// 检查是否存在 go.mod 文件来确认项目根目录
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			break
		}

		parent := filepath.Dir(wd)
		if parent == wd {
			return nil, fmt.Errorf("go.mod not found. Please run this from the project root")
		}
		wd = parent
	}

	e2eDir := filepath.Join(wd, "test", "e2e")

	return &Runner{
		projectRoot: wd,
		e2eDir:      e2eDir,
	}, nil
}

// checkCommand 检查命令是否存在
func (r *Runner) checkCommand(command string) error {
	_, err := exec.LookPath(command)
	if err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command)
	}
	return nil
}

// runCommand 执行命令
func (r *Runner) runCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Running: %s %s", name, strings.Join(args, " "))
	return cmd.Run()
}

// buildTestBinary 编译测试二进制文件
func (r *Runner) buildTestBinary() error {
	logger.Info("Building e2e test binary...")

	// 检查 go.mod 文件
	goModPath := filepath.Join(r.projectRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found. Please run this from the project root")
	}

	// 编译 e2e 测试
	outputPath := filepath.Join(r.e2eDir, "e2e-test")
	sourcePath := filepath.Join(r.e2eDir, ".")

	env := []string{
		"CGO_ENABLED=0",
		fmt.Sprintf("GOOS=%s", runtime.GOOS),
		"GOARCH=amd64",
	}

	cmd := exec.Command("go", "build", "-a", "-installsuffix", "cgo", "-o", outputPath, sourcePath)
	cmd.Dir = r.projectRoot
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build e2e test binary: %v", err)
	}

	logger.Success("E2E test binary built successfully")
	return nil
}

// runWithDocker 使用 Docker 运行测试
func (r *Runner) runWithDocker() error {
	logger.Info("Running e2e tests with Docker...")

	if err := r.checkCommand("docker"); err != nil {
		return err
	}

	// 构建和运行
	err := r.runCommand(r.e2eDir, "docker-compose", "up", "--build", "--abort-on-container-exit", "--exit-code-from", "test-runner")
	if err != nil {
		logger.Error("E2E tests failed with Docker")
		return err
	}

	logger.Success("E2E tests passed with Docker")

	// 清理
	r.runCommand(r.e2eDir, "docker-compose", "down", "-v")

	return nil
}

// runWithPodman 使用 Podman 运行测试
func (r *Runner) runWithPodman() error {
	logger.Info("Running e2e tests with Podman...")

	if err := r.checkCommand("podman"); err != nil {
		return err
	}

	if err := r.checkCommand("podman-compose"); err != nil {
		return err
	}

	// 构建和运行
	err := r.runCommand(r.e2eDir, "podman-compose", "up", "--build", "--abort-on-container-exit", "--exit-code-from", "test-runner")
	if err != nil {
		logger.Error("E2E tests failed with Podman")
		return err
	}

	logger.Success("E2E tests passed with Podman")

	// 清理
	r.runCommand(r.e2eDir, "podman-compose", "down", "-v")

	return nil
}

// runLocally 本地运行测试
func (r *Runner) runLocally() error {
	logger.Info("Running e2e tests locally...")

	// 构建二进制文件
	if err := r.buildTestBinary(); err != nil {
		return err
	}

	logger.Info("Starting local Hub service...")

	// 构建 Hub 二进制文件
	hubBinary := filepath.Join(r.projectRoot, "hub-binary")
	buildHubCmd := exec.Command("go", "build", "-o", hubBinary, "./cmd/hub")
	buildHubCmd.Dir = r.projectRoot
	if err := buildHubCmd.Run(); err != nil {
		return fmt.Errorf("failed to build Hub binary: %v", err)
	}

	// 启动本地 Hub 服务（在后台）
	hubCmd := exec.Command(hubBinary)
	hubCmd.Dir = r.projectRoot
	hubCmd.Stdout = os.Stdout
	hubCmd.Stderr = os.Stderr

	if err := hubCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Hub service: %v", err)
	}

	// 等待 Hub 服务启动
	logger.Info("Waiting for Hub service to start...")
	time.Sleep(2 * time.Second)

	// 构建真正的 Entry 二进制文件
	entryBinary := filepath.Join(r.projectRoot, "entry-binary")
	buildEntryCmd := exec.Command("go", "build", "-o", entryBinary, "./cmd/entry")
	buildEntryCmd.Dir = r.projectRoot
	if err := buildEntryCmd.Run(); err != nil {
		return fmt.Errorf("failed to build Entry binary: %v", err)
	}

	// 启动本地 Entry 服务（在后台）
	logger.Info("Starting local Entry service...")
	entryCmd := exec.Command(entryBinary, "-hub", "ws://localhost:8080", "-token", "test-token", "-conn-id", "test-entry", "-local", "localhost:10022")
	entryCmd.Dir = r.projectRoot
	entryCmd.Stdout = os.Stdout
	entryCmd.Stderr = os.Stderr

	if err := entryCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Entry service: %v", err)
	}

	// 等待 Entry 服务启动
	logger.Info("Waiting for Entry service to start...")
	time.Sleep(3 * time.Second)

	// 检查端口是否在监听
	logger.Info("Checking if port 10022 is listening...")
	checkPortCmd := exec.Command("netstat", "-tlnp", "|", "grep", ":10022")
	checkPortCmd.Stdout = os.Stdout
	checkPortCmd.Stderr = os.Stderr
	checkPortCmd.Run() // 不检查错误，因为 netstat 可能不可用

	// 启动本地 Agent 服务（在后台）
	logger.Info("Starting local Agent service...")
	agentCmd := exec.Command(filepath.Join(r.e2eDir, "e2e-test"))
	agentCmd.Dir = r.projectRoot
	agentCmd.Env = append(os.Environ(), "COMPONENT_TYPE=agent", "HUB_ADDRESS=ws://localhost:8080", "AGENT_TOKEN=test-token", "AGENT_ID=test-agent")
	agentCmd.Stdout = os.Stdout
	agentCmd.Stderr = os.Stderr

	if err := agentCmd.Start(); err != nil {
		return fmt.Errorf("failed to start Agent service: %v", err)
	}

	// 等待所有服务启动
	logger.Info("Waiting for all services to be ready...")
	time.Sleep(5 * time.Second)

	// 运行测试
	logger.Info("Running e2e test suite...")
	testBinary := filepath.Join(r.e2eDir, "e2e-test")
	testCmd := exec.Command(testBinary)
	testCmd.Dir = r.projectRoot
	testCmd.Env = append(os.Environ(), "COMPONENT_TYPE=test-runner")
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr

	err := testCmd.Run()
	if err != nil {
		logger.Error("E2E tests failed locally")

		// 清理后台进程
		hubCmd.Process.Kill()
		entryCmd.Process.Kill()
		agentCmd.Process.Kill()

		// 清理临时文件
		os.Remove(hubBinary)
		os.Remove(entryBinary)

		return err
	}

	logger.Success("E2E tests passed locally")

	// 清理后台进程
	hubCmd.Process.Kill()
	entryCmd.Process.Kill()
	agentCmd.Process.Kill()

	// 清理临时文件
	os.Remove(hubBinary)
	os.Remove(entryBinary)

	return nil
}

// cleanup 清理测试产物
func (r *Runner) cleanup() error {
	logger.Info("Cleaning up test artifacts...")

	// 清理二进制文件
	testBinary := filepath.Join(r.e2eDir, "e2e-test")
	if _, err := os.Stat(testBinary); err == nil {
		if err := os.Remove(testBinary); err != nil {
			logger.Warn("Failed to remove test binary: %v", err)
		} else {
			logger.Info("Removed test binary")
		}
	}

	// 清理 Docker 容器
	if r.checkCommand("docker") == nil {
		r.runCommand(r.e2eDir, "docker-compose", "down", "-v")
	}

	// 清理 Podman 容器
	if r.checkCommand("podman") == nil {
		r.runCommand(r.e2eDir, "podman-compose", "down", "-v")
	}

	logger.Success("Cleanup completed")
	return nil
}

// showUsage 显示使用说明
func showUsage() {
	fmt.Println("Usage: go run test/e2e/tools/runner.go [OPTION]")
	fmt.Println("")
	fmt.Println("Run e2e tests for sshole project")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  docker     Run tests using Docker")
	fmt.Println("  podman     Run tests using Podman (default)")
	fmt.Println("  local      Run tests locally (for development)")
	fmt.Println("  build      Only build the test binary")
	fmt.Println("  clean      Clean up test artifacts")
	fmt.Println("  help       Show this help message")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  go run test/e2e/tools/runner.go podman    # Run tests with Podman")
	fmt.Println("  go run test/e2e/tools/runner.go docker    # Run tests with Docker")
	fmt.Println("  go run test/e2e/tools/runner.go local     # Run tests locally")
	fmt.Println("  go run test/e2e/tools/runner.go build     # Only build binary")
}

// main 主函数
func main() {
	// 解析命令行参数
	var command string
	if len(os.Args) > 1 {
		command = os.Args[1]
	} else {
		command = "podman" // 默认使用 podman
	}

	// 创建运行器
	runner, err := NewRunner()
	if err != nil {
		logger.Error("%v", err)
		os.Exit(1)
	}

	// 执行相应命令
	switch command {
	case "docker":
		if err := runner.buildTestBinary(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		if err := runner.runWithDocker(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "podman":
		if err := runner.buildTestBinary(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
		if err := runner.runWithPodman(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "local":
		if err := runner.runLocally(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "build":
		if err := runner.buildTestBinary(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "clean":
		if err := runner.cleanup(); err != nil {
			logger.Error("%v", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		showUsage()
	default:
		logger.Error("Unknown option: %s", command)
		fmt.Println("")
		showUsage()
		os.Exit(1)
	}
}
