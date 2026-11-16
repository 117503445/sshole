package hub

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/117503445/sshole/pkg/proto"
)

// agentStatusView 是 /api/agents 的单条输出，字段面向运维排查，不承诺长期稳定。
type agentStatusView struct {
	// AgentName 是 Agent 在 Hub 登记的名称。
	AgentName string `json:"agent_name"`
	// HubPort 是该 Agent 的入口端口。
	HubPort int `json:"hub_port"`
	// Online 表示 Hub 当前持有这条控制连接。
	Online bool `json:"online"`
	// LastSeenMS 是最后一次证明控制连接活着的时间；为 0 表示从未证明过。
	LastSeenMS int64 `json:"last_seen_ms"`
	// StatusReported 表示该 Agent 会上报状态帧（旧版本为 false，相关字段为空）。
	StatusReported bool `json:"status_reported"`
	// Hostname 是 Agent 所在机器的主机名。
	Hostname string `json:"hostname,omitempty"`
	// Version 是 Agent 的构建版本标识。
	Version string `json:"version,omitempty"`
	// Commit 是 Agent 的构建提交。
	Commit string `json:"commit,omitempty"`
	// BuildTime 是 Agent 的构建时间。
	BuildTime string `json:"build_time,omitempty"`
	// SSHPort 是 Agent 本机 SSH 端口。
	SSHPort int `json:"ssh_port,omitempty"`
	// SSHReady 表示 Agent 本机 SSH 此刻可连接。
	SSHReady bool `json:"ssh_ready"`
	// SSLError 是最近一次本机 SSH 不可用的原因。
	SSLError string `json:"ssh_error,omitempty"`
}

// touchAgent 记录某条控制连接最后一次被证明活着的时间。
// 参数 name：Agent 名称。
func (h *Hub) touchAgent(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if state, ok := h.agents[name]; ok {
		state.LastSeen = time.Now()
	}
}

// recordAgentStatus 解析 Agent 上报的状态帧；无法识别的帧只当作保活，不覆盖已有状态。
// 参数 name：Agent 名称；参数 data：控制连接上收到的一帧文本。
func (h *Hub) recordAgentStatus(name string, data []byte) {
	var status proto.AgentStatus
	if err := json.Unmarshal(data, &status); err != nil || status.Type != "STATUS" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	state, ok := h.agents[name]
	if !ok {
		return
	}
	copied := status
	state.Status = &copied
	state.LastSeen = time.Now()
}

// agentViewLocked 需要在持有读锁时调用，组装单个 Agent 的运维视图。
// 参数 name：Agent 名称；参数 state：Agent 状态。
func agentViewLocked(name string, state *AgentState) agentStatusView {
	view := agentStatusView{
		AgentName:  name,
		HubPort:    state.HubPort,
		Online:     state.Control != nil,
		LastSeenMS: state.LastSeen.UnixMilli(),
	}
	if status := state.Status; status != nil {
		view.StatusReported = true
		view.Hostname = status.Hostname
		view.Version = status.Version
		view.Commit = status.Commit
		view.BuildTime = status.BuildTime
		view.SSHPort = status.SSHPort
		view.SSHReady = status.SSHReady
		view.SSLError = status.SSLError
	}
	return view
}

// handleAgentsAPI 用 JSON 返回全部 Agent 的在线与本机 SSH 就绪情况。
//
// 有了它，"连不上" 不再只能靠猜：机器关机（online=false 或 last_seen 停住）、
// Agent 在跑但本机 SSH 没起来（online=true 且 ssh_ready=false）、
// 以及现场跑的是哪个构建，都能直接查出来。
// 参数 w：响应；参数 r：请求，需带与 RPC 相同的 Bearer token。
func (h *Hub) handleAgentsAPI(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AuthToken != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != h.cfg.AuthToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	h.mu.RLock()
	views := make([]agentStatusView, 0, len(h.agents))
	for name, state := range h.agents {
		views = append(views, agentViewLocked(name, state))
	}
	h.mu.RUnlock()
	sort.Slice(views, func(i, j int) bool { return views[i].AgentName < views[j].AgentName })
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"agents": views}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
