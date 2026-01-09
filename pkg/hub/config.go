package hub

import (
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
	return out
}
