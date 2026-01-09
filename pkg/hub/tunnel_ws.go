package hub

import (
	"net/http"
	"strings"

	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

func (h *Hub) handleTunnelWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agentName := r.Header.Get("X-Agent")
	sessionID := r.Header.Get("X-Session")
	if agentName == "" || sessionID == "" {
		http.Error(w, "missing headers", http.StatusBadRequest)
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
	if state.Control == nil {
		http.Error(w, "agent offline", http.StatusServiceUnavailable)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Error().Err(err).Msg("accept tunnel failed")
		return
	}

	if err := tunnel.ReadHandshake(ctx, ws, sessionID); err != nil {
		log.Warn().Err(err).Str("session", sessionID).Msg("handshake failed")
		ws.Close(websocket.StatusPolicyViolation, "handshake failed")
		return
	}

	p, err := h.bindTunnel(sessionID, ws)
	if err != nil {
		log.Warn().Err(err).Str("session", sessionID).Msg("bind tunnel failed")
		ws.Close(websocket.StatusPolicyViolation, err.Error())
		return
	}
	if p.AgentName != agentName {
		log.Warn().Str("session", sessionID).Str("agent", agentName).Msg("session agent mismatch")
		ws.Close(websocket.StatusPolicyViolation, string(common.ErrSessionMismatch))
		return
	}

	log.Info().Str("session", sessionID).Str("agent", agentName).Msg("tunnel bound, starting forward")
	h.startForwarding(p)
}
