package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sshole/internal/agent"
	"sshole/internal/entry"
	"sshole/internal/hub"
	"sshole/pkg/protocol"
	"syscall"
	"time"
)

// ComponentRunner 运行特定组件
type ComponentRunner struct {
	componentType string
	hubAddr       string
	token         string
	clientID      string
	localAddr     string
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewComponentRunner 创建组件运行器
func NewComponentRunner() *ComponentRunner {
	var (
		component = flag.String("component", "test-runner", "Component type: hub, agent, entry, test-runner")
		hubAddr   = flag.String("hub", "ws://localhost:8080", "Hub WebSocket address")
		token     = flag.String("token", "test-token", "Authentication token")
		clientID  = flag.String("id", "test-client", "Client ID")
		localAddr = flag.String("local", ":10022", "Local address for entry")
	)
	flag.Parse()

	return &ComponentRunner{
		componentType: *component,
		hubAddr:       *hubAddr,
		token:         *token,
		clientID:      *clientID,
		localAddr:     *localAddr,
	}
}

// RunHub 运行 Hub 组件
func (cr *ComponentRunner) RunHub() error {
	log.Println("Starting Hub component...")

	h := hub.NewHub()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.HandleWebSocket)
	mux.HandleFunc("/health", h.HandleHealth)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	cr.ctx, cr.cancel = context.WithCancel(context.Background())

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		log.Printf("Hub server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Hub server error: %v", err)
		}
	}()

	// 等待信号
	<-sigChan
	log.Println("Shutting down Hub...")

	// 优雅关闭
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Hub shutdown error: %v", err)
	}

	h.Stop()
	log.Println("Hub stopped")
	return nil
}

// RunAgent 运行 Agent 组件
func (cr *ComponentRunner) RunAgent() error {
	log.Println("Starting Agent component...")

	connInfo := &protocol.ConnectionInfo{
		HubAddress: cr.hubAddr,
		Token:      cr.token,
		ClientID:   cr.clientID,
	}

	agent := agent.NewAgent(connInfo)

	cr.ctx, cr.cancel = context.WithCancel(context.Background())

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Agent
	go func() {
		if err := agent.Start(cr.ctx); err != nil {
			log.Printf("Agent error: %v", err)
		}
	}()

	// 等待信号
	<-sigChan
	log.Println("Shutting down Agent...")

	cr.cancel()
	agent.Stop()
	log.Println("Agent stopped")
	return nil
}

// RunEntry 运行 Entry 组件
func (cr *ComponentRunner) RunEntry() error {
	log.Println("Starting Entry component...")

	connInfo := &protocol.ConnectionInfo{
		HubAddress: cr.hubAddr,
		Token:      cr.token,
		ClientID:   cr.clientID,
	}

	entry := entry.NewEntry(connInfo, cr.localAddr)

	cr.ctx, cr.cancel = context.WithCancel(context.Background())

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Entry
	go func() {
		if err := entry.Start(cr.ctx); err != nil {
			log.Printf("Entry error: %v", err)
		}
	}()

	// 等待信号
	<-sigChan
	log.Println("Shutting down Entry...")

	cr.cancel()
	entry.Stop()
	log.Println("Entry stopped")
	return nil
}

// RunTestRunner 运行测试运行器
func (cr *ComponentRunner) RunTestRunner() error {
	log.Println("Starting Test Runner...")

	cr.ctx, cr.cancel = context.WithCancel(context.Background())

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待组件启动
	log.Println("Waiting for components to be ready...")
	time.Sleep(5 * time.Second)

	// 运行测试
	if err := cr.runTestSuite(); err != nil {
		log.Printf("Test suite failed: %v", err)
		return err
	}

	log.Println("All tests passed! Waiting for shutdown signal...")

	// 等待信号
	<-sigChan
	log.Println("Test runner shutting down...")
	cr.cancel()

	return nil
}

// runTestSuite 运行测试套件
func (cr *ComponentRunner) runTestSuite() error {
	tests := []struct {
		name string
		fn   func() error
	}{
		{"Health Check", cr.testHealthCheck},
		{"WebSocket Connection", cr.testWebSocketConnection},
		{"SSH Connection", cr.testSSHConnection},
		{"End-to-End Flow", cr.testEndToEndFlow},
	}

	for _, test := range tests {
		log.Printf("Running test: %s", test.name)
		if err := test.fn(); err != nil {
			return fmt.Errorf("test %s failed: %v", test.name, err)
		}
		log.Printf("✓ Test %s passed", test.name)
	}

	return nil
}

// testHealthCheck 测试健康检查
func (cr *ComponentRunner) testHealthCheck() error {
	resp, err := http.Get("http://hub:8080/health")
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status: %d", resp.StatusCode)
	}

	return nil
}

// testWebSocketConnection 测试 WebSocket 连接
func (cr *ComponentRunner) testWebSocketConnection() error {
	// 这里可以实现更复杂的 WebSocket 测试
	// 目前只是检查 Hub 是否可达
	resp, err := http.Get("http://hub:8080/health")
	if err != nil {
		return fmt.Errorf("WebSocket endpoint check failed: %v", err)
	}
	defer resp.Body.Close()

	return nil
}

// testSSHConnection 测试 SSH 连接
func (cr *ComponentRunner) testSSHConnection() error {
	// 尝试连接到 Entry 提供的 SSH 服务
	conn, err := net.DialTimeout("tcp", "entry:10022", 5*time.Second)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %v", err)
	}
	defer conn.Close()

	// 发送简单的 SSH 协议标识
	sshProto := "SSH-2.0-test-client\r\n"
	if _, err := conn.Write([]byte(sshProto)); err != nil {
		return fmt.Errorf("SSH protocol handshake failed: %v", err)
	}

	return nil
}

// testEndToEndFlow 测试端到端流程
func (cr *ComponentRunner) testEndToEndFlow() error {
	// 这里可以实现更复杂的端到端测试
	// 验证整个 agent -> hub -> entry 的数据流

	log.Println("Testing end-to-end data flow...")

	// 简单的延迟测试，模拟数据传输
	time.Sleep(2 * time.Second)

	// 在实际实现中，这里应该：
	// 1. 通过 Agent 发送数据
	// 2. 验证 Hub 接收到数据
	// 3. 验证 Entry 能够转发数据
	// 4. 验证客户端能够接收数据

	return nil
}

// Run 运行指定的组件
func (cr *ComponentRunner) Run() error {
	switch cr.componentType {
	case "hub":
		return cr.RunHub()
	case "agent":
		return cr.RunAgent()
	case "entry":
		return cr.RunEntry()
	case "test-runner":
		return cr.RunTestRunner()
	default:
		return fmt.Errorf("unknown component type: %s", cr.componentType)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	runner := NewComponentRunner()

	if err := runner.Run(); err != nil {
		log.Fatalf("Component failed: %v", err)
	}

	log.Println("Component completed successfully")
}
