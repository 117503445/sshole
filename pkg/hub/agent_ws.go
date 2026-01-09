package hub

import (
	"fmt"
	"net"
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
		// Agent doesn't exist, add it with an available port
		h.mu.Lock()
		port := h.findAvailablePort()
		h.agents[agentName] = &AgentState{
			Name:    agentName,
			HubPort: port,
		}
		h.ports[port] = agentName

		// Start listener for the new port
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			h.mu.Unlock()
			log.Error().Err(err).Int("port", port).Msg("failed to start listener for new agent")
			http.Error(w, "failed to start listener", http.StatusInternalServerError)
			return
		}
		h.listeners[port] = ln
		h.mu.Unlock()

		go h.servePort(h.ctx, ln, agentName)

		// Persist the mapping
		if err := h.saveMapping(); err != nil {
			log.Error().Err(err).Str("agent", agentName).Msg("failed to save mapping")
			http.Error(w, "failed to persist agent", http.StatusInternalServerError)
			return
		}

		log.Info().Str("agent", agentName).Int("port", port).Msg("agent added and persisted")
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
