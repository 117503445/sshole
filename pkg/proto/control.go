package proto

// ControlMessage is the only control plane JSON message on /agent.
type ControlMessage struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id,omitempty"`
	KnownHost string `json:"known_host,omitempty"`
}

// AgentStatus 是 Agent 在控制连接上周期上报的状态帧（Agent → Hub）。
//
// 之前控制连接只有 Hub 往 Agent 发帧，Hub 只知道"TCP 还在"，于是机器关机后
// ListAgents 仍然长期显示 online，Entry 只能拿到一个没有信息量的握手 EOF。
// 状态帧让 Hub 第一次能回答三个运维问题：这台机器跑的是哪个构建、本机 SSH
// 到底通不通、最后一次真正说话是什么时候。
type AgentStatus struct {
	// Type 固定为 STATUS，用于和将来其他 Agent → Hub 帧区分。
	Type string `json:"type"`
	// Version 是构建期注入的版本标识（GitVersion）。
	Version string `json:"version,omitempty"`
	// Commit 是构建提交短哈希。
	Commit string `json:"commit,omitempty"`
	// BuildTime 是构建时间。
	BuildTime string `json:"build_time,omitempty"`
	// Hostname 是 Agent 所在机器的主机名，用于确认"这到底是哪台机器"。
	Hostname string `json:"hostname,omitempty"`
	// SSHPort 是内嵌 sshd 监听的本机端口。
	SSHPort int `json:"ssh_port,omitempty"`
	// SSHReady 表示此刻能连上本机 SSH 端口。
	SSHReady bool `json:"ssh_ready"`
	// SSLError 是最近一次 SSH 不可用的原因，就绪时为空。
	SSLError string `json:"ssh_error,omitempty"`
}
