package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog/log"
	"sshole/internal/agent"
	"sshole/pkg/protocol"
)

func main() {
	// 初始化 zerolog
	goutils.InitZeroLog()

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
			log.Error().Err(err).Msg("Agent failed")
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Info().Msg("Shutting down agent...")
	cancel()

	// 等待清理完成
	agent.Stop()
	log.Info().Msg("Agent stopped")
}
