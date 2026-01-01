package utils

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

type SshExecuteParams struct {
	Host       string
	Port       int
	User       string
	Command    string
	PrivateKey []byte
}

type SshExecuteResult struct {
	Output   string
	ExitCode int
}

func SshExecute(ctx context.Context, params SshExecuteParams) (SshExecuteResult, error) {
	// 解析私钥
	signer, err := ssh.ParsePrivateKey(params.PrivateKey)
	if err != nil {
		return SshExecuteResult{}, fmt.Errorf("failed to parse private key: %w", err)
	}

	// 创建 SSH 客户端配置
	config := &ssh.ClientConfig{
		User: params.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 注意：生产环境中应该验证主机密钥
	}

	// 建立连接
	hostPort := fmt.Sprintf("%s:%d", params.Host, params.Port)
	client, err := ssh.Dial("tcp", hostPort, config)
	if err != nil {
		return SshExecuteResult{}, fmt.Errorf("failed to dial SSH server: %w", err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return SshExecuteResult{}, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// 设置上下文取消
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-done:
		}
	}()

	// 执行命令并获取输出
	output, err := session.CombinedOutput(params.Command)
	exitCode := 0

	if err != nil {
		// 检查是否是退出错误
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return SshExecuteResult{}, fmt.Errorf("failed to execute command: %w", err)
		}
	}

	// 返回结果
	return SshExecuteResult{
		Output:   strings.TrimSpace(string(output)),
		ExitCode: exitCode,
	}, nil
}
