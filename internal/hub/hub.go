package hub

import (
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

// Hub Hub 模块的主要结构
type Hub struct {
	connections map[string]*ClientConnection
	mu          sync.RWMutex
	running     bool
}

// ClientConnection 客户端连接
type ClientConnection struct {
	ID         string
	Conn       *websocket.Conn
	ClientType string // "agent" or "entry"
	SessionID  string
}

// NewHub 创建新的 Hub 实例
func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*ClientConnection),
		running:     true,
	}
}

// HandleWebSocket 处理 WebSocket 连接
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// TODO: 实现 WebSocket 升级
	// TODO: 实现连接管理
	// TODO: 实现消息路由

	log.Info().Msg("WebSocket connection received")
}

// HandleHealth 健康检查接口
func (h *Hub) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Stop 停止 Hub
func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.running = false

	// 关闭所有连接
	for id, conn := range h.connections {
		conn.Conn.Close(websocket.StatusGoingAway, "Hub shutting down")
		delete(h.connections, id)
	}
}

// IsRunning 检查 Hub 是否正在运行
func (h *Hub) IsRunning() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.running
}
