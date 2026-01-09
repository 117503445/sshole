package hub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/117503445/sshole/pkg/proto"
	"github.com/117503445/sshole/pkg/rpc/v1"
	"github.com/117503445/sshole/pkg/rpc/v1/rpcv1connect"
	"github.com/coder/websocket"
)

type rpcServer struct {
	hub *Hub
}

func (s *rpcServer) ListAgents(ctx context.Context, _ *connect.Request[rpcv1.ListAgentsRequest]) (*connect.Response[rpcv1.ListAgentsResponse], error) {
	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()

	agents := make([]*rpcv1.AgentInfo, 0, len(s.hub.agents))
	for name, state := range s.hub.agents {
		agents = append(agents, &rpcv1.AgentInfo{
			AgentName: name,
			HubPort:   int32(state.HubPort),
			Online:    state.Control != nil,
		})
	}
	return connect.NewResponse(&rpcv1.ListAgentsResponse{
		Agents: agents,
	}), nil
}

func (s *rpcServer) AppendKnownHost(ctx context.Context, req *connect.Request[rpcv1.AppendKnownHostRequest]) (*connect.Response[rpcv1.AppendKnownHostResponse], error) {
	agentName := req.Msg.GetAgentName()
	key := strings.TrimSpace(req.Msg.GetPublicKey())
	if agentName == "" || key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("agent_name and public_key required"))
	}

	s.hub.mu.RLock()
	state, ok := s.hub.agents[agentName]
	s.hub.mu.RUnlock()
	if !ok || state.Control == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("agent offline"))
	}

	msg := proto.ControlMessage{
		Type:      "ADD_KNOWN_HOST",
		KnownHost: key,
	}
	data, err := json.Marshal(&msg)
	if err != nil {
		return nil, err
	}

	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := state.Control.Write(sendCtx, websocket.MessageText, data); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&rpcv1.AppendKnownHostResponse{}), nil
}

func (h *Hub) rpcPath() string {
	path, _ := rpcv1connect.NewHoleServiceHandler(&rpcServer{hub: h}, connect.WithInterceptors(h.rpcAuthInterceptor()))
	return path
}

func (h *Hub) rpcHandler() http.Handler {
	_, handler := rpcv1connect.NewHoleServiceHandler(&rpcServer{hub: h}, connect.WithInterceptors(h.rpcAuthInterceptor()))
	return handler
}

func (h *Hub) rpcAuthInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if h.cfg.AuthToken != "" {
				auth := req.Header().Get("Authorization")
				if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.cfg.AuthToken {
					return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthorized"))
				}
			}
			return next(ctx, req)
		}
	}
}
