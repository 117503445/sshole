package hub

import (
	"net"
	"time"

	"github.com/coder/websocket"
)

type PendingState int

const (
	PendingINIT PendingState = iota
	PendingOPEN_SENT
	PendingBOUND
	PendingTIMEOUT
	PendingCLOSED
)

type PendingSession struct {
	SessionID string
	AgentName string

	SSHConn net.Conn

	State PendingState

	CreatedAt time.Time
	Deadline  time.Time

	Tunnel *websocket.Conn
}
