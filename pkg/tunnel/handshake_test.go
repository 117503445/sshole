package tunnel

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
)

func TestSessionIDTo16BytesDeterministic(t *testing.T) {
	id := "abc-123"
	b1 := SessionIDTo16Bytes(id)
	b2 := SessionIDTo16Bytes(id)
	if b1 != b2 {
		t.Fatalf("expected deterministic bytes")
	}
}

func TestValidateHandshake(t *testing.T) {
	sid := "123e4567-e89b-12d3-a456-426614174000"
	data := BuildHandshake(sid)
	if err := ValidateHandshake(data, sid); err != nil {
		t.Fatalf("expected handshake valid: %v", err)
	}
	if err := ValidateHandshake(data, "other"); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestSendAndReadHandshake(t *testing.T) {
	sid := "sid-1"
	ctx := context.Background()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen not permitted: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatalf("accept: %v", err)
		}
		defer ws.Close(websocket.StatusNormalClosure, "")
		if err := ReadHandshake(ctx, ws, sid); err != nil {
			t.Fatalf("validate handshake: %v", err)
		}
	}))
	server.Listener = ln
	server.Start()
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	ws, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	if err := SendHandshake(ctx, ws, sid); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
}
