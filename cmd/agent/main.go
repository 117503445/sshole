package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"sshole/internal/agent"
	"sshole/pkg/common"
	"sshole/pkg/protocol"

	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{Component: "agent"})

	log.Ctx(ctx).Info().Msg("Starting agent application")

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
		logger := log.Ctx(ctx)
		if err := agent.Start(ctx); err != nil {
			logger.Error().Err(err).Msg("Agent failed")
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	logger := log.Ctx(ctx)
	logger.Info().Msg("Shutting down agent...")
	cancel()

	// 等待清理完成
	agent.Stop()
	logger.Info().Msg("Agent stopped")
}
