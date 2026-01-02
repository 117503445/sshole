package main

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"connectrpc.com/connect"
	rpcv1 "github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/117503445/sshole/pkg/utils"

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

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	var port int32
	rpcClient := rpcv1connect.NewHoleServiceClient(http.DefaultClient, cli.HubServer)

	response, err := rpcClient.AgentCreate(ctx, connect.NewRequest(&rpcv1.ApiRequest{
		Auth: cli.Auth,
		Payload: &rpcv1.ApiRequest_AgentCreate{
			AgentCreate: &rpcv1.AgentCreateRequest{
				Name: cli.Name,
			},
		},
	}))
	if err != nil {
		logger.Panic().Err(err).Msg("failed to create agent")
	}
	if response.Msg.Code != 0 {
		logger.Panic().Int64("code", response.Msg.Code).
			Str("message", response.Msg.Message).
			Str("agent_name", cli.Name).
			Msg("failed to create agent via hub")
	}
	port = int32(response.Msg.GetAgentCreate().GetAgent().GetPort())
	if port == 0 {
		logger.Panic().Int32("port", port).
			Msg("failed to create agent via hub")
	}

	// 获取 hub 返回的公钥
	publicKey := response.Msg.GetAgentCreate().GetPublicKey()
	if publicKey == "" {
		logger.Panic().Msg("failed to get public key from hub")
	}

	var sshCmd *exec.Cmd

	startSSHD := func(publicKey string) {
		if isPortListening(cli.SshdPort) {
			logger.Info().Msg("sshd is already listening")
			return
		}

		tempDir, err := os.MkdirTemp("", "sshole_agent")
		if err != nil {
			logger.Panic().Err(err).Msg("create temp dir failed")
		}
		defer os.RemoveAll(tempDir)

		fileSSHTarGz := "/tmp/sshole_agent/openssh.tar.gz"

		if !goutils.FileExists(fileSSHTarGz) {
			logger.Info().Msg("Downloading openssh")

			if err := os.MkdirAll("/tmp/sshole_agent", 0755); err != nil {
				logger.Panic().Err(err).Msg("create tmp dir failed")
			}
			// TODO: replace url
			if err := goutils.Download("https://webdav.cloud.117503445.top/public-writable/openssh-V_9_9_P2.tgz", fileSSHTarGz); err != nil {
				logger.Panic().Err(err).Msg("download openssh failed")
			}
		} else {
			logger.Info().Msg("Using cached openssh tar.gz")
		}

		fileSsh := "/tmp/sshole_agent/opt/openssh/bin/ssh"
		if !goutils.FileExists(fileSsh) {
			logger.Info().Msg("Extracting openssh")
			if err := goutils.Extract(ctx, fileSSHTarGz, "/tmp/sshole_agent"); err != nil {
				logger.Panic().Err(err).Msg("extract openssh failed")
			}
		} else {
			logger.Info().Msg("Using cached openssh")
		}

		if err := goutils.WriteText("/tmp/sshole_agent/opt/openssh/etc/sshd_config", fmt.Sprintf(`Port %v
ListenAddress 127.0.0.1
PermitRootLogin yes
PasswordAuthentication no
PubkeyAuthentication yes
AuthorizedKeysFile /tmp/sshole_agent/authorized_keys
HostKey /tmp/sshole_agent/opt/openssh/etc/ssh_host_ed25519_key
Subsystem	sftp	/tmp/sshole_agent/opt/openssh/libexec/sftp-server`, cli.SshdPort)); err != nil {
			logger.Panic().Err(err).Msg("write sshd_config failed")
		}

		{
			src := "/tmp/sshole_agent/opt/openssh/libexec/sshd-session"
			target := "/opt/openssh/libexec/sshd-session"

			if !goutils.FileExists(target) {
				// 确保父目录存在
				linkDir := filepath.Dir(target)
				if err := os.MkdirAll(linkDir, 0755); err != nil {
					logger.Error().Err(err).Str("linkDir", linkDir).Msg("failed to create parent directory")
				}

				// 创建符号链接
				if err := os.Symlink(src, target); err != nil {
					logger.Error().Err(err).Str("target", src).Str("linkPath", target).Msg("failed to create symlink")
				}

				logger.Info().Str("target", src).Str("linkPath", target).Msg("created symlink")
			}
		}

		// utils.Execute(ctx, utils.ExecuteParams{
		// 	Cmd: utils.Command("/tmp/sshole_agent/opt/openssh/bin/ssh-keygen -A"),
		// })

		{
			if err := goutils.WriteText("/tmp/sshole_agent/opt/openssh/etc/ssh_host_ed25519_key", `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCBBWLFnI3INjjSXukHEWabLHmz3lSbm7tqTe3P/Yty4AAAAJg/bIunP2yL
pwAAAAtzc2gtZWQyNTUxOQAAACCBBWLFnI3INjjSXukHEWabLHmz3lSbm7tqTe3P/Yty4A
AAAEBb33HgWMfWGHw29eFrHV/lNCbTNkUt4zVVz/rNPWm6j4EFYsWcjcg2ONJe6QcRZpss
ebPeVJubu2pN7c/9i3LgAAAAEXJvb3RANjUzNGE4ZmEzNjMyAQIDBA==
-----END OPENSSH PRIVATE KEY-----
			`); err != nil {
				logger.Warn().Err(err).Msg("failed to write ssh_host_ed25519_key")
			}
			if err := os.Chmod("/tmp/sshole_agent/opt/openssh/etc/ssh_host_ed25519_key", 0600); err != nil {
				logger.Warn().Err(err).Msg("failed to chmod ssh_host_ed25519_key")
			}
		}

		{
			dir := "/opt/openssh/var/empty"

			// 创建目录（等效于 mkdir -p）
			if err := os.MkdirAll(dir, 0755); err != nil {
				logger.Error().Err(err).Str("dir", dir).Msg("failed to create directory")
			}
		}

		sshCmd = utils.Command("/tmp/sshole_agent/opt/openssh/sbin/sshd -D -e -f /tmp/sshole_agent/opt/openssh/etc/sshd_config")
		go func() {
			utils.Execute(ctx, utils.ExecuteParams{
				Cmd: sshCmd,
			})
		}()

		for {
			time.Sleep(time.Second) // 等待 sshd 启动
			if isPortListening(cli.SshdPort) {
				logger.Info().Msg("sshd is already listening")

				// SSHD 启动后，将 hub 的公钥添加到 authorized_keys
				if err := goutils.WriteText("/tmp/sshole_agent/authorized_keys", publicKey); err != nil {
					logger.Warn().Err(err).Msg("failed to write authorized_keys")
				} else {
					logger.Info().Msg("added hub public key to authorized_keys")
				}

				return
			}
		}
	}

	startSSHD(publicKey)
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
		Int("AgentPort", cli.SshdPort).
		Msg("Starting chisel")

	c, err := chclient.NewClient(&chclient.Config{
		Server:  cli.HubServer,
		Remotes: []string{fmt.Sprintf("R:%d:localhost:%v", port, cli.SshdPort)}, // 将 本地的 SshdPort 端口映射到 hub 的 port 端口
		Auth:    cli.Auth,
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
