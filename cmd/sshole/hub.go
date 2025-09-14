package main

import (
	"context"

	"github.com/rs/zerolog/log"

	chserver "github.com/jpillora/chisel/server"
)

func cmdHub(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting hub")
	// 创建服务器配置
	config := chserver.Config{
		Reverse: true, // 启用反向模式
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
