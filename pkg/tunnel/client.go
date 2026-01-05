package tunnel

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

// Connect establishes a WebSocket connection to the remote TunnelManager.
func (tm *TunnelManager) Connect(targetURL string, auth string) (*Conn, error) {
	// Parse and modify URL to include auth and tm_id
	u, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	q := u.Query()
	q.Set("auth", auth)
	q.Set("tm_id", tm.ID)
	u.RawQuery = q.Encode()

	// Convert http(s) to ws(s) if needed
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	ctx, cancel := context.WithTimeout(tm.ctx, 30*time.Second)
	defer cancel()

	// Dial WebSocket
	ws, resp, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Basic " + auth},
		},
	})
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			return nil, ErrAuthFailed
		}
		return nil, err
	}

	// Create and register connection
	remoteID := "server" // Server side ID, could be extracted from response headers
	conn := tm.newConn(ws, remoteID)
	tm.registerConn(conn)

	log.Info().
		Str("conn_id", conn.ID).
		Str("target", targetURL).
		Msg("Connected to remote TunnelManager")

	// Start connection handlers
	go conn.startHeartbeat()
	go conn.handleMessages()

	return conn, nil
}

// ListConns returns all active or disconnected connections.
func (tm *TunnelManager) ListConns() []*Conn {
	tm.connsMu.RLock()
	defer tm.connsMu.RUnlock()

	conns := make([]*Conn, 0, len(tm.conns))
	for _, conn := range tm.conns {
		conns = append(conns, conn)
	}
	return conns
}

// Disconnect closes and removes a connection by ID.
func (tm *TunnelManager) Disconnect(connID string) error {
	tm.connsMu.Lock()
	conn, ok := tm.conns[connID]
	if !ok {
		tm.connsMu.Unlock()
		return ErrConnNotFound
	}
	delete(tm.conns, connID)
	tm.connsMu.Unlock()

	// Remove all tunnels associated with this connection
	tm.tunnelsMu.Lock()
	for tunnelID, tunnel := range tm.tunnels {
		if tunnel.ConnID == connID {
			delete(tm.tunnels, tunnelID)
		}
	}
	tm.tunnelsMu.Unlock()

	return conn.Close()
}

// GetConn retrieves a connection by ID.
func (tm *TunnelManager) GetConn(connID string) (*Conn, error) {
	return tm.getConn(connID)
}

// IsConnected checks if a connection is currently connected.
func (c *Conn) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Status == StatusConnected
}

// GetID returns the connection ID.
func (c *Conn) GetID() string {
	return c.ID
}

// GetRemoteID returns the remote TunnelManager's ID.
func (c *Conn) GetRemoteID() string {
	return c.RemoteID
}

// GetStatus returns the connection status.
func (c *Conn) GetStatus() ConnStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Status
}

// GetCreatedAt returns the connection creation time.
func (c *Conn) GetCreatedAt() time.Time {
	return c.CreatedAt
}

// GetLastHeartbeat returns the last heartbeat time.
func (c *Conn) GetLastHeartbeat() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LastHeartbeat
}


