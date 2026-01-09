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
	if out.Timeouts.AgentReconnectMaxRetries == 0 {
		out.Timeouts.AgentReconnectMaxRetries = timeouts.AgentReconnectMaxRetries
	}
	return out
}

// retryBackoff returns a bounded backoff duration.
func (c AgentConfig) retryBackoff(attempt int) time.Duration {
	backoff := c.Timeouts.AgentReconnectBackoff * time.Duration(attempt+1)
	max := 30 * time.Second
	if backoff > max {
		return max
	}
	return backoff
}
