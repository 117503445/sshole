// Package tunnel provides a WebSocket-based bidirectional port forwarding implementation.
package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// TunnelType represents the direction of the tunnel.
type TunnelType string

const (
	// LocalToRemote means local port listens and forwards to remote port.
	LocalToRemote TunnelType = "localToRemote"
	// RemoteToLocal means remote port listens and forwards to local port.
	RemoteToLocal TunnelType = "remoteToLocal"
)

// ConnStatus represents the connection status.
type ConnStatus string

const (
	StatusConnected    ConnStatus = "connected"
	StatusDisconnected ConnStatus = "disconnected"
)

// MessageType represents the type of control message.
type MessageType string

const (
	MsgPing             MessageType = "ping"
	MsgPong             MessageType = "pong"
	MsgTunnelRequest    MessageType = "tunnel_request"
	MsgTunnelResponse   MessageType = "tunnel_response"
	MsgTunnelRemove     MessageType = "tunnel_remove"
	MsgTunnelRemoveAck  MessageType = "tunnel_remove_ack"
	MsgStreamOpen       MessageType = "stream_open"
	MsgStreamOpenAck    MessageType = "stream_open_ack"
	MsgStreamData       MessageType = "stream_data"
	MsgStreamClose      MessageType = "stream_close"
	MsgError            MessageType = "error"
)

// Constants for heartbeat mechanism.
const (
	HeartbeatInterval = 5 * time.Second
	HeartbeatTimeout  = 12 * time.Second
	WriteTimeout      = 10 * time.Second
	ReadTimeout       = 15 * time.Second
)

// Message represents a control message sent over WebSocket.
type Message struct {
	Type      MessageType     `json:"type"`
	ID        string          `json:"id,omitempty"`        // Message ID for request/response matching
	TunnelID  string          `json:"tunnel_id,omitempty"` // Tunnel ID
	StreamID  string          `json:"stream_id,omitempty"` // Stream ID for data forwarding
	Payload   json.RawMessage `json:"payload,omitempty"`   // Additional payload (for structured data)
	Data      []byte          `json:"data,omitempty"`      // Binary data (base64 encoded in JSON)
	Error     string          `json:"error,omitempty"`     // Error message
	Timestamp int64           `json:"timestamp"`           // Unix timestamp
}

// TunnelRequest is the payload for tunnel creation request.
type TunnelRequest struct {
	TunnelType TunnelType `json:"tunnel_type"`
	LocalPort  int        `json:"local_port"`
	RemotePort int        `json:"remote_port"`
}

// TunnelResponse is the payload for tunnel creation response.
type TunnelResponse struct {
	TunnelID string `json:"tunnel_id"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// StreamOpenRequest is the payload for stream open request.
type StreamOpenRequest struct {
	TunnelID string `json:"tunnel_id"`
}

// Tunnel represents a port forwarding tunnel.
type Tunnel struct {
	ID         string     `json:"id"`
	ConnID     string     `json:"conn_id"`
	Type       TunnelType `json:"type"`
	LocalPort  int        `json:"local_port"`
	RemotePort int        `json:"remote_port"`
	Status     string     `json:"status"` // "active", "stopped"

	// Internal fields
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	streams   map[string]*Stream // streamID -> Stream
	streamsMu sync.RWMutex
}

// Stream represents a single TCP connection being forwarded through the tunnel.
type Stream struct {
	ID       string
	TunnelID string
	conn     net.Conn       // Local TCP connection
	ctx      context.Context
	cancel   context.CancelFunc
	closed   atomic.Bool
}

// Conn represents a WebSocket connection to a remote TunnelManager.
type Conn struct {
	ID             string     `json:"id"`
	RemoteID       string     `json:"remote_id"` // Remote TunnelManager's ID
	Status         ConnStatus `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	LastHeartbeat  time.Time  `json:"last_heartbeat"`

	// Internal fields
	ws            *websocket.Conn
	ctx           context.Context
	cancel        context.CancelFunc
	tm            *TunnelManager
	mu            sync.Mutex
	tunnels       map[string]*Tunnel // tunnelID -> Tunnel
	tunnelsMu     sync.RWMutex
	pendingReqs   map[string]chan *Message // messageID -> response channel
	pendingReqsMu sync.Mutex
	writeMu       sync.Mutex // Mutex for concurrent write protection
}

// TunnelManager manages all connections and tunnels.
type TunnelManager struct {
	ID   string `json:"id"`
	Auth string `json:"-"` // Hidden from JSON

	conns   map[string]*Conn
	connsMu sync.RWMutex

	tunnels   map[string]*Tunnel
	tunnelsMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// Common errors
var (
	ErrAuthFailed       = errors.New("authentication failed")
	ErrConnNotFound     = errors.New("connection not found")
	ErrTunnelNotFound   = errors.New("tunnel not found")
	ErrInvalidTunnelType = errors.New("invalid tunnel type")
	ErrPortInUse        = errors.New("port already in use")
	ErrConnectionClosed = errors.New("connection closed")
)

// NewTunnelManager creates a new TunnelManager instance.
func NewTunnelManager(auth string) *TunnelManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &TunnelManager{
		ID:      uuid.New().String(),
		Auth:    auth,
		conns:   make(map[string]*Conn),
		tunnels: make(map[string]*Tunnel),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Close shuts down the TunnelManager and all its connections.
func (tm *TunnelManager) Close() error {
	tm.cancel()

	tm.connsMu.Lock()
	defer tm.connsMu.Unlock()

	for _, conn := range tm.conns {
		conn.Close()
	}
	tm.conns = make(map[string]*Conn)

	return nil
}

// newConn creates a new Conn instance.
func (tm *TunnelManager) newConn(ws *websocket.Conn, remoteID string) *Conn {
	ctx, cancel := context.WithCancel(tm.ctx)
	conn := &Conn{
		ID:            uuid.New().String(),
		RemoteID:      remoteID,
		Status:        StatusConnected,
		CreatedAt:     time.Now(),
		LastHeartbeat: time.Now(),
		ws:            ws,
		ctx:           ctx,
		cancel:        cancel,
		tm:            tm,
		tunnels:       make(map[string]*Tunnel),
		pendingReqs:   make(map[string]chan *Message),
	}
	return conn
}

// registerConn adds a connection to the manager.
func (tm *TunnelManager) registerConn(conn *Conn) {
	tm.connsMu.Lock()
	defer tm.connsMu.Unlock()
	tm.conns[conn.ID] = conn
}

// unregisterConn removes a connection from the manager.
func (tm *TunnelManager) unregisterConn(connID string) {
	tm.connsMu.Lock()
	defer tm.connsMu.Unlock()
	if conn, ok := tm.conns[connID]; ok {
		conn.Close()
		delete(tm.conns, connID)
	}
}

// getConn retrieves a connection by ID.
func (tm *TunnelManager) getConn(connID string) (*Conn, error) {
	tm.connsMu.RLock()
	defer tm.connsMu.RUnlock()
	if conn, ok := tm.conns[connID]; ok {
		return conn, nil
	}
	return nil, ErrConnNotFound
}

// Close closes the connection and all its tunnels.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Status = StatusDisconnected
	c.cancel()

	// Close all tunnels
	c.tunnelsMu.Lock()
	for _, tunnel := range c.tunnels {
		tunnel.Close()
	}
	c.tunnels = make(map[string]*Tunnel)
	c.tunnelsMu.Unlock()

	// Close pending requests
	c.pendingReqsMu.Lock()
	for _, ch := range c.pendingReqs {
		close(ch)
	}
	c.pendingReqs = make(map[string]chan *Message)
	c.pendingReqsMu.Unlock()

	if c.ws != nil {
		return c.ws.Close(websocket.StatusNormalClosure, "connection closed")
	}
	return nil
}

// sendMessage sends a message over the WebSocket connection.
func (c *Conn) sendMessage(msg *Message) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	msg.Timestamp = time.Now().Unix()
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(c.ctx, WriteTimeout)
	defer cancel()

	return c.ws.Write(ctx, websocket.MessageText, data)
}

// sendRequest sends a request and waits for response.
func (c *Conn) sendRequest(msg *Message, timeout time.Duration) (*Message, error) {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	respCh := make(chan *Message, 1)
	c.pendingReqsMu.Lock()
	c.pendingReqs[msg.ID] = respCh
	c.pendingReqsMu.Unlock()

	defer func() {
		c.pendingReqsMu.Lock()
		delete(c.pendingReqs, msg.ID)
		c.pendingReqsMu.Unlock()
	}()

	if err := c.sendMessage(msg); err != nil {
		return nil, err
	}

	select {
	case resp, ok := <-respCh:
		if !ok {
			return nil, ErrConnectionClosed
		}
		return resp, nil
	case <-time.After(timeout):
		return nil, errors.New("request timeout")
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// handleResponse handles a response message.
func (c *Conn) handleResponse(msg *Message) {
	c.pendingReqsMu.Lock()
	if ch, ok := c.pendingReqs[msg.ID]; ok {
		select {
		case ch <- msg:
		default:
		}
	}
	c.pendingReqsMu.Unlock()
}

// Close closes the tunnel and stops port forwarding.
func (t *Tunnel) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Status = "stopped"
	if t.cancel != nil {
		t.cancel()
	}

	// Close all streams
	t.streamsMu.Lock()
	for _, stream := range t.streams {
		stream.Close()
	}
	t.streams = make(map[string]*Stream)
	t.streamsMu.Unlock()

	if t.listener != nil {
		return t.listener.Close()
	}
	return nil
}

// addStream adds a stream to the tunnel.
func (t *Tunnel) addStream(stream *Stream) {
	t.streamsMu.Lock()
	defer t.streamsMu.Unlock()
	t.streams[stream.ID] = stream
}

// removeStream removes a stream from the tunnel.
func (t *Tunnel) removeStream(streamID string) {
	t.streamsMu.Lock()
	defer t.streamsMu.Unlock()
	if stream, ok := t.streams[streamID]; ok {
		stream.Close()
		delete(t.streams, streamID)
	}
}

// getStream gets a stream by ID.
func (t *Tunnel) getStream(streamID string) (*Stream, bool) {
	t.streamsMu.RLock()
	defer t.streamsMu.RUnlock()
	stream, ok := t.streams[streamID]
	return stream, ok
}

// Close closes the stream.
func (s *Stream) Close() error {
	if s.closed.CompareAndSwap(false, true) {
		if s.cancel != nil {
			s.cancel()
		}
		if s.conn != nil {
			return s.conn.Close()
		}
	}
	return nil
}

// Write writes data to the stream's TCP connection.
func (s *Stream) Write(data []byte) (int, error) {
	if s.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	if s.conn == nil {
		return 0, errors.New("no connection")
	}
	return s.conn.Write(data)
}

// Read reads data from the stream's TCP connection.
func (s *Stream) Read(buf []byte) (int, error) {
	if s.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	if s.conn == nil {
		return 0, errors.New("no connection")
	}
	return s.conn.Read(buf)
}

