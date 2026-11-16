package hub

import (
	"io/fs"
	"time"

	"github.com/117503445/sshole/pkg/common"
)

// HubConfig holds runtime configuration for the hub.
type HubConfig struct {
	AuthToken         string
	HTTPAddr          string
	MappingFile       string
	PendingTimeout    time.Duration
	TunnelDialTimeout time.Duration
	// AgentPingInterval 是 Hub 主动探测 Agent 控制连接的周期； pong 回不来就判定离线。
	AgentPingInterval time.Duration
	// AgentPingTimeout 是单次探测等待 pong 的上限。
	AgentPingTimeout time.Duration
	// BinsFS holds entry/agent binaries embedded at build time (CI only).
	BinsFS fs.FS
}

func (c *HubConfig) withDefaults() HubConfig {
	out := *c
	if out.HTTPAddr == "" {
		out.HTTPAddr = ":9000"
	}
	timeouts := common.DefaultTimeouts()
	if out.PendingTimeout == 0 {
		out.PendingTimeout = timeouts.PendingTimeout
	}
	if out.TunnelDialTimeout == 0 {
		out.TunnelDialTimeout = timeouts.TunnelDialTimeout
	}
	if out.AgentPingInterval == 0 {
		out.AgentPingInterval = 20 * time.Second
	}
	if out.AgentPingTimeout == 0 {
		out.AgentPingTimeout = 10 * time.Second
	}
	return out
}
