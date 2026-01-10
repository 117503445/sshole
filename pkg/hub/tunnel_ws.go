package hub

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

func (h *Hub) handleTunnelWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentName := r.Header.Get("X-Agent")
	sessionID := r.Header.Get("X-Session")
	if agentName == "" {
		http.Error(w, "missing X-Agent header", http.StatusBadRequest)
		return
	}
	if h.cfg.AuthToken != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.cfg.AuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	h.mu.RLock()
	state, ok := h.agents[agentName]
	h.mu.RUnlock()
	if !ok {
		http.Error(w, "unknown agent", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Error().Err(err).Msg("accept tunnel failed")
		return
	}

	// Read handshake to get session ID if not provided in header
	var actualSessionID string
	if sessionID == "" {
		// Read handshake to extract session ID
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			log.Warn().Err(err).Msg("read handshake failed")
			ws.Close(websocket.StatusPolicyViolation, "handshake failed")
			return
		}
		if msgType != websocket.MessageBinary {
			log.Warn().Msg("expected binary handshake")
			ws.Close(websocket.StatusPolicyViolation, "expected binary handshake")
			return
		}
		if len(data) != tunnel.HandshakeSize {
			log.Warn().Int("size", len(data)).Msg("invalid handshake size")
			ws.Close(websocket.StatusPolicyViolation, "invalid handshake size")
			return
		}
		// Extract session ID from handshake
		sessionBytes := data[tunnel.HandshakeMagicSize:]
		actualSessionID = fmt.Sprintf("%x", sessionBytes)
	} else {
		actualSessionID = sessionID
		if err := tunnel.ReadHandshake(ctx, ws, sessionID); err != nil {
			log.Warn().Err(err).Str("session", sessionID).Msg("handshake failed")
			ws.Close(websocket.StatusPolicyViolation, "handshake failed")
			return
		}
	}

	// Check if this is an agent tunnel (has pending session) or entry tunnel (create pending session)
	h.mu.RLock()
	p, hasPending := h.pending[actualSessionID]
	h.mu.RUnlock()

	if hasPending {
		// Check what type of pending session this is
		if p.EntryWS != nil {
			// This is an agent responding to an entry-initiated session
			// The entry WebSocket is waiting in waitForAgentAndForward
			log.Info().Str("session", actualSessionID).Str("agent", agentName).Msg("agent tunnel bound for entry-initiated session")
			// Just mark as bound - waitForAgentAndForward will handle the forwarding
			h.mu.Lock()
			p.Tunnel = ws
			p.State = PendingBOUND
			h.mu.Unlock()
		} else {
			// Agent tunnel responding to SSH-initiated session
			p, err := h.bindTunnel(actualSessionID, ws)
			if err != nil {
				log.Warn().Err(err).Str("session", actualSessionID).Msg("bind tunnel failed")
				ws.Close(websocket.StatusPolicyViolation, err.Error())
				return
			}
			if p.AgentName != agentName {
				log.Warn().Str("session", actualSessionID).Str("agent", agentName).Msg("session agent mismatch")
				ws.Close(websocket.StatusPolicyViolation, string(common.ErrSessionMismatch))
				return
			}

			log.Info().Str("session", actualSessionID).Str("agent", agentName).Msg("agent tunnel bound for SSH-initiated session, starting forward")
			h.startForwarding(p)
		}
	} else {
		// Entry tunnel: create a pending session and send OPEN to agent
		log.Info().Str("session", actualSessionID).Str("agent", agentName).Msg("entry tunnel connected, creating pending session")

		// Create a pending session as if SSH connection came in
		now := time.Now()
		p := &PendingSession{
			SessionID: actualSessionID,
			AgentName: agentName,
			EntryWS:   ws, // Mark this as entry-initiated
			State:     PendingINIT,
			CreatedAt: now,
			Deadline:  now.Add(h.cfg.PendingTimeout),
		}

		h.mu.Lock()
		h.pending[actualSessionID] = p
		h.mu.Unlock()

		// Send OPEN to agent
		if err := h.sendOpen(state.Control, actualSessionID); err != nil {
			log.Error().Err(err).Str("session", actualSessionID).Msg("send OPEN failed")
			h.cleanupPending(actualSessionID, PendingCLOSED)
			ws.Close(websocket.StatusInternalError, "failed to open session")
			return
		}

		h.mu.Lock()
		p.State = PendingOPEN_SENT
		h.mu.Unlock()

		// Wait for agent to connect and bind
		go h.waitForAgentAndForward(actualSessionID, ws)
		go h.watchPendingTimeout(actualSessionID)
	}
}
