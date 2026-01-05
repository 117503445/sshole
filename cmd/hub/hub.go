package main

import (
	"context"
	"net/http"

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
