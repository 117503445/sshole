package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"sshole/pkg/utils"
	"time"

	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

//go:embed openssh-V_9_9_P2.tgz
var opensshTarGz []byte

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	logger.Info().Msg("Extracting openssh")
	{
		// 创建 gzip 读取器
		gz, err := gzip.NewReader(bytes.NewReader(opensshTarGz))
		if err != nil {
			panic("无法读取 gzip 数据: " + err.Error())
		}
		defer gz.Close()

		// 创建 tar 读取器
		tr := tar.NewReader(gz)

		// 遍历 tar 中的每个文件
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break // 解压完成
			}
			if err != nil {
				panic("读取 tar 头部失败: " + err.Error())
			}

			// 构建目标文件的完整路径（直接拼接到根目录）
			target := filepath.Join("/", header.Name)

			// 根据文件类型处理
			switch header.Typeflag {
			case tar.TypeDir:
				// 创建目录
				os.MkdirAll(target, os.FileMode(header.Mode))
				logger.Info().Str("dir", target).Msg("create dir")
			case tar.TypeReg:
				// 创建文件
				file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
				if err != nil {
					panic("创建文件失败 " + target + ": " + err.Error())
				}
				// 将 tar 中的文件内容拷贝到新文件
				io.Copy(file, tr)
				file.Close()
				logger.Info().Str("file", target).Msg("create file")
			}
		}
	}

	utils.Execute(ctx, utils.ExecuteParams{
		Cmd: utils.Command("/opt/openssh/bin/ssh-keygen -A"),
	})

	go func() {
		utils.Execute(ctx, utils.ExecuteParams{
			Cmd: utils.Command("/opt/openssh/sbin/sshd -D -e"),
		})
	}()

	time.Sleep(time.Second) // 等待 sshd 启动

	c, err := chclient.NewClient(&chclient.Config{
		Server:  "localhost:9000",
		Remotes: []string{"R:23:localhost:22"}, // 本地 22 端口，映射到 hub 的 23 端口
	})
	if err != nil {
		logger.Panic().Err(err).Msg("Failed to create chisel client")
	}
	if err := c.Start(ctx); err != nil {
		logger.Panic().Err(err).Msg("Failed to start chisel client")
	}
	if err := c.Wait(); err != nil {
		logger.Panic().Err(err).Msg("Failed to wait chisel client")
	}

}
