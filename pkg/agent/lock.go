package agent

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrAlreadyRunning 表示已有另一个 agent 实例持有单实例锁。
var ErrAlreadyRunning = errors.New("another agent instance is already running")

// AcquireInstanceLock 获取 agent 单实例锁（$HOME/.sshole/agent.lock）。
// 成功返回释放函数；已有实例运行时返回 ErrAlreadyRunning。
// 锁由操作系统在进程退出时自动释放，无残留状态。
func AcquireInstanceLock() (func(), error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	ssholeDir := filepath.Join(home, ".sshole")
	if err := os.MkdirAll(ssholeDir, 0o700); err != nil {
		return nil, err
	}
	return acquireFileLock(filepath.Join(ssholeDir, "agent.lock"))
}
