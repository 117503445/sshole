package tunnel

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/coder/websocket"
)

const (
	HandshakeMagicSize = 8
	HandshakeSIDSize   = 16
	HandshakeSize      = HandshakeMagicSize + HandshakeSIDSize
)

var HandshakeMagic = [HandshakeMagicSize]byte{'S', 'S', 'H', 'O', 'L', 'E', '0', '1'}

// HandshakeHeader is the fixed-length binary header sent as the first frame on /tunnel.
type HandshakeHeader struct {
	Magic   [HandshakeMagicSize]byte
	Session [HandshakeSIDSize]byte
}

// SessionIDTo16Bytes converts a session ID string into a deterministic 16-byte value.
// If the session ID looks like a UUID, the raw bytes are used; otherwise a sha256 hash is truncated.
func SessionIDTo16Bytes(sessionID string) [16]byte {
	var out [16]byte
	// Try parse as hex UUID without dashes first
	if len(sessionID) == 32 {
		if b, err := hex.DecodeString(sessionID); err == nil && len(b) == 16 {
			copy(out[:], b)
			return out
		}
	}
	// Remove dashes if present
	clean := make([]byte, 0, len(sessionID))
	for i := 0; i < len(sessionID); i++ {
		if sessionID[i] != '-' {
			clean = append(clean, sessionID[i])
		}
	}
	if len(clean) == 32 {
		if b, err := hex.DecodeString(string(clean)); err == nil && len(b) == 16 {
			copy(out[:], b)
			return out
		}
	}

	// Fallback: sha256 truncate
	sum := sha256.Sum256([]byte(sessionID))
	copy(out[:], sum[:HandshakeSIDSize])
	return out
}

// BuildHandshake builds the fixed header bytes.
func BuildHandshake(sessionID string) []byte {
	h := HandshakeHeader{
		Magic:   HandshakeMagic,
		Session: SessionIDTo16Bytes(sessionID),
	}
	buf := make([]byte, HandshakeSize)
	copy(buf[0:HandshakeMagicSize], h.Magic[:])
	copy(buf[HandshakeMagicSize:], h.Session[:])
	return buf
}

// ValidateHandshake validates the first binary frame read from the websocket against the expected session.
func ValidateHandshake(data []byte, expectedSessionID string) error {
	if len(data) != HandshakeSize {
		return fmt.Errorf("invalid handshake size: %d", len(data))
	}
	if !equalBytes(data[:HandshakeMagicSize], HandshakeMagic[:]) {
		return errors.New("invalid handshake magic")
	}
	expected := SessionIDTo16Bytes(expectedSessionID)
	if !equalBytes(data[HandshakeMagicSize:], expected[:]) {
		return errors.New("session mismatch")
	}
	return nil
}

// SendHandshake writes the handshake frame as the first binary message.
func SendHandshake(ctx context.Context, ws *websocket.Conn, sessionID string) error {
	return ws.Write(ctx, websocket.MessageBinary, BuildHandshake(sessionID))
}

// ReadHandshake reads and validates the first binary message.
func ReadHandshake(ctx context.Context, ws *websocket.Conn, expectedSessionID string) error {
	msgType, data, err := ws.Read(ctx)
	if err != nil {
		return err
	}
	if msgType != websocket.MessageBinary {
		return fmt.Errorf("expected binary handshake, got %v", msgType)
	}
	if err := ValidateHandshake(data, expectedSessionID); err != nil {
		return err
	}
	return nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// EncodeUint64 is a tiny helper used in tests to make binary framing explicit.
func EncodeUint64(v uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return buf
}
