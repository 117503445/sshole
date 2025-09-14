package main

import (
	"context"
	"os/exec"

	"github.com/gliderlabs/ssh"
	"github.com/rs/zerolog/log"
)

func cmdAgent(ctx context.Context) {
	logger := log.Ctx(ctx)
	logger.Info().Msg("Starting agent")

	ssh.Handle(func(s ssh.Session) {
		// 启动一个 shell，例如 /bin/bash
		cmd := exec.Command("/bin/bash")
		cmd.Stdin = s
		cmd.Stdout = s
		cmd.Stderr = s

		// 设置终端环境（可选但推荐）
		pty, _, isPty := s.Pty()
		if isPty {
			cmd.Env = append(cmd.Env, "TERM="+pty.Term)
		}

		// 运行 shell
		err := cmd.Run()
		if err != nil {
			log.Printf("shell exited with error: %v", err)
		}
	})

	ssh.ListenAndServe(":22222", nil)

}
