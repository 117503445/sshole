package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"os/exec"
	"syscall"

	"golang.org/x/crypto/ssh"
)

func main() {
	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal("Failed to generate private key:", err)
	}

	// 创建 SSH 服务器配置
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// 简单的用户名密码验证 (用户名: test, 密码: test)
			if c.User() == "test" && subtle.ConstantTimeCompare(pass, []byte("test")) == 1 {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	// 添加主机密钥
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		log.Fatal("Failed to create signer:", err)
	}
	config.AddHostKey(signer)

	// 监听端口
	listener, err := net.Listen("tcp", "localhost:2222")
	if err != nil {
		log.Fatal("Failed to listen for connection:", err)
	}
	defer listener.Close()

	fmt.Println("SSH server listening on localhost:2222")
	fmt.Println("Connect with: ssh -p 2222 test@localhost (password: test)")

	for {
		// 接受连接
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection: %v", err)
			continue
		}

		// 处理连接
		go handleConnection(conn, config)
	}
}

func handleConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()

	// 执行 SSH 握手
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		log.Printf("Failed to handshake: %v", err)
		return
	}

	log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())

	// 丢弃所有全局请求
	go ssh.DiscardRequests(reqs)

	// 接受通道
	for newChannel := range chans {
		// 确保是会话通道
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		// 接受通道
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept channel: %v", err)
			continue
		}

		// 创建 PTY
		var pty PTY

		// 处理会话请求
		go func(in <-chan *ssh.Request) {
			for req := range in {
				switch req.Type {
				case "pty-req":
					// 解析 PTY 请求
					if len(req.Payload) < 4 {
						req.Reply(false, nil)
						continue
					}
					pty.Width = int(req.Payload[0])<<24 | int(req.Payload[1])<<16 | int(req.Payload[2])<<8 | int(req.Payload[3])
					pty.Height = int(req.Payload[4])<<24 | int(req.Payload[5])<<16 | int(req.Payload[6])<<8 | int(req.Payload[7])
					pty.IsSet = true
					req.Reply(true, nil)
				case "shell":
					// 启动 shell
					go handleShell(channel, pty)
					req.Reply(true, nil)
				default:
					req.Reply(false, nil)
				}
			}
		}(requests)
	}
}

type PTY struct {
	Width  int
	Height int
	IsSet  bool
}

func handleShell(channel ssh.Channel, pty PTY) {
	defer channel.Close()

	// 启动 bash shell
	cmd := exec.Command("/bin/bash")

	// 设置标准输入输出
	cmd.Stdin = channel
	cmd.Stdout = channel
	cmd.Stderr = channel

	// 如果有 PTY，则设置系统进程属性
	if pty.IsSet {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setctty: true,
			Setsid:  true,
		}
	}

	// 运行命令
	err := cmd.Run()
	if err != nil {
		log.Printf("Failed to run command: %v", err)
	}
}