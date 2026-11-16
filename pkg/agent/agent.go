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
	"os"
	"strings"
	"sync"
	"time"

	"github.com/117503445/sshole/internal/buildinfo"
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

	// 内嵌 sshd 的当前可清理句柄与最近一次失败原因，由 sshMu 保护，
	// 只用于状态上报和重启，不影响控制连接。
	sshMu      sync.Mutex
	sshCleanup func()
	sshErr     string
}

// 控制连接建立后 Agent 侧完全空闲，NAT 和反向代理会在几十秒内把它掐断，
// 所以定期 ping Hub 保活；ping 失败说明链路已坏，直接关闭连接触发重连。
const (
	controlKeepaliveInterval = 20 * time.Second
	controlPingTimeout       = 10 * time.Second
)

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
		a.setSSHCleanup(cleanup)
		defer a.closeSSHCleanup()
		// 内嵌 sshd 一旦退出，控制连接仍然健康，Hub 会一直以为这台机器可用，
		// 所以这里必须看护它并在监听消失时重启。
		a.watchSSHD(a.ctx, a.cfg.LocalPort)
	} else {
		log.Info().Int("port", a.cfg.LocalPort).Msg("skip sshd, assuming existing SSH server")
		cleanup = func() {}
	}

	attempt := 0
	for {
		if err := a.connectControl(); err != nil {
			delay := a.cfg.retryBackoff(attempt)
			attempt++
			log.Warn().Err(err).Dur("backoff", delay).Msg("control connect failed, retrying")
			// 设备侧网络抖动、Hub 重启都不应该让调试 Agent 永久退出，所以无限重连。
			if !sleepWithContext(ctx, delay) {
				return ctx.Err()
			}
			continue
		}
		attempt = 0 // reset after successful connect
		if err := a.readControl(); err != nil {
			log.Warn().Err(err).Msg("control connection closed, reconnecting")
		}
	}
}

// sleepWithContext 等待退避时间，ctx 提前结束时返回 false。
func sleepWithContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
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
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	go a.reportControlStatus(ctx, a.control)

	for {
		msgType, data, err := a.control.Read(ctx)
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

// reportControlStatus 在 ctx 结束前立即并周期地向 Hub 发一帧状态，替代原来只保活的 ping。
//
// 状态帧同时承担保活作用；写失败说明链路已坏，直接关闭连接触发重连。
func (a *Agent) reportControlStatus(ctx context.Context, ws *websocket.Conn) {
	ticker := time.NewTicker(controlKeepaliveInterval)
	defer ticker.Stop()

	for {
		if err := a.sendStatus(ctx, ws); err != nil {
			log.Warn().Err(err).Msg("control status failed, reconnecting")
			_ = ws.Close(websocket.StatusTryAgainLater, "status failed")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// sendStatus 组装并发送一帧 Agent 状态，包含构建标识、主机名和本机 SSH 就绪情况。
// 参数 ctx：控制连接生命周期；参数 ws：当前控制连接。
func (a *Agent) sendStatus(ctx context.Context, ws *websocket.Conn) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	ready, reason := a.sshProbe()
	status := proto.AgentStatus{
		Type:      "STATUS",
		Version:   buildinfo.GitVersion,
		Commit:    buildinfo.GitCommit,
		BuildTime: buildinfo.BuildTime,
		Hostname:  hostname,
		SSHPort:   a.cfg.LocalPort,
		SSHReady:  ready,
		SSLError:  reason,
	}
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	writeCtx, cancel := context.WithTimeout(ctx, controlPingTimeout)
	defer cancel()
	return ws.Write(writeCtx, websocket.MessageText, data)
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
