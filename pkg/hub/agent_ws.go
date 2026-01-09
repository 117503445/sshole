package hub

import (
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

func (h *Hub) handleAgentWS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log.Info().Msg("handleAgentWS")
	agentName := r.Header.Get("X-Agent")
	if agentName == "" {
		http.Error(w, "missing X-Agent", http.StatusBadRequest)
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
	_, ok := h.agents[agentName]
	h.mu.RUnlock()
	if !ok {
		http.Error(w, "unknown agent", http.StatusForbidden)
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Error().Err(err).Msg("accept agent ws failed")
		return
	}
	wsConn := &WSConn{Conn: ws}

	h.mu.Lock()
	state := h.agents[agentName]
	state.Control = wsConn
	state.ConnectedAt = time.Now()
	h.mu.Unlock()

	log.Info().Str("agent", agentName).Msg("agent control connected")

	// Read loop: only allow Text frames; on close remove control.
	for {
		msgType, _, err := ws.Read(ctx)
		if err != nil {
			break
		}
		if msgType != websocket.MessageText {
			ws.Close(websocket.StatusUnsupportedData, "text only")
			break
		}
		// control plane is one-way; ignore payload
	}

	h.mu.Lock()
	state = h.agents[agentName]
	if state.Control != nil && state.Control.Conn == ws {
		state.Control = nil
	}
	h.mu.Unlock()

	ws.Close(websocket.StatusNormalClosure, "")
	log.Info().Str("agent", agentName).Msg("agent control disconnected")
}
