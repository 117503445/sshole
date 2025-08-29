package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"sshole/internal/agent"
	"sshole/pkg/protocol"
)

func main() {
	var (
		hubAddr = flag.String("hub", "ws://localhost:8080", "Hub WebSocket address")
		token   = flag.String("token", "", "Authentication token")
		id      = flag.String("id", "", "Agent ID")
	)
	flag.Parse()

	// 创建连接信息
	connInfo := &protocol.ConnectionInfo{
		HubAddress: *hubAddr,
		Token:      *token,
		ClientID:   *id,
	}

	// 创建 Agent 实例
	agent := agent.NewAgent(connInfo)

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Agent
	go func() {
		if err := agent.Start(ctx); err != nil {
			log.Printf("Agent failed: %v", err)
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Println("Shutting down agent...")
	cancel()

	// 等待清理完成
	agent.Stop()
	log.Println("Agent stopped")
}
