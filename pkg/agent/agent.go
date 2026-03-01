package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/117503445/sshole/pkg/proto"
	"github.com/117503445/sshole/pkg/tunnel"
	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
)

// Agent implements the runtime for the agent process.
type Agent struct {
	cfg     AgentConfig
	control *websocket.Conn
	ctx     context.Context
	cancel  context.CancelFunc
}

func New(cfg AgentConfig) *Agent {
	c := cfg.withDefaults()
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		cfg:    c,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start blocks until fatal error or context cancellation.
func (a *Agent) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.ctx = ctx
	a.cancel = cancel
	defer cancel()

	var cleanup func()
	if !a.cfg.SkipSSHD {
		var err error
		cleanup, err = a.ensureSSHD(a.ctx, a.cfg.LocalPort)
		if err != nil {
			return err
		}
		defer cleanup()
	} else {
		log.Info().Int("port", a.cfg.LocalPort).Msg("skip sshd, assuming existing SSH server")
		cleanup = func() {}
	}

	attempt := 0
	for {
		if err := a.connectControl(); err != nil {
			attempt++
			if attempt >= a.cfg.Timeouts.AgentReconnectMaxRetries {
				return fmt.Errorf("control connect failed after %d attempts: %w", attempt, err)
			}
			delay := a.cfg.retryBackoff(attempt)
			log.Warn().Err(err).Dur("backoff", delay).Msg("control connect failed, retrying")
			time.Sleep(delay)
			continue
		}
		attempt = 0 // reset after successful connect
		if err := a.readControl(); err != nil {
			log.Warn().Err(err).Msg("control connection closed, reconnecting")
			continue
		}
	}
}

func (a *Agent) controlURL() (string, error) {
	u, err := url.Parse(a.cfg.HubURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/agent"
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String(), nil
}

func (a *Agent) tunnelURL() (string, error) {
	u, err := url.Parse(a.cfg.HubURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/tunnel"
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String(), nil
}

func (a *Agent) connectControl() error {
	url, err := a.controlURL()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.Timeouts.TunnelDialTimeout)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: httpHeader(a.cfg.Token, a.cfg.AgentName, ""),
	})
	if err != nil {
		return err
	}
	a.control = ws
	log.Info().Msg("control connected")
	return nil
}

func (a *Agent) readControl() error {
	for {
		msgType, data, err := a.control.Read(a.ctx)
		if err != nil {
			return err
		}
		if msgType != websocket.MessageText {
			a.control.Close(websocket.StatusUnsupportedData, "text only")
			return errors.New("received non-text control frame")
		}
		var msg proto.ControlMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Warn().Err(err).Msg("invalid control message")
			continue
		}
		if msg.Type == "OPEN" && msg.SessionID != "" {
			go a.handleOpen(msg.SessionID)
		} else if msg.Type == "ADD_KNOWN_HOST" && msg.KnownHost != "" {
			if err := appendAuthorizedKey(msg.KnownHost); err != nil {
				log.Warn().Err(err).Msg("append authorized_key failed")
			}
		}
	}
}

func (a *Agent) handleOpen(sessionID string) {
	ctx, cancel := context.WithTimeout(a.ctx, a.cfg.Timeouts.TunnelDialTimeout)
	defer cancel()

	tunnelURL, err := a.tunnelURL()
	if err != nil {
		log.Error().Err(err).Msg("build tunnel url")
		return
	}

	ws, _, err := websocket.Dial(ctx, tunnelURL, &websocket.DialOptions{
		HTTPHeader: httpHeader(a.cfg.Token, a.cfg.AgentName, sessionID),
	})
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("dial tunnel failed")
		return
	}

	if err := tunnel.SendHandshake(ctx, ws, sessionID); err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("send handshake failed")
		ws.Close(websocket.StatusPolicyViolation, "handshake failed")
		return
	}

	local, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", a.cfg.LocalPort), a.cfg.Timeouts.TunnelDialTimeout)
	if err != nil {
		log.Error().Err(err).Str("session", sessionID).Msg("dial local sshd failed")
		ws.Close(websocket.StatusInternalError, "local dial failed")
		return
	}

	netConn := tunnel.NetConn(a.ctx, ws)
	closeOnce := sync.Once{}
	cleanup := func() {
		closeOnce.Do(func() {
			netConn.Close()
			local.Close()
		})
	}

	go func() {
		_, err := ioCopy(netConn, local)
		if err != nil {
			log.Debug().Err(err).Msg("local->ws copy ended")
		}
		cleanup()
	}()
	_, err = ioCopy(local, netConn)
	if err != nil {
		log.Debug().Err(err).Msg("ws->local copy ended")
	}
	cleanup()
}

func httpHeader(token, agentName, sessionID string) http.Header {
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	if agentName != "" {
		h.Set("X-Agent", agentName)
	}
	if sessionID != "" {
		h.Set("X-Session", sessionID)
	}
	return h
}

// ioCopy wraps io.Copy to simplify unit testing injection if needed.
var ioCopy = func(dst net.Conn, src net.Conn) (int64, error) {
	return io.Copy(dst, src)
}
