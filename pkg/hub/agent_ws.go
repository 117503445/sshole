package hub

import (
	"context"
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
	state.LastSeen = time.Now()
	h.mu.Unlock()

	log.Info().Str("agent", agentName).Msg("agent control connected")

	// 机器直接关机时 TCP 不会发 FIN，只靠读循环会永远以为对端还在，
	// 所以 Hub 必须主动探测；pong 回不来就立刻判定离线。
	go h.keepaliveAgent(ctx, agentName, ws)

	// Read loop: only allow Text frames; on close remove control.
	for {
		msgType, data, err := ws.Read(ctx)
		if err != nil {
			// 把断开原因留在 Hub 日志里，否则事后只能看到 connected 却不知道为什么会掉线。
			code := websocket.CloseStatus(err)
			event := log.Warn()
			message := "agent control read failed"
			if code == websocket.StatusNormalClosure || code == websocket.StatusGoingAway {
				event = log.Info()
				message = "agent control closed"
			}
			event.Err(err).Str("agent", agentName).Int("close_status", int(code)).Msg(message)
			break
		}
		if msgType != websocket.MessageText {
			ws.Close(websocket.StatusUnsupportedData, "text only")
			break
		}
		// Agent 侧唯一的主动帧是 STATUS；其他内容只当保活。
		h.recordAgentStatus(agentName, data)
	}

	h.markAgentOffline(agentName, ws)

	ws.Close(websocket.StatusNormalClosure, "")
	log.Info().Str("agent", agentName).Msg("agent control disconnected")
}

// markAgentOffline 清除该 Agent 的控制连接，只有仍持有同一条连接时才清除，
// 避免把重连后的新连接误删。
//
// 探测失败时必须先把状态改成离线再去关连接：coder/websocket 的本地 Close 不会立刻
// 打断阻塞中的 Read，如果只依赖读循环收尾，机器已经掉电的 Agent 会继续被报成在线。
// 参数 name：Agent 名称；参数 ws：本次结束的控制连接。
func (h *Hub) markAgentOffline(name string, ws *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.agents[name]
	if ok && state.Control != nil && state.Control.Conn == ws {
		state.Control = nil
	}
}

// keepaliveAgent 周期探测 Agent 控制连接，探测失败立即关闭连接并标记离线。
// 参数 ctx：本次控制连接的生命周期；参数 name：Agent 名称；参数 ws：控制连接。
func (h *Hub) keepaliveAgent(ctx context.Context, name string, ws *websocket.Conn) {
	ticker := time.NewTicker(h.cfg.AgentPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		pingCtx, cancel := context.WithTimeout(ctx, h.cfg.AgentPingTimeout)
		err := ws.Ping(pingCtx)
		cancel()
		if err != nil {
			log.Warn().Err(err).Str("agent", name).Msg("agent liveness probe failed, marking offline")
			h.markAgentOffline(name, ws)
			_ = ws.Close(websocket.StatusTryAgainLater, "liveness probe failed")
			return
		}
		h.touchAgent(name)
	}
}
