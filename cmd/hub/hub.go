package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/tunnel"
)

// tunnelManager is the global TunnelManager instance for the hub
var tunnelManager *tunnel.TunnelManager

func cmdHub(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")

	// Create TunnelManager with auth
	tunnelManager = tunnel.NewTunnelManager(cli.Auth)
	defer tunnelManager.Close()

	logger.Info().
		Str("tunnel_manager_id", tunnelManager.ID).
		Msg("TunnelManager created")

	mux := http.NewServeMux()

	// 添加 /bin 路由处理器，用于返回二进制文件
	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) {
		// 获取当前执行的二进制文件路径
		execPath, err := os.Executable()
		if err != nil {
			http.Error(w, "Failed to get executable path", http.StatusInternalServerError)
			return
		}

		// 打开二进制文件
		file, err := os.Open(execPath)
		if err != nil {
			http.Error(w, "Failed to open executable", http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// 获取文件信息
		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to get file info", http.StatusInternalServerError)
			return
		}

		// 设置响应头
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=sshole")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// 将文件内容复制到响应中
		_, err = io.Copy(w, file)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to copy file to response")
			return
		}
	})

	// 添加 /healthz 路由处理器，用于健康检查
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// 添加 WebSocket tunnel 路由处理器
	mux.HandleFunc("/tunnel", tunnelManager.Handler)

	// RPC 服务
	holeServer := newHoleServer()
	interceptors := connect.WithInterceptors(
		NewCtxInterceptor(),
	)
	path, handler := rpcv1connect.NewHoleServiceHandler(holeServer, interceptors)
	mux.Handle(path, handler)

	logger.Info().Msg("Starting HTTP server on :9000")

	// 启动 HTTP 服务器
	err := http.ListenAndServe(
		"0.0.0.0:9000",
		// Use h2c so we can serve HTTP/2 without TLS.
		h2c.NewHandler(mux, &http2.Server{}),
	)
	if err != nil {
		logger.Panic().Err(err).Msg("Server closed unexpectedly")
	}
}
