package agent

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"sshole/pkg/protocol"
	"sshole/pkg/websocket"
)

// Agent Agent 模块的主要结构
type Agent struct {
	connInfo *protocol.ConnectionInfo
	conn     *websocket.WebSocketConnection
	running  bool
	mu       sync.RWMutex
}

// NewAgent 创建新的 Agent 实例
func NewAgent(connInfo *protocol.ConnectionInfo) *Agent {
	return &Agent{
		connInfo: connInfo,
	}
}

// Start 启动 Agent
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	a.running = true
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	log.Info().Str("hub_addr", a.connInfo.HubAddress).Msg("Starting agent")

	// TODO: 实现 WebSocket 连接逻辑
	// TODO: 实现 SSH shell 监听逻辑
	// TODO: 实现数据转发机制

	<-ctx.Done()
	return ctx.Err()
}

// Stop 停止 Agent
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.conn != nil {
		a.conn.Close()
	}
	a.running = false
}

// IsRunning 检查 Agent 是否正在运行
func (a *Agent) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}
