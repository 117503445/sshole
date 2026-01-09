package tunnel

import (
	"context"
	"net"

	"github.com/coder/websocket"
)

// NetConn wraps the websocket into a net.Conn using binary messages.
func NetConn(ctx context.Context, ws *websocket.Conn) net.Conn {
	return websocket.NetConn(ctx, ws, websocket.MessageBinary)
}
