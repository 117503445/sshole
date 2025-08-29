package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog/log"
	"sshole/internal/entry"
	"sshole/pkg/protocol"
)

func main() {
	// 初始化 zerolog
	goutils.InitZeroLog()

	var (
		hubAddr   = flag.String("hub", "ws://localhost:8080", "Hub WebSocket address")
		token     = flag.String("token", "", "Authentication token")
		connID    = flag.String("conn-id", "", "Connection ID")
		localAddr = flag.String("local", ":10022", "Local SSH server address")
	)
	flag.Parse()

	// 创建连接信息
	connInfo := &protocol.ConnectionInfo{
		HubAddress: *hubAddr,
		Token:      *token,
		ClientID:   *connID,
	}

	// 创建 Entry 实例
	entry := entry.NewEntry(connInfo, *localAddr)

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动 Entry
	go func() {
		if err := entry.Start(ctx); err != nil {
			log.Error().Err(err).Msg("Entry failed")
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Info().Msg("Shutting down entry...")
	cancel()

	// 等待清理完成
	entry.Stop()
	log.Info().Msg("Entry stopped")
}
