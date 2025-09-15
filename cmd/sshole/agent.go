package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sshole/pkg/utils"
	"time"

	"github.com/117503445/goutils"
	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

func isPortListening(port int) bool {
	address := fmt.Sprintf(":%d", port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false // 无法连接 = 未监听
	}
	conn.Close()
	return true // 能连接 = 正在监听
}
func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	sshdPort := 22222

	startSSHD := func() {
		if isPortListening(sshdPort) {
			logger.Info().Msg("sshd is already listening")
			return
		}

		if err := goutils.WriteText("/opt/openssh/etc/sshd_config", fmt.Sprintf(`Port %v
Subsystem	sftp	/opt/openssh/libexec/sftp-server`, sshdPort)); err != nil {
			logger.Panic().Err(err).Msg("write sshd_config failed")
		}

		fileSSHTarGz := "/tmp/openssh.tar.gz"

		if !goutils.FileExists(fileSSHTarGz) {
			logger.Info().Msg("Downloading openssh")

			if err := os.MkdirAll("/tmp", 0755); err != nil {
				logger.Panic().Err(err).Msg("create tmp dir failed")
			}
			if err := goutils.Download("https://webdav.cloud.117503445.top/public-writable/openssh-V_9_9_P2.tgz", fileSSHTarGz); err != nil {
				logger.Panic().Err(err).Msg("download openssh failed")
			}
			opensshTarGz, err := os.ReadFile(fileSSHTarGz)
			if err != nil {
				logger.Panic().Err(err).Msg("read openssh.tar.gz failed")
			}

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
		}else{
			logger.Info().Msg("Using cached openssh")
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
	}

	startSSHD()

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.Agent.HubServer,
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
