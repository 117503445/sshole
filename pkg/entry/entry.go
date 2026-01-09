package entry

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	"github.com/117503445/sshole/pkg/agent"
	"github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/rs/zerolog/log"
)

type EntryConfig struct {
	HubAddr       string
	Token         string
	AgentName     string
	EntryPort     int
	PublicKeyPath string
}

type Entry struct {
	cfg EntryConfig
}

func New(cfg EntryConfig) *Entry {
	if cfg.EntryPort == 0 {
		cfg.EntryPort = 22222
	}
	return &Entry{cfg: cfg}
}

func (e *Entry) Start(ctx context.Context) error {
	agent, err := e.pickAgent(ctx)
	if err != nil {
		return err
	}
	pubKey, err := e.readPublicKey()
	if err != nil {
		return err
	}
	if err := e.appendKnownHost(ctx, agent.AgentName, pubKey); err != nil {
		log.Warn().Err(err).Msg("failed to push known_hosts to agent")
	}
	if err := ensureKnownHost(e.cfg.EntryPort); err != nil {
		log.Warn().Err(err).Msg("failed to append known_hosts")
	}
	listenAddr := fmt.Sprintf(":%d", e.cfg.EntryPort)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	log.Info().Str("addr", listenAddr).Str("agent", agent.AgentName).Int32("hub_port", agent.HubPort).Msg("entry listening")

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go e.handleConn(ctx, conn, int(agent.HubPort))
	}
}

func (e *Entry) pickAgent(ctx context.Context) (*rpcv1.AgentInfo, error) {
	client := rpcv1connect.NewHoleServiceClient(httpClient, e.cfg.HubAddr)
	req := connect.NewRequest(&rpcv1.ListAgentsRequest{})
	if e.cfg.Token != "" {
		req.Header().Set("Authorization", "Bearer "+e.cfg.Token)
	}
	resp, err := client.ListAgents(ctx, req)
	if err != nil {
		return nil, err
	}
	var chosen *rpcv1.AgentInfo
	if e.cfg.AgentName == "" {
		if len(resp.Msg.Agents) == 1 {
			chosen = resp.Msg.Agents[0]
		} else {
			return nil, fmt.Errorf("multiple agents available, specify AGENT_NAME")
		}
	} else {
		for _, a := range resp.Msg.Agents {
			if a.AgentName == e.cfg.AgentName {
				chosen = a
				break
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("agent %s not found", e.cfg.AgentName)
		}
	}
	return chosen, nil
}

func (e *Entry) appendKnownHost(ctx context.Context, agentName, pubKey string) error {
	client := rpcv1connect.NewHoleServiceClient(httpClient, e.cfg.HubAddr)
	req := connect.NewRequest(&rpcv1.AppendKnownHostRequest{
		AgentName: agentName,
		PublicKey: pubKey,
	})
	if e.cfg.Token != "" {
		req.Header().Set("Authorization", "Bearer "+e.cfg.Token)
	}
	_, err := client.AppendKnownHost(ctx, req)
	return err
}

func (e *Entry) handleConn(ctx context.Context, local net.Conn, hubPort int) {
	defer local.Close()
	targetHost, err := hostFromURL(e.cfg.HubAddr)
	if err != nil {
		log.Error().Err(err).Msg("parse hub host failed")
		return
	}
	remote, err := net.Dial("tcp", fmt.Sprintf("%s:%d", targetHost, hubPort))
	if err != nil {
		log.Error().Err(err).Msg("dial hub failed")
		return
	}
	defer remote.Close()

	go func() {
		_, _ = ioCopy(remote, local)
		remote.Close()
	}()
	_, _ = ioCopy(local, remote)
}

func hostFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return u.Hostname(), nil
}

var httpClient = http.DefaultClient

var ioCopy = func(dst net.Conn, src net.Conn) (int64, error) {
	return io.Copy(dst, src)
}

func (e *Entry) readPublicKey() (string, error) {
	path := e.cfg.PublicKeyPath
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func ensureKnownHost(entryPort int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	knownHosts := filepath.Join(sshDir, "known_hosts")
	entry := fmt.Sprintf("[localhost]:%d %s\n", entryPort, agent.HostPublicKey())
	if data, err := os.ReadFile(knownHosts); err == nil && strings.Contains(string(data), entry) {
		return nil
	}
	f, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(entry)
	return err
}
