package hub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/117503445/sshole/pkg/common"
	"github.com/117503445/sshole/pkg/proto"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// WSConn wraps websocket.Conn with a write lock for safe concurrent writes.
type WSConn struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func (w *WSConn) Write(ctx context.Context, typ websocket.MessageType, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Conn.Write(ctx, typ, data)
}

func (w *WSConn) Close(status websocket.StatusCode, reason string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Conn.Close(status, reason)
}

type AgentState struct {
	Name        common.AgentName
	Control     *WSConn
	HubPort     int
	ConnectedAt time.Time
}

// Hub is the main runtime for the hub process.
type Hub struct {
	cfg HubConfig

	agents    map[string]*AgentState // name -> state
	ports     map[int]string         // hubPort -> agentName
	listeners map[int]net.Listener
	pending   map[string]*PendingSession // sessionID -> pending

	mu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

func NewHub(cfg HubConfig) (*Hub, error) {
	c := cfg.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		cfg:       c,
		agents:    map[string]*AgentState{},
		ports:     map[int]string{},
		listeners: map[int]net.Listener{},
		pending:   map[string]*PendingSession{},
		ctx:       ctx,
		cancel:    cancel,
	}

	if err := h.loadMapping(); err != nil {
		cancel()
		return nil, err
	}
	return h, nil
}

func (h *Hub) loadMapping() error {
	pm, err := LoadMapping(h.cfg.MappingFile)
	if err != nil {
		return err
	}
	for agent, port := range pm.Agents {
		h.agents[agent] = &AgentState{
			Name:    agent,
			HubPort: port,
		}
		if existing, ok := h.ports[port]; ok {
			return fmt.Errorf("duplicate hub port %d for agents %s and %s", port, existing, agent)
		}
		h.ports[port] = agent
	}
	return nil
}

// Start runs HTTP server and port listeners until context is cancelled.
func (h *Hub) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	h.ctx = ctx
	h.cancel = cancel
	defer cancel()

	if err := h.startPortListeners(ctx); err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/agent", h.handleAgentWS)
	mux.HandleFunc("/tunnel", h.handleTunnelWS)
	mux.Handle(h.rpcPath(), h.rpcHandler())

	server := &http.Server{
		Addr:    h.cfg.HTTPAddr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", h.cfg.HTTPAddr).Msg("hub listening")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		_ = server.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		return err
	}
}

func (h *Hub) startPortListeners(ctx context.Context) error {
	for port, agent := range h.ports {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return fmt.Errorf("listen hubPort %d: %w", port, err)
		}
		h.listeners[port] = ln
		log.Info().Int("port", port).Str("agent", agent).Msg("hubPort listening")
		go h.servePort(ctx, ln, agent)
	}
	return nil
}

func (h *Hub) servePort(ctx context.Context, ln net.Listener, agent string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			log.Error().Err(err).Msg("accept ssh connection")
			continue
		}
		go h.onSSHConn(ctx, agent, conn)
	}
}

func (h *Hub) onSSHConn(ctx context.Context, agentName string, sshConn net.Conn) {
	h.mu.RLock()
	state, ok := h.agents[agentName]
	h.mu.RUnlock()
	if !ok || state.Control == nil {
		log.Warn().Str("agent", agentName).Msg("agent offline, closing ssh conn")
		sshConn.Close()
		return
	}

	sessionID := uuid.New().String()
	now := time.Now()
	p := &PendingSession{
		SessionID: sessionID,
		AgentName: agentName,
		SSHConn:   sshConn,
		State:     PendingINIT,
		CreatedAt: now,
		Deadline:  now.Add(h.cfg.PendingTimeout),
	}

	h.mu.Lock()
	h.pending[sessionID] = p
	h.mu.Unlock()

	if err := h.sendOpen(state.Control, sessionID); err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("send OPEN failed")
		h.cleanupPending(sessionID, PendingCLOSED)
		return
	}

	h.mu.Lock()
	p.State = PendingOPEN_SENT
	h.mu.Unlock()

	go h.watchPendingTimeout(sessionID)
}

func (h *Hub) sendOpen(conn *WSConn, sessionID string) error {
	msg := proto.ControlMessage{
		Type:      "OPEN",
		SessionID: sessionID,
	}
	data, err := json.Marshal(&msg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, data)
}

func (h *Hub) watchPendingTimeout(sessionID string) {
	timer := time.NewTimer(h.cfg.PendingTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		h.mu.RLock()
		p, ok := h.pending[sessionID]
		h.mu.RUnlock()
		if !ok || p.State == PendingBOUND || p.State == PendingCLOSED {
			return
		}
		log.Warn().Str("session", sessionID).Msg("pending session timeout")
		h.cleanupPending(sessionID, PendingTIMEOUT)
	case <-h.ctx.Done():
		return
	}
}

func (h *Hub) cleanupPending(sessionID string, newState PendingState) {
	h.mu.Lock()
	p, ok := h.pending[sessionID]
	if ok {
		p.State = newState
		if p.SSHConn != nil {
			p.SSHConn.Close()
		}
		if p.Tunnel != nil {
			p.Tunnel.Close(websocket.StatusNormalClosure, "closed")
		}
		delete(h.pending, sessionID)
	}
	h.mu.Unlock()
}

func (h *Hub) bindTunnel(sessionID string, ws *websocket.Conn) (*PendingSession, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	p, ok := h.pending[sessionID]
	if !ok {
		return nil, fmt.Errorf("%s: %s", common.ErrSessionNotFound, sessionID)
	}
	if p.State != PendingOPEN_SENT || p.Tunnel != nil {
		return nil, fmt.Errorf("%s: %s", common.ErrDuplicateTunnel, sessionID)
	}
	p.Tunnel = ws
	p.State = PendingBOUND

	// For entry-initiated sessions, don't start forwarding here - let waitForAgentAndForward handle it
	if p.EntryWS == nil {
		// SSH-initiated session: start forwarding between SSH conn and tunnel
		log.Info().Str("session", sessionID).Msg("SSH-initiated session: starting forwarding between SSH and tunnel")
		h.startForwarding(p)
	} else {
		log.Info().Str("session", sessionID).Msg("entry-initiated session: agent tunnel bound, will forward entry<->agent")
		// For entry-initiated sessions, the waitForAgentAndForward goroutine will handle forwarding
	}

	return p, nil
}

func (h *Hub) startForwarding(p *PendingSession) {
	ctx := h.ctx
	wsConn := tunnel.NetConn(ctx, p.Tunnel)
	sshConn := p.SSHConn

	closeOnce := sync.Once{}
	cleanup := func() {
		closeOnce.Do(func() {
			sshConn.Close()
			wsConn.Close()
			h.cleanupPending(p.SessionID, PendingCLOSED)
		})
	}

	go func() {
		_, err := ioCopy(wsConn, sshConn)
		if err != nil {
			log.Debug().Err(err).Msg("ssh->ws copy ended")
		}
		cleanup()
	}()
	go func() {
		_, err := ioCopy(sshConn, wsConn)
		if err != nil {
			log.Debug().Err(err).Msg("ws->ssh copy ended")
		}
		cleanup()
	}()
}

// findAvailablePort finds an available port starting from 10000.
func (h *Hub) findAvailablePort() int {
	port := 10000
	for {
		if _, exists := h.ports[port]; !exists {
			return port
		}
		port++
	}
}

// saveMapping saves the current agent mappings to disk.
func (h *Hub) saveMapping() error {
	pm := &PortMapping{Agents: make(map[string]int)}
	for agent, state := range h.agents {
		pm.Agents[agent] = state.HubPort
	}
	return SaveMapping(h.cfg.MappingFile, pm)
}

// waitForAgentAndForward waits for agent tunnel to connect, then forwards between entry and agent WebSockets.
func (h *Hub) waitForAgentAndForward(sessionID string, entryWS *websocket.Conn) {
	// Wait for the pending session to be bound (agent connects)
	timeout := time.NewTimer(h.cfg.PendingTimeout)
	defer timeout.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			log.Warn().Str("session", sessionID).Msg("timeout waiting for agent tunnel")
			entryWS.Close(websocket.StatusInternalError, "timeout waiting for agent")
			h.cleanupPending(sessionID, PendingTIMEOUT)
			return
		case <-ticker.C:
			h.mu.RLock()
			p, ok := h.pending[sessionID]
			h.mu.RUnlock()
			if !ok {
				log.Warn().Str("session", sessionID).Msg("pending session disappeared")
				entryWS.Close(websocket.StatusInternalError, "session error")
				return
			}
			if p.State == PendingBOUND && p.Tunnel != nil {
				// Agent tunnel is ready, start forwarding between entry and agent WebSockets
				log.Info().Str("session", sessionID).Msg("agent tunnel ready, starting entry-agent forwarding")
				h.startWebSocketForwarding(entryWS, p.Tunnel)
				return
			}
		}
	}
}

// startWebSocketForwarding forwards data between two WebSocket connections.
func (h *Hub) startWebSocketForwarding(ws1, ws2 *websocket.Conn) {
	wsConn1 := tunnel.NetConn(h.ctx, ws1)
	wsConn2 := tunnel.NetConn(h.ctx, ws2)

	closeOnce := sync.Once{}
	cleanup := func() {
		closeOnce.Do(func() {
			wsConn1.Close()
			wsConn2.Close()
		})
	}

	go func() {
		defer cleanup()
		_, err := ioCopy(wsConn1, wsConn2)
		if err != nil {
			log.Debug().Err(err).Msg("ws2->ws1 copy ended")
		}
	}()

	go func() {
		defer cleanup()
		_, err := ioCopy(wsConn2, wsConn1)
		if err != nil {
			log.Debug().Err(err).Msg("ws1->ws2 copy ended")
		}
	}()
}

// ioCopy wraps io.Copy with deadlines to ensure timely shutdown.
func ioCopy(dst net.Conn, src net.Conn) (int64, error) {
	return io.Copy(dst, src)
}
