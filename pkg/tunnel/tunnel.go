package tunnel

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// AddTunnel creates a new tunnel on the specified connection.
// tunnelType should be "localToRemote" or "remoteToLocal".
// Returns the tunnel ID on success.
func (tm *TunnelManager) AddTunnel(connID string, tunnelType string, localPort, remotePort int) (string, error) {
	conn, err := tm.getConn(connID)
	if err != nil {
		return "", err
	}

	tt := TunnelType(tunnelType)
	if tt != LocalToRemote && tt != RemoteToLocal {
		return "", ErrInvalidTunnelType
	}

	var tunnelID string

	switch tt {
	case LocalToRemote:
		// Local listens, remote forwards to target
		tunnelID, err = tm.createLocalToRemoteTunnel(conn, localPort, remotePort)
	case RemoteToLocal:
		// Remote listens, local forwards to target
		tunnelID, err = tm.createRemoteToLocalTunnel(conn, localPort, remotePort)
	}

	if err != nil {
		return "", err
	}

	return tunnelID, nil
}

// createLocalToRemoteTunnel creates a tunnel that listens locally and forwards to remote.
func (tm *TunnelManager) createLocalToRemoteTunnel(conn *Conn, localPort, remotePort int) (string, error) {
	tunnelID := uuid.New().String()

	// Create local listener
	listener, err := net.Listen("tcp", formatAddr(localPort))
	if err != nil {
		return "", err
	}

	tunnel := &Tunnel{
		ID:         tunnelID,
		ConnID:     conn.ID,
		Type:       LocalToRemote,
		LocalPort:  localPort,
		RemotePort: remotePort,
		Status:     "active",
		listener:   listener,
		streams:    make(map[string]*Stream),
	}
	tunnel.ctx, tunnel.cancel = context.WithCancel(conn.ctx)

	// Request remote to create corresponding endpoint
	reqPayload, _ := json.Marshal(&TunnelRequest{
		TunnelType: LocalToRemote,
		LocalPort:  localPort,
		RemotePort: remotePort,
	})

	resp, err := conn.sendRequest(&Message{
		Type:    MsgTunnelRequest,
		Payload: reqPayload,
	}, 30*time.Second)

	if err != nil {
		listener.Close()
		return "", err
	}

	var tunnelResp TunnelResponse
	if err := json.Unmarshal(resp.Payload, &tunnelResp); err != nil {
		listener.Close()
		return "", err
	}

	if !tunnelResp.Success {
		listener.Close()
		if tunnelResp.Error != "" {
			return "", &TunnelError{Message: tunnelResp.Error}
		}
		return "", &TunnelError{Message: "failed to create tunnel"}
	}

	// Use the remote's tunnel ID for consistency
	tunnel.ID = tunnelResp.TunnelID
	tunnelID = tunnelResp.TunnelID

	conn.tunnelsMu.Lock()
	conn.tunnels[tunnelID] = tunnel
	conn.tunnelsMu.Unlock()

	tm.tunnelsMu.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.tunnelsMu.Unlock()

	// Start accepting local connections
	go tm.acceptLocalConnections(conn, tunnel)

	log.Info().
		Str("tunnel_id", tunnelID).
		Int("local_port", localPort).
		Int("remote_port", remotePort).
		Msg("LocalToRemote tunnel created")

	return tunnelID, nil
}

// createRemoteToLocalTunnel creates a tunnel that listens remotely and forwards to local.
func (tm *TunnelManager) createRemoteToLocalTunnel(conn *Conn, localPort, remotePort int) (string, error) {
	tunnelID := uuid.New().String()

	// Request remote to listen on the specified port
	reqPayload, _ := json.Marshal(&TunnelRequest{
		TunnelType: RemoteToLocal,
		LocalPort:  localPort,
		RemotePort: remotePort,
	})

	resp, err := conn.sendRequest(&Message{
		Type:    MsgTunnelRequest,
		Payload: reqPayload,
	}, 30*time.Second)

	if err != nil {
		return "", err
	}

	var tunnelResp TunnelResponse
	if err := json.Unmarshal(resp.Payload, &tunnelResp); err != nil {
		return "", err
	}

	if !tunnelResp.Success {
		if tunnelResp.Error != "" {
			return "", &TunnelError{Message: tunnelResp.Error}
		}
		return "", &TunnelError{Message: "failed to create tunnel"}
	}

	// Use the remote's tunnel ID
	tunnelID = tunnelResp.TunnelID

	tunnel := &Tunnel{
		ID:         tunnelID,
		ConnID:     conn.ID,
		Type:       RemoteToLocal,
		LocalPort:  localPort,
		RemotePort: remotePort,
		Status:     "active",
		streams:    make(map[string]*Stream),
	}
	tunnel.ctx, tunnel.cancel = context.WithCancel(conn.ctx)

	conn.tunnelsMu.Lock()
	conn.tunnels[tunnelID] = tunnel
	conn.tunnelsMu.Unlock()

	tm.tunnelsMu.Lock()
	tm.tunnels[tunnelID] = tunnel
	tm.tunnelsMu.Unlock()

	log.Info().
		Str("tunnel_id", tunnelID).
		Int("local_port", localPort).
		Int("remote_port", remotePort).
		Msg("RemoteToLocal tunnel created")

	return tunnelID, nil
}

// acceptLocalConnections accepts TCP connections on the local listener.
func (tm *TunnelManager) acceptLocalConnections(conn *Conn, tunnel *Tunnel) {
	for {
		select {
		case <-tunnel.ctx.Done():
			return
		default:
		}

		tcpConn, err := tunnel.listener.Accept()
		if err != nil {
			if tunnel.ctx.Err() != nil {
				return
			}
			log.Error().Err(err).Str("tunnel_id", tunnel.ID).Msg("Failed to accept local connection")
			continue
		}

		// Create stream for this connection
		streamID := uuid.New().String()
		streamCtx, streamCancel := context.WithCancel(tunnel.ctx)
		stream := &Stream{
			ID:       streamID,
			TunnelID: tunnel.ID,
			conn:     tcpConn,
			ctx:      streamCtx,
			cancel:   streamCancel,
		}
		tunnel.addStream(stream)

		// Open stream on remote and start forwarding
		go tm.handleLocalToRemoteStream(conn, tunnel, stream)
	}
}

// handleLocalToRemoteStream handles forwarding from local connection to remote.
func (tm *TunnelManager) handleLocalToRemoteStream(conn *Conn, tunnel *Tunnel, stream *Stream) {
	defer func() {
		log.Debug().Str("stream_id", stream.ID).Msg("handleLocalToRemoteStream: stopping")
		tunnel.removeStream(stream.ID)
		conn.sendMessage(&Message{
			Type:     MsgStreamClose,
			TunnelID: tunnel.ID,
			StreamID: stream.ID,
		})
	}()

	// Request stream open on remote
	log.Debug().Str("stream_id", stream.ID).Str("tunnel_id", tunnel.ID).Msg("handleLocalToRemoteStream: requesting stream open")
	reqPayload, _ := json.Marshal(&StreamOpenRequest{TunnelID: tunnel.ID})
	resp, err := conn.sendRequest(&Message{
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

	log.Debug().Str("stream_id", stream.ID).Msg("handleLocalToRemoteStream: stream opened, starting forwarding")

	// Start forwarding local data to WebSocket
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stream.ctx.Done():
			return
		default:
		}

		n, err := stream.conn.Read(buf)
		if err != nil {
			log.Debug().Err(err).Str("stream_id", stream.ID).Msg("handleLocalToRemoteStream: read error")
			return
		}

		if n > 0 {
			log.Debug().Str("stream_id", stream.ID).Int("bytes", n).Msg("handleLocalToRemoteStream: sending data")
			data := make([]byte, n)
			copy(data, buf[:n])
			if err := conn.sendMessage(&Message{
				Type:     MsgStreamData,
				TunnelID: tunnel.ID,
				StreamID: stream.ID,
				Data:     data,
			}); err != nil {
				log.Debug().Err(err).Str("stream_id", stream.ID).Msg("handleLocalToRemoteStream: send error")
				return
			}
		}
	}
}

// RemoveTunnel removes a tunnel by ID.
func (tm *TunnelManager) RemoveTunnel(tunnelID string) error {
	tm.tunnelsMu.Lock()
	tunnel, ok := tm.tunnels[tunnelID]
	if !ok {
		tm.tunnelsMu.Unlock()
		return ErrTunnelNotFound
	}
	delete(tm.tunnels, tunnelID)
	tm.tunnelsMu.Unlock()

	// Get connection to notify remote
	conn, err := tm.getConn(tunnel.ConnID)
	if err == nil {
		// Send remove request to remote
		conn.sendRequest(&Message{
			Type:     MsgTunnelRemove,
			TunnelID: tunnelID,
		}, 5*time.Second)

		conn.tunnelsMu.Lock()
		delete(conn.tunnels, tunnelID)
		conn.tunnelsMu.Unlock()
	}

	return tunnel.Close()
}

// ListTunnels returns all tunnels.
func (tm *TunnelManager) ListTunnels() []*Tunnel {
	tm.tunnelsMu.RLock()
	defer tm.tunnelsMu.RUnlock()

	tunnels := make([]*Tunnel, 0, len(tm.tunnels))
	for _, tunnel := range tm.tunnels {
		tunnels = append(tunnels, tunnel)
	}
	return tunnels
}

// GetTunnel retrieves a tunnel by ID.
func (tm *TunnelManager) GetTunnel(tunnelID string) (*Tunnel, error) {
	tm.tunnelsMu.RLock()
	defer tm.tunnelsMu.RUnlock()

	if tunnel, ok := tm.tunnels[tunnelID]; ok {
		return tunnel, nil
	}
	return nil, ErrTunnelNotFound
}

// TunnelError represents a tunnel-related error.
type TunnelError struct {
	Message string
}

func (e *TunnelError) Error() string {
	return e.Message
}

// GetInfo returns information about the tunnel.
func (t *Tunnel) GetInfo() map[string]interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.streamsMu.RLock()
	streamCount := len(t.streams)
	t.streamsMu.RUnlock()

	return map[string]interface{}{
		"id":           t.ID,
		"conn_id":      t.ConnID,
		"type":         t.Type,
		"local_port":   t.LocalPort,
		"remote_port":  t.RemotePort,
		"status":       t.Status,
		"stream_count": streamCount,
	}
}
