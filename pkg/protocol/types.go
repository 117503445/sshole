package protocol

import (
	"time"
)

// MessageType 定义消息类型
type MessageType string

const (
	// 连接建立相关
	MessageTypeHandshake MessageType = "handshake"
	MessageTypeAuth      MessageType = "auth"

	// 数据传输相关
	MessageTypeData MessageType = "data"
	MessageTypeAck  MessageType = "ack"

	// 控制相关
	MessageTypeHeartbeat MessageType = "heartbeat"
	MessageTypeClose     MessageType = "close"
	MessageTypeError     MessageType = "error"
)

// Message WebSocket 消息结构
type Message struct {
	Type      MessageType `json:"type"`
	ID        string      `json:"id"`
	SessionID string      `json:"session_id,omitempty"`
	Payload   []byte      `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// HandshakeMessage 握手消息
type HandshakeMessage struct {
	ClientType string `json:"client_type"` // "agent", "entry"
	ClientID   string `json:"client_id"`
	Version    string `json:"version"`
}

// AuthMessage 认证消息
type AuthMessage struct {
	Token string `json:"token"`
}

// DataMessage 数据消息
type DataMessage struct {
	Data []byte `json:"data"`
}

// HeartbeatMessage 心跳消息
type HeartbeatMessage struct {
	Sequence int64 `json:"sequence"`
}

// ErrorMessage 错误消息
type ErrorMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	HubAddress string `json:"hub_address"`
	Token      string `json:"token,omitempty"`
	ClientID   string `json:"client_id,omitempty"`
}
