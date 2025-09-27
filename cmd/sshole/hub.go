package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	rpcv1 "sshole/pkg/rpc/v1"
	"sshole/pkg/rpc/v1/rpcv1connect"

	"connectrpc.com/connect"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	chserver "github.com/jpillora/chisel/server"
)

type HoleServer struct{}

func (s *HoleServer) CreateConn(
	ctx context.Context,
	req *connect.Request[rpcv1.CreateConnRequest],
) (*connect.Response[rpcv1.CreateConnResponse], error) {
	log.Info().Msg("CreateConn RPC")
	res := connect.NewResponse(&rpcv1.CreateConnResponse{
		Greeting: fmt.Sprintf("Hello, %s!", req.Msg.Name),
	})
	res.Header().Set("Greet-Version", "v1")
	return res, nil
}

func cmdHub(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")

	go func() {
		holeServer := &HoleServer{}
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

		path, handler := rpcv1connect.NewHoleServiceHandler(holeServer)
		mux.Handle(path, handler)

		err := http.ListenAndServe(
			"0.0.0.0:9001",
			// Use h2c so we can serve HTTP/2 without TLS.
			h2c.NewHandler(mux, &http2.Server{}),
		)
		if err != nil {
			logger.Error().Err(err).Send()
		}
	}()

	// 创建服务器配置
	config := chserver.Config{
		Reverse: true, // 启用反向模式
		Proxy:   "http://localhost:9001",
	}

	// 创建服务器实例
	s, err := chserver.NewServer(&config)
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create server")
	}

	// 设置服务器参数
	s.Debug = true // 启用调试模式

	// 启动服务器在指定端口
	if err := s.Start("", "9000"); err != nil {
		logger.Panic().Err(err).Msg("Failed to start server")
	}

	// 等待服务器关闭
	if err := s.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Server closed unexpectedly")
	}
}
