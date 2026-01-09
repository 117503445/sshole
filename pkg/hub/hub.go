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

// ioCopy wraps io.Copy with deadlines to ensure timely shutdown.
func ioCopy(dst net.Conn, src net.Conn) (int64, error) {
	return io.Copy(dst, src)
}
