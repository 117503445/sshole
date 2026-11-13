package agent

import (
	"time"

	"github.com/117503445/sshole/pkg/common"
)

// AgentConfig holds runtime settings for the agent process.
type AgentConfig struct {
	HubURL    string
	Token     string
	AgentName string
	LocalPort int

	// SkipSSHD when true skips starting the embedded sshd server.
	// The agent will connect to an existing SSH server on LocalPort.
	SkipSSHD bool

	Timeouts common.Timeouts
}

func (c *AgentConfig) withDefaults() AgentConfig {
	out := *c
	if out.LocalPort == 0 {
		out.LocalPort = 22222
	}
	timeouts := common.DefaultTimeouts()
	if out.Timeouts.PendingTimeout == 0 {
		out.Timeouts.PendingTimeout = timeouts.PendingTimeout
	}
	if out.Timeouts.TunnelDialTimeout == 0 {
		out.Timeouts.TunnelDialTimeout = timeouts.TunnelDialTimeout
	}
	if out.Timeouts.AgentReconnectBackoff == 0 {
		out.Timeouts.AgentReconnectBackoff = timeouts.AgentReconnectBackoff
	}
	return out
}

// retryBackoff 返回第 attempt 次失败后的退避时间，最长 30s；重连本身不限次数。
func (c AgentConfig) retryBackoff(attempt int) time.Duration {
	backoff := c.Timeouts.AgentReconnectBackoff * time.Duration(attempt+1)
	max := 30 * time.Second
	if backoff > max {
		return max
	}
	return backoff
}
