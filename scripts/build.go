package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// BuildConfig 构建配置
type BuildConfig struct {
	outputDir string
	verbose   bool
	clean     bool
	targetOS  string
	targetArch string
}

// Component 组件信息
type Component struct {
	name    string
	path    string
	binary  string
	desc    string
	usage   string
}

// Logger 简单日志记录器
type Logger struct {
	verbose bool
}

// Info 记录信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	fmt.Printf("ℹ️  "+format+"\n", args...)
}

// Verbose 记录详细日志
func (l *Logger) Verbose(format string, args ...interface{}) {
	if l.verbose {
		fmt.Printf("🔍 "+format+"\n", args...)
	}
}

// Success 记录成功日志
func (l *Logger) Success(format string, args ...interface{}) {
	fmt.Printf("✅ "+format+"\n", args...)
}

// Error 记录错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	fmt.Printf("❌ "+format+"\n", args...)
}

// NewLogger 创建日志记录器
func NewLogger(verbose bool) *Logger {
	return &Logger{verbose: verbose}
}

// getProjectRoot 获取项目根目录
func getProjectRoot() (string, error) {
	// 假设脚本从项目根目录运行
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}

	// 验证当前目录是否有 go.mod 文件
	if _, err := os.Stat(filepath.Join(wd, "go.mod")); os.IsNotExist(err) {
		return "", fmt.Errorf("go.mod not found in current directory. Please run from project root")
	}

	return wd, nil
}

// NewBuildConfig 创建构建配置
func NewBuildConfig() *BuildConfig {
	config := &BuildConfig{}

	flag.StringVar(&config.outputDir, "output", "bin", "Output directory for binaries")
	flag.BoolVar(&config.verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&config.clean, "clean", false, "Clean output directory before building")
	flag.StringVar(&config.targetOS, "os", runtime.GOOS, "Target operating system")
	flag.StringVar(&config.targetArch, "arch", runtime.GOARCH, "Target architecture")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: go run scripts/build.go [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Build sshole project binaries\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nExamples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  go run scripts/build.go                 # Build for current platform\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  go run scripts/build.go -verbose        # Build with verbose output\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  go run scripts/build.go -clean          # Clean before building\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  go run scripts/build.go -os linux -arch amd64  # Cross-compile\n")
	}

	return config
}

// getComponents 获取要构建的组件列表
func getComponents() []Component {
	return []Component{
		{
			name:   "Agent",
			path:   "./cmd/agent",
			binary: "agent",
			desc:   "SSH Agent component",
			usage:  "./bin/agent -hub 'ws://localhost:8080'",
		},
		{
			name:   "Hub",
			path:   "./cmd/hub",
			binary: "hub",
			desc:   "SSH Hub server component",
			usage:  "./bin/hub -addr ':8080'",
		},
		{
			name:   "Entry",
			path:   "./cmd/entry",
			binary: "entry",
			desc:   "SSH Entry component",
			usage:  "./bin/entry -hub 'ws://localhost:8080' -local ':10022'",
		},
	}
}

// createOutputDir 创建输出目录
func (b *BuildConfig) createOutputDir(ctx context.Context, logger *zerolog.Logger) error {
	logger.Debug().Str("output_dir", b.outputDir).Msg("Creating output directory")

	if b.clean {
		logger.Info().Msg("Cleaning output directory...")
		if err := os.RemoveAll(b.outputDir); err != nil {
			logger.Warn().Err(err).Msg("Warning: failed to clean directory")
		}
	}

	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	return nil
}

// buildComponent 构建单个组件
func (b *BuildConfig) buildComponent(ctx context.Context, component Component, logger *zerolog.Logger) error {
	logger.Info().Str("component", component.name).Str("desc", component.desc).Msg("Building component")

	outputPath := filepath.Join(b.outputDir, component.binary)

	// 构建参数
	args := []string{
		"build",
		"-o", outputPath,
	}

	// 如果是交叉编译，添加更多参数
	if b.targetOS != runtime.GOOS || b.targetArch != runtime.GOARCH {
		args = append(args, fmt.Sprintf("-target=%s/%s", b.targetOS, b.targetArch))
		logger.Debug().Str("target_os", b.targetOS).Str("target_arch", b.targetArch).Msg("Cross-compiling")
	}

	args = append(args, component.path)

	logger.Debug().Str("command", "go "+strings.Join(args, " ")).Msg("Running build command")

	// 执行构建
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build %s: %v", component.name, err)
	}

	logger.Info().Str("component", component.name).Msg("Built successfully")
	logger.Debug().Str("output_path", outputPath).Msg("Output location")

	return nil
}

// showBuildResults 显示构建结果
func (b *BuildConfig) showBuildResults(ctx context.Context, components []Component, logger *zerolog.Logger) error {
	logger.Info().Msg("Build complete!")

	logger.Info().Msg("Generated binaries:")
	fmt.Println("📋 Generated binaries:")

	// 列出所有生成的文件
	files, err := os.ReadDir(b.outputDir)
	if err != nil {
		return fmt.Errorf("failed to read output directory: %v", err)
	}

	for _, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				continue
			}
			fmt.Printf("  %s (%d bytes)\n", file.Name(), info.Size())
		}
	}

	fmt.Println("")
	fmt.Println("🚀 Usage:")

	// 显示使用说明
	for _, component := range components {
		fmt.Printf("  %s  # %s\n", component.usage, component.desc)
	}

	return nil
}

// Run 执行构建
func (b *BuildConfig) Run(ctx context.Context) error {
	logger := log.Ctx(ctx)

	logger.Info().Msg("Building sshole project...")

	// 获取项目根目录
	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("failed to get project root: %v", err)
	}
	logger.Debug().Str("project_root", projectRoot).Msg("Project root")

	// 创建输出目录
	if err := b.createOutputDir(ctx, logger); err != nil {
		return err
	}

	// 获取组件列表
	components := getComponents()

	// 构建所有组件
	for _, component := range components {
		if err := b.buildComponent(ctx, component, logger); err != nil {
			logger.Error().Str("component", component.name).Err(err).Msg("Build failed")
			return err
		}
	}

	// 显示构建结果
	return b.showBuildResults(ctx, components, logger)
}

func main() {
	// 初始化 zerolog
	goutils.InitZeroLog()

	// 设置全局 context logger
	ctx := context.Background()
	ctx = log.Logger.WithContext(ctx)
	log.Ctx(ctx).Info().Msg("Starting build application")

	config := NewBuildConfig()
	flag.Parse()

	if err := config.Run(ctx); err != nil {
		logger := log.Ctx(ctx)
		logger.Fatal().Err(err).Msg("Build failed")
	}
}
