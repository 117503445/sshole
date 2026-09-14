package common

import (
	"testing"
	"time"
)

func TestDefaultTimeouts(t *testing.T) {
	to := DefaultTimeouts()
	if to.PendingTimeout <= 0 || to.TunnelDialTimeout <= 0 {
		t.Fatalf("timeouts must be positive: %+v", to)
	}
	if to.PendingTimeout < to.TunnelDialTimeout {
		t.Fatalf("pending timeout %v should cover tunnel dial %v", to.PendingTimeout, to.TunnelDialTimeout)
	}
	if to.AgentReconnectMaxRetries < 1 {
		t.Fatalf("reconnect retries = %d", to.AgentReconnectMaxRetries)
	}
	if to.AgentReconnectBackoff < 100*time.Millisecond {
		t.Fatalf("reconnect backoff too small: %v", to.AgentReconnectBackoff)
	}
}
