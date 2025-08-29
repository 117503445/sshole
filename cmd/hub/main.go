package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sshole/internal/hub"
)

func main() {
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
		log.Printf("Starting hub server on %s", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server failed: %v", err)
			cancel()
		}
	}()

	// 等待关闭信号
	<-sigChan
	log.Println("Shutting down hub...")

	// 优雅关闭服务器
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown failed: %v", err)
	}

	// 停止 Hub
	h.Stop()
	log.Println("Hub stopped")
}
