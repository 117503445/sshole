package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sshole/internal/hub"
	"sshole/pkg/common"

	"github.com/rs/zerolog/log"
)

func main() {
	// 设置全局 context logger
	ctx := context.Background()
	ctx = common.InitLogger(ctx, common.InitLoggerOption{Component: "hub"})
	
	log.Ctx(ctx).Info().Msg("Starting hub application")

	var (
		addr    = flag.String("addr", ":8080", "HTTP server address")
		timeout = flag.Duration("timeout", 30*time.Second, "Read/write timeout")
	)
	flag.Parse()

	// 创建 Hub 实例
	h := hub.NewHub()

	// 创建 HTTP 服务器
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		h.HandleWebSocket(ctx, w, r)
	})
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
		logger := log.Ctx(ctx)
		logger.Info().Str("addr", *addr).Msg("Starting hub server")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("Server failed")
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	logger := log.Ctx(ctx)
	logger.Info().Msg("Shutting down hub...")

	// 优雅关闭服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Server shutdown failed")
	}

	// 停止 Hub
	h.Stop()
	logger.Info().Msg("Hub stopped")
}
