package common

// ErrCode is a minimal error code set for user-facing failures.
type ErrCode string

const (
	ErrAuthFailed      ErrCode = "AUTH_FAILED"
	ErrAgentOffline    ErrCode = "AGENT_OFFLINE"
	ErrSessionNotFound ErrCode = "SESSION_NOT_FOUND"
	ErrSessionMismatch ErrCode = "SESSION_MISMATCH"
	ErrDuplicateTunnel ErrCode = "DUPLICATE_TUNNEL"
	ErrHandshakeFailed ErrCode = "HANDSHAKE_FAILED"
	ErrTimeout         ErrCode = "TIMEOUT"
)
