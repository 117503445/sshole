package proto

// ControlMessage is the only control plane JSON message on /agent.
type ControlMessage struct {
	Type       string `json:"type"`
	SessionID  string `json:"session_id,omitempty"`
	KnownHost  string `json:"known_host,omitempty"`
}
