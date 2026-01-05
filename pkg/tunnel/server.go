package tunnel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Handler handles WebSocket upgrade requests for tunnel connections.
// It validates the auth token and establishes the WebSocket connection.
func (tm *TunnelManager) Handler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.Ctx(ctx)

	// Extract auth from query parameter or subprotocol
	auth := r.URL.Query().Get("auth")
	if auth == "" {
		// Try to get from Authorization header (Basic auth format)
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 6 && authHeader[:6] == "Basic " {
			decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
			if err == nil {
				auth = string(decoded)
			}
		}
	}

	// Validate auth
	if tm.Auth != "" && auth != tm.Auth {
		logger.Warn().Str("remote", r.RemoteAddr).Msg("WebSocket auth failed")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Accept WebSocket connection
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections from any origin
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to accept WebSocket connection")
		return
	}

	// Get remote TunnelManager ID from query or generate placeholder
	remoteID := r.URL.Query().Get("tm_id")
	if remoteID == "" {
		remoteID = "unknown-" + uuid.New().String()[:8]
	}

	// Create and register connection
	conn := tm.newConn(ws, remoteID)
	tm.registerConn(conn)

	logger.Info().
		Str("conn_id", conn.ID).
		Str("remote_id", remoteID).
		Str("remote_addr", r.RemoteAddr).
		Msg("WebSocket connection established")

	// Start connection handlers
	go conn.startHeartbeat()
	go conn.handleMessages()
}

// startHeartbeat sends periodic ping messages and checks for timeout.
func (c *Conn) startHeartbeat() {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// Send ping
			if err := c.sendMessage(&Message{
				Type: MsgPing,
				ID:   uuid.New().String(),
			}); err != nil {
				log.Debug().Err(err).Str("conn_id", c.ID).Msg("Failed to send ping")
				c.Status = StatusDisconnected
				continue
			}

			// Check for timeout
			c.mu.Lock()
			if time.Since(c.LastHeartbeat) > HeartbeatTimeout {
				c.Status = StatusDisconnected
				log.Warn().Str("conn_id", c.ID).Msg("Connection heartbeat timeout")
			} else {
				c.Status = StatusConnected
			}
			c.mu.Unlock()
		}
	}
}

// handleMessages reads and processes messages from the WebSocket connection.
func (c *Conn) handleMessages() {
	defer func() {
		c.tm.unregisterConn(c.ID)
		log.Info().Str("conn_id", c.ID).Msg("Connection message handler stopped")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read message
		_, data, err := c.ws.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) != -1 {
				log.Debug().Err(err).Str("conn_id", c.ID).Msg("WebSocket closed")
			} else {
				log.Error().Err(err).Str("conn_id", c.ID).Msg("Failed to read message")
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Error().Err(err).Str("conn_id", c.ID).Msg("Failed to unmarshal message")
			continue
		}

		// Update last heartbeat time for any message
		c.mu.Lock()
		c.LastHeartbeat = time.Now()
		c.mu.Unlock()

		// Handle message based on type
		switch msg.Type {
		case MsgPing:
			c.handlePing(&msg)
		case MsgPong:
			// Just update heartbeat (already done above)
		case MsgTunnelRequest:
			c.handleTunnelRequest(&msg)
		case MsgTunnelResponse:
			c.handleResponse(&msg)
		case MsgTunnelRemove:
			c.handleTunnelRemove(&msg)
		case MsgTunnelRemoveAck:
			c.handleResponse(&msg)
		case MsgStreamOpen:
			c.handleStreamOpen(&msg)
		case MsgStreamOpenAck:
			c.handleResponse(&msg)
		case MsgStreamData:
			c.handleStreamData(&msg)
		case MsgStreamClose:
			c.handleStreamClose(&msg)
		case MsgError:
			log.Error().Str("error", msg.Error).Str("conn_id", c.ID).Msg("Received error message")
		default:
			log.Warn().Str("type", string(msg.Type)).Str("conn_id", c.ID).Msg("Unknown message type")
		}
	}
}

// handlePing responds to a ping message with a pong.
func (c *Conn) handlePing(msg *Message) {
	if err := c.sendMessage(&Message{
		Type: MsgPong,
		ID:   msg.ID,
	}); err != nil {
		log.Error().Err(err).Str("conn_id", c.ID).Msg("Failed to send pong")
	}
}

// handleTunnelRequest handles a request to create a new tunnel.
func (c *Conn) handleTunnelRequest(msg *Message) {
	var req TunnelRequest
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		c.sendTunnelResponse(msg.ID, "", false, "invalid request payload")
		return
	}

	log.Info().
		Str("conn_id", c.ID).
		Str("type", string(req.TunnelType)).
		Int("local_port", req.LocalPort).
		Int("remote_port", req.RemotePort).
		Msg("Received tunnel request")

	// Create tunnel based on type
	// From server's perspective:
	// - LocalToRemote: client listens locally, server connects to remotePort when stream opens
	// - RemoteToLocal: server listens on remotePort, client connects to localPort when stream opens
	var tunnelID string
	var err error

	switch req.TunnelType {
	case LocalToRemote:
		// Client wants to forward their local to our remote port
		// We just register the tunnel - we'll connect to remotePort when stream opens
		tunnelID = uuid.New().String()
		tunnel := &Tunnel{
			ID:         tunnelID,
			ConnID:     c.ID,
			Type:       LocalToRemote,
			LocalPort:  req.LocalPort,
			RemotePort: req.RemotePort,
			Status:     "active",
			streams:    make(map[string]*Stream),
		}
		tunnel.ctx, tunnel.cancel = context.WithCancel(c.ctx)
		c.tunnelsMu.Lock()
		c.tunnels[tunnelID] = tunnel
		c.tunnelsMu.Unlock()
		c.tm.tunnelsMu.Lock()
		c.tm.tunnels[tunnelID] = tunnel
		c.tm.tunnelsMu.Unlock()
	case RemoteToLocal:
		// Client wants us to listen on remotePort and forward to their localPort
		// We need to listen on the specified remote port
		tunnelID, err = c.createRemoteListener(req.RemotePort, req.LocalPort)
	default:
		err = ErrInvalidTunnelType
	}

	if err != nil {
		c.sendTunnelResponse(msg.ID, "", false, err.Error())
		return
	}

	c.sendTunnelResponse(msg.ID, tunnelID, true, "")
}

// createRemoteListener creates a listener on the remote side for LocalToRemote tunnels.
func (c *Conn) createRemoteListener(listenPort, targetPort int) (string, error) {
	tunnelID := uuid.New().String()

	// Create listener
	listener, err := net.Listen("tcp", formatAddr(listenPort))
	if err != nil {
		return "", err
	}

	tunnel := &Tunnel{
		ID:         tunnelID,
		ConnID:     c.ID,
		Type:       LocalToRemote,
		LocalPort:  targetPort,  // From requester's perspective
		RemotePort: listenPort,  // Where we listen
		Status:     "active",
		listener:   listener,
		streams:    make(map[string]*Stream),
	}
	tunnel.ctx, tunnel.cancel = context.WithCancel(c.ctx)

	c.tunnelsMu.Lock()
	c.tunnels[tunnelID] = tunnel
	c.tunnelsMu.Unlock()

	c.tm.tunnelsMu.Lock()
	c.tm.tunnels[tunnelID] = tunnel
	c.tm.tunnelsMu.Unlock()

	// Start accepting connections
	go c.acceptConnections(tunnel)

	log.Info().
		Str("tunnel_id", tunnelID).
		Int("listen_port", listenPort).
		Int("target_port", targetPort).
		Msg("Remote listener created")

	return tunnelID, nil
}

// acceptConnections accepts TCP connections on the tunnel's listener.
func (c *Conn) acceptConnections(tunnel *Tunnel) {
	for {
		select {
		case <-tunnel.ctx.Done():
			return
		default:
		}

		conn, err := tunnel.listener.Accept()
		if err != nil {
			if tunnel.ctx.Err() != nil {
				return // Context cancelled
			}
			log.Error().Err(err).Str("tunnel_id", tunnel.ID).Msg("Failed to accept connection")
			continue
		}

		// Create stream for this connection
		streamID := uuid.New().String()
		streamCtx, streamCancel := context.WithCancel(tunnel.ctx)
		stream := &Stream{
			ID:       streamID,
			TunnelID: tunnel.ID,
			conn:     conn,
			ctx:      streamCtx,
			cancel:   streamCancel,
		}
		tunnel.addStream(stream)

		// Request stream open on remote side
		go c.handleLocalConnection(tunnel, stream)
	}
}

// handleLocalConnection handles a new local TCP connection by opening a stream to remote.
func (c *Conn) handleLocalConnection(tunnel *Tunnel, stream *Stream) {
	defer func() {
		tunnel.removeStream(stream.ID)
		c.sendMessage(&Message{
			Type:     MsgStreamClose,
			TunnelID: tunnel.ID,
			StreamID: stream.ID,
		})
	}()

	// Request stream open on remote
	reqPayload, _ := json.Marshal(&StreamOpenRequest{TunnelID: tunnel.ID})
	resp, err := c.sendRequest(&Message{
		Type:     MsgStreamOpen,
		TunnelID: tunnel.ID,
		StreamID: stream.ID,
		Payload:  reqPayload,
	}, 10*time.Second)

	if err != nil {
		log.Error().Err(err).Str("stream_id", stream.ID).Msg("Failed to open remote stream")
		return
	}

	if resp.Error != "" {
		log.Error().Str("error", resp.Error).Str("stream_id", stream.ID).Msg("Remote stream open failed")
		return
	}

	// Start forwarding data from local connection to WebSocket
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stream.ctx.Done():
			return
		default:
		}

		n, err := stream.conn.Read(buf)
		if err != nil {
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			if err := c.sendMessage(&Message{
				Type:     MsgStreamData,
				TunnelID: tunnel.ID,
				StreamID: stream.ID,
				Data:     data,
			}); err != nil {
				return
			}
		}
	}
}

// handleStreamOpen handles a request to open a new stream.
func (c *Conn) handleStreamOpen(msg *Message) {
	c.tunnelsMu.RLock()
	tunnel, ok := c.tunnels[msg.TunnelID]
	c.tunnelsMu.RUnlock()

	if !ok {
		log.Debug().
			Str("conn_id", c.ID).
			Str("tunnel_id", msg.TunnelID).
			Msg("Stream open: tunnel not found")
		c.sendMessage(&Message{
			Type:     MsgStreamOpenAck,
			ID:       msg.ID,
			TunnelID: msg.TunnelID,
			StreamID: msg.StreamID,
			Error:    "tunnel not found",
		})
		return
	}

	// Determine target port based on tunnel type and our role
	// LocalToRemote: client listens locally, we (server) connect to remotePort
	// RemoteToLocal: we listen on remotePort, client connects to localPort
	var targetPort int
	if tunnel.Type == LocalToRemote {
		// We are on the server side, connect to remotePort (target on our side)
		targetPort = tunnel.RemotePort
	} else {
		// We are on the server side, client should connect to localPort
		// But if we receive stream open, it means someone connected to our listener
		// and we need the client to connect to their local port - this is handled differently
		targetPort = tunnel.LocalPort
	}

	log.Debug().
		Str("conn_id", c.ID).
		Str("tunnel_id", msg.TunnelID).
		Str("stream_id", msg.StreamID).
		Int("target_port", targetPort).
		Msg("Stream open: connecting to target")

	tcpConn, err := net.Dial("tcp", formatAddr(targetPort))
	if err != nil {
		log.Error().
			Err(err).
			Str("conn_id", c.ID).
			Int("target_port", targetPort).
			Msg("Stream open: failed to connect to target")
		c.sendMessage(&Message{
			Type:     MsgStreamOpenAck,
			ID:       msg.ID,
			TunnelID: msg.TunnelID,
			StreamID: msg.StreamID,
			Error:    err.Error(),
		})
		return
	}

	streamCtx, streamCancel := context.WithCancel(tunnel.ctx)
	stream := &Stream{
		ID:       msg.StreamID,
		TunnelID: tunnel.ID,
		conn:     tcpConn,
		ctx:      streamCtx,
		cancel:   streamCancel,
	}
	tunnel.addStream(stream)

	log.Debug().
		Str("conn_id", c.ID).
		Str("stream_id", msg.StreamID).
		Msg("Stream open: connection established, sending ack")

	// Send ack
	c.sendMessage(&Message{
		Type:     MsgStreamOpenAck,
		ID:       msg.ID,
		TunnelID: msg.TunnelID,
		StreamID: msg.StreamID,
	})

	// Start forwarding data from target connection to WebSocket
	go c.forwardLocalToWebSocket(tunnel, stream)
}

// forwardLocalToWebSocket forwards data from local TCP connection to WebSocket.
func (c *Conn) forwardLocalToWebSocket(tunnel *Tunnel, stream *Stream) {
	defer func() {
		log.Debug().
			Str("conn_id", c.ID).
			Str("stream_id", stream.ID).
			Msg("forwardLocalToWebSocket: stopping")
		tunnel.removeStream(stream.ID)
		c.sendMessage(&Message{
			Type:     MsgStreamClose,
			TunnelID: tunnel.ID,
			StreamID: stream.ID,
		})
	}()

	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stream.ctx.Done():
			return
		default:
		}

		n, err := stream.conn.Read(buf)
		if err != nil {
			log.Debug().
				Err(err).
				Str("stream_id", stream.ID).
				Msg("forwardLocalToWebSocket: read error")
			return
		}

		if n > 0 {
			log.Debug().
				Str("stream_id", stream.ID).
				Int("bytes", n).
				Msg("forwardLocalToWebSocket: sending data")
			data := make([]byte, n)
			copy(data, buf[:n])
			if err := c.sendMessage(&Message{
				Type:     MsgStreamData,
				TunnelID: tunnel.ID,
				StreamID: stream.ID,
				Data:     data,
			}); err != nil {
				log.Debug().
					Err(err).
					Str("stream_id", stream.ID).
					Msg("forwardLocalToWebSocket: send error")
				return
			}
		}
	}
}

// handleStreamData handles incoming stream data.
func (c *Conn) handleStreamData(msg *Message) {
	c.tunnelsMu.RLock()
	tunnel, ok := c.tunnels[msg.TunnelID]
	c.tunnelsMu.RUnlock()

	if !ok {
		log.Debug().
			Str("conn_id", c.ID).
			Str("tunnel_id", msg.TunnelID).
			Msg("handleStreamData: tunnel not found")
		return
	}

	stream, ok := tunnel.getStream(msg.StreamID)
	if !ok {
		log.Debug().
			Str("conn_id", c.ID).
			Str("stream_id", msg.StreamID).
			Msg("handleStreamData: stream not found")
		return
	}

	if len(msg.Data) > 0 {
		log.Debug().
			Str("stream_id", msg.StreamID).
			Int("bytes", len(msg.Data)).
			Msg("handleStreamData: writing to stream")
		_, err := stream.Write(msg.Data)
		if err != nil {
			log.Debug().
				Err(err).
				Str("stream_id", msg.StreamID).
				Msg("handleStreamData: write error")
			tunnel.removeStream(msg.StreamID)
		}
	}
}

// handleStreamClose handles stream close notification.
func (c *Conn) handleStreamClose(msg *Message) {
	c.tunnelsMu.RLock()
	tunnel, ok := c.tunnels[msg.TunnelID]
	c.tunnelsMu.RUnlock()

	if !ok {
		return
	}

	tunnel.removeStream(msg.StreamID)
}

// handleTunnelRemove handles a request to remove a tunnel.
func (c *Conn) handleTunnelRemove(msg *Message) {
	c.tunnelsMu.Lock()
	tunnel, ok := c.tunnels[msg.TunnelID]
	if ok {
		tunnel.Close()
		delete(c.tunnels, msg.TunnelID)
	}
	c.tunnelsMu.Unlock()

	c.tm.tunnelsMu.Lock()
	delete(c.tm.tunnels, msg.TunnelID)
	c.tm.tunnelsMu.Unlock()

	c.sendMessage(&Message{
		Type:     MsgTunnelRemoveAck,
		ID:       msg.ID,
		TunnelID: msg.TunnelID,
	})
}

// sendTunnelResponse sends a tunnel response message.
func (c *Conn) sendTunnelResponse(msgID, tunnelID string, success bool, errMsg string) {
	resp := TunnelResponse{
		TunnelID: tunnelID,
		Success:  success,
		Error:    errMsg,
	}
	payload, _ := json.Marshal(resp)
	c.sendMessage(&Message{
		Type:    MsgTunnelResponse,
		ID:      msgID,
		Payload: payload,
	})
}

// formatAddr formats a port number as a TCP address.
func formatAddr(port int) string {
	return "127.0.0.1:" + itoa(port)
}

// itoa converts an int to string.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	n := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		b[n] = '-'
	}
	return string(b[n:])
}

