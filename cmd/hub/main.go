package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/117503445/goutils"
	"github.com/rs/zerolog/log"
	"sshole/internal/hub"
)

func main() {
	// 初始化 zerolog
	goutils.InitZeroLog()

	var (
		addr    = flag.String("addr", ":8080", "HTTP server address")
		timeout = flag.Duration("timeout", 30*time.Second, "Read/write timeout")
	)
	flag.Parse()

	// 创建 Hub 实例
	h := hub.NewHub()

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", h.HandleWebSocket)
	mux.HandleFunc("/health", h.HandleHealth)

	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  *timeout,
		WriteTimeout: *timeout,
	}

	// 设置优雅关闭
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动服务器
	go func() {
		log.Info().Str("addr", *addr).Msg("Starting hub server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("Server failed")
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Info().Msg("Shutting down hub...")

	// 优雅关闭服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("Server shutdown failed")
	}

	// 停止 Hub
	h.Stop()
	log.Info().Msg("Hub stopped")
}
