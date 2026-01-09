package common

import "time"

// Timeouts groups all timeout and retry related settings.
type Timeouts struct {
	PendingTimeout           time.Duration
	TunnelDialTimeout        time.Duration
	AgentReconnectMaxRetries int
	AgentReconnectBackoff    time.Duration
}

// DefaultTimeouts returns the recommended defaults from the design docs.
func DefaultTimeouts() Timeouts {
	return Timeouts{
		PendingTimeout:           10 * time.Second,
		TunnelDialTimeout:        5 * time.Second,
		AgentReconnectMaxRetries: 3,
		AgentReconnectBackoff:    1 * time.Second,
	}
}
