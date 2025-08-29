package entry

import (
	"context"
	"net"
	"sync"

	"github.com/rs/zerolog/log"
	"sshole/pkg/protocol"
	"sshole/pkg/websocket"
)

// Entry Entry 模块的主要结构
type Entry struct {
	connInfo   *protocol.ConnectionInfo
	localAddr  string
	listener   net.Listener
	conn       *websocket.WebSocketConnection
	running    bool
	mu         sync.RWMutex
}

// NewEntry 创建新的 Entry 实例
func NewEntry(connInfo *protocol.ConnectionInfo, localAddr string) *Entry {
	return &Entry{
		connInfo:  connInfo,
		localAddr: localAddr,
	}
}

// Start 启动 Entry
func (e *Entry) Start(ctx context.Context) error {
	logger := log.Ctx(ctx)

	e.mu.Lock()
	e.running = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	logger.Info().Str("local_addr", e.localAddr).Msg("Starting entry")

	// 启动本地 SSH 服务器
	if err := e.startLocalServer(ctx); err != nil {
		return err
	}

	// TODO: 实现与 Hub 的 WebSocket 连接
	// TODO: 实现 SSH 协议处理
	// TODO: 实现数据转发机制

	<-ctx.Done()
	return ctx.Err()
}

// startLocalServer 启动本地 SSH 服务器
func (e *Entry) startLocalServer(ctx context.Context) error {
	logger := log.Ctx(ctx)

	var err error
	e.listener, err = net.Listen("tcp", e.localAddr)
	if err != nil {
		return err
	}

	logger.Info().Str("local_addr", e.localAddr).Msg("Local SSH server listening")

			go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					conn, err := e.listener.Accept()
					if err != nil {
						logger.Error().Err(err).Msg("Accept error")
						continue
					}

					go e.handleSSHConnection(ctx, conn)
				}
			}
		}()

	return nil
}

// handleSSHConnection 处理 SSH 连接
func (e *Entry) handleSSHConnection(ctx context.Context, conn net.Conn) {
	logger := log.Ctx(ctx)

	defer conn.Close()

	// TODO: 实现 SSH 协议处理
	logger.Info().Str("remote_addr", conn.RemoteAddr().String()).Msg("New SSH connection")
}

// Stop 停止 Entry
func (e *Entry) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.listener != nil {
		e.listener.Close()
	}

	if e.conn != nil {
		e.conn.Close()
	}

	e.running = false
}

// IsRunning 检查 Entry 是否正在运行
func (e *Entry) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}
