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
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"sshole/pkg/utils"

	"github.com/117503445/goutils"
	chclient "github.com/jpillora/chisel/client"
	"github.com/rs/zerolog/log"
)

// terminateProcess 安全终止进程
func terminateProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return // 进程未启动或已结束
	}

	// 尝试优雅终止
	if err := cmd.Process.Signal(syscall.SIGTERM); err == nil {
		// 等待几秒让进程优雅退出
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait() // 避免阻塞
		}()
		select {
		case <-done:
			fmt.Println("Process exited gracefully.")
		case <-time.After(3 * time.Second):
			// 超时，强制 kill
			fmt.Println("Graceful termination failed, force killing...")
			_ = cmd.Process.Kill()
			_ = cmd.Wait() // 等待回收资源，避免僵尸进程
		}
	} else {
		// 无法发送 SIGTERM，直接强制 kill
		fmt.Println("Cannot send SIGTERM, force killing...")
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func isPortListening(port int) bool {
	address := fmt.Sprintf(":%d", port)
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false // 无法连接 = 未监听
	}
	conn.Close()
	return true // 能连接 = 正在监听
}

func setupSSHKeys(ctx context.Context, connId string) int32 {
	logger := log.Ctx(ctx)

	logger.Info().Str("connId", connId).Msg("Setting up SSH keys")

	// 返回默认端口，不再调用AcquireConnection RPC
	return 22222
}

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	// 从环境变量获取连接ID
	connId := os.Getenv("CONN_ID")
	var port int32
	if connId != "" {
		logger.Info().Str("connId", connId).Msg("Setting up SSH keys with conn ID")
		port = setupSSHKeys(ctx, connId)
	} else {
		logger.Warn().Msg("CONN_ID not found in environment variables")
	}

	sshdPort := 22222

	var sshCmd *exec.Cmd

	startSSHD := func() {
		if isPortListening(sshdPort) {
			logger.Info().Msg("sshd is already listening")
			return
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
						if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
							panic("无法创建目录: " + err.Error())
						}
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
		} else {
			logger.Info().Msg("Using cached openssh")
		}

		if err := goutils.WriteText("/opt/openssh/etc/sshd_config", fmt.Sprintf(`Port %v
PermitRootLogin yes
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile /root/.ssh/authorized_keys
Subsystem	sftp	/opt/openssh/libexec/sftp-server`, sshdPort)); err != nil {
			logger.Panic().Err(err).Msg("write sshd_config failed")
		}

		utils.Execute(ctx, utils.ExecuteParams{
			Cmd: utils.Command("/opt/openssh/bin/ssh-keygen -A"),
		})

		sshCmd = utils.Command("/opt/openssh/sbin/sshd -D -e")
		go func() {
			utils.Execute(ctx, utils.ExecuteParams{
				Cmd: sshCmd,
			})
		}()

		for {
			time.Sleep(time.Second) // 等待 sshd 启动
			if isPortListening(sshdPort) {
				logger.Info().Msg("sshd is already listening")
				return
			}
		}
	}

	startSSHD()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic: %v\n", r)
			// 终止子进程
			terminateProcess(sshCmd)
		} else {
			// 正常退出时也清理（可选）
			terminateProcess(sshCmd)
		}
	}()

	logger.Info().
		Int("HubPort", int(port)).
		Int("AgentPort", sshdPort).
		Msg("Starting chisel")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.Agent.HubServer,
		Remotes: []string{fmt.Sprintf("R:%d:localhost:%v", port, sshdPort)}, // 本地 22222 端口，映射到 hub 的指定端口
		Auth:    cli.Agent.Auth,
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
