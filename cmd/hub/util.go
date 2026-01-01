package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

func newConnWithPort(port int32) (conn, error) {
	// port: 随机一个本地可用的 tcp port 或使用指定端口
	// ssh key: 随机生成一个 ed25519 key

	// 检查指定端口是否可用
	var listener net.Listener
	var err error

	if port != 0 {
		// 尝试监听指定端口
		listener, err = net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			// 指定端口不可用，回退到随机端口
			listener, err = net.Listen("tcp", "localhost:0")
			if err != nil {
				return conn{}, err
			}
		}
	} else {
		// 使用随机端口
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return conn{}, err
		}
	}

	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port

	// 生成 ed25519 SSH 密钥对
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return conn{}, err
	}

	// 将密钥转换为 SSH 格式
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	if err != nil {
		return conn{}, err
	}

	// 将私钥转换为OpenSSH格式
	sshPrivKey, err := ssh.MarshalPrivateKey(privKey, "")
	if err != nil {
		return conn{}, err
	}

	return conn{
		Port:          int32(actualPort),
		SshPublicKey:  string(ssh.MarshalAuthorizedKey(sshPubKey)),
		SshPrivateKey: string(pem.EncodeToMemory(sshPrivKey)),
	}, nil
}

// findFreePort 找到一个本地可用的 tcp port
func findFreePort() (int32, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return int32(listener.Addr().(*net.TCPAddr).Port), nil
}
