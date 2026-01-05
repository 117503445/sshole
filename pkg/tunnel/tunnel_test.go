package tunnel

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNewTunnelManager tests the creation of a new TunnelManager.
func TestNewTunnelManager(t *testing.T) {
	tm := NewTunnelManager("test-auth")
	if tm == nil {
		t.Fatal("NewTunnelManager returned nil")
	}
	if tm.ID == "" {
		t.Error("TunnelManager ID should not be empty")
	}
	if tm.Auth != "test-auth" {
		t.Errorf("Expected auth 'test-auth', got '%s'", tm.Auth)
	}
	if err := tm.Close(); err != nil {
		t.Errorf("Failed to close TunnelManager: %v", err)
	}
}

// TestTunnelManagerClose tests closing the TunnelManager.
func TestTunnelManagerClose(t *testing.T) {
	tm := NewTunnelManager("")
	if err := tm.Close(); err != nil {
		t.Errorf("Failed to close TunnelManager: %v", err)
	}
	// Close again should not panic
	if err := tm.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestHandlerAuthSuccess tests successful WebSocket authentication.
func TestHandlerAuthSuccess(t *testing.T) {
	tm := NewTunnelManager("secret123")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	// Convert http to ws URL and add auth
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?auth=secret123"
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "secret123")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if conn.ID == "" {
		t.Error("Connection ID should not be empty")
	}

	// Give some time for the connection to be registered
	time.Sleep(100 * time.Millisecond)

	// Check that connection is registered on server side
	serverConns := tm.ListConns()
	if len(serverConns) != 1 {
		t.Errorf("Expected 1 server connection, got %d", len(serverConns))
	}
}

// TestHandlerAuthFailure tests failed WebSocket authentication.
func TestHandlerAuthFailure(t *testing.T) {
	tm := NewTunnelManager("secret123")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	// Try to connect with wrong auth
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?auth=wrongauth"
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	_, err := clientTM.Connect(wsURL, "wrongauth")
	if err == nil {
		t.Fatal("Expected authentication to fail")
	}
}

// TestListConns tests listing connections.
func TestListConns(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	// Initially no connections
	conns := tm.ListConns()
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(conns))
	}

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Client should have 1 connection
	clientConns := clientTM.ListConns()
	if len(clientConns) != 1 {
		t.Errorf("Expected 1 client connection, got %d", len(clientConns))
	}

	time.Sleep(100 * time.Millisecond)

	// Server should have 1 connection
	serverConns := tm.ListConns()
	if len(serverConns) != 1 {
		t.Errorf("Expected 1 server connection, got %d", len(serverConns))
	}
}

// TestDisconnect tests disconnecting a connection.
func TestDisconnect(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	connID := conn.ID

	// Disconnect
	if err := clientTM.Disconnect(connID); err != nil {
		t.Errorf("Failed to disconnect: %v", err)
	}

	// Connection should be removed
	conns := clientTM.ListConns()
	if len(conns) != 0 {
		t.Errorf("Expected 0 connections after disconnect, got %d", len(conns))
	}

	// Disconnect non-existent should fail
	err = clientTM.Disconnect("non-existent-id")
	if err != ErrConnNotFound {
		t.Errorf("Expected ErrConnNotFound, got %v", err)
	}
}

// TestConnStatus tests connection status tracking.
func TestConnStatus(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Initially connected
	if !conn.IsConnected() {
		t.Error("Connection should be connected initially")
	}

	if conn.GetStatus() != StatusConnected {
		t.Errorf("Expected StatusConnected, got %v", conn.GetStatus())
	}
}

// TestConnGetters tests connection getter methods.
func TestConnGetters(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if conn.GetID() == "" {
		t.Error("GetID should return non-empty string")
	}

	if conn.GetRemoteID() == "" {
		t.Error("GetRemoteID should return non-empty string")
	}

	if conn.GetCreatedAt().IsZero() {
		t.Error("GetCreatedAt should return non-zero time")
	}

	if conn.GetLastHeartbeat().IsZero() {
		t.Error("GetLastHeartbeat should return non-zero time")
	}
}

// TestListTunnels tests listing tunnels.
func TestListTunnels(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	// Initially no tunnels
	tunnels := tm.ListTunnels()
	if len(tunnels) != 0 {
		t.Errorf("Expected 0 tunnels, got %d", len(tunnels))
	}
}

// TestAddTunnelInvalidType tests adding a tunnel with invalid type.
func TestAddTunnelInvalidType(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	
	clientTM := NewTunnelManager("")
	defer clientTM.Close()

	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	_, err = clientTM.AddTunnel(conn.ID, "invalidType", 8080, 9090)
	if err != ErrInvalidTunnelType {
		t.Errorf("Expected ErrInvalidTunnelType, got %v", err)
	}
}

// TestAddTunnelConnNotFound tests adding a tunnel with non-existent connection.
func TestAddTunnelConnNotFound(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	_, err := tm.AddTunnel("non-existent-conn", "localToRemote", 8080, 9090)
	if err != ErrConnNotFound {
		t.Errorf("Expected ErrConnNotFound, got %v", err)
	}
}

// TestRemoveTunnelNotFound tests removing a non-existent tunnel.
func TestRemoveTunnelNotFound(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	err := tm.RemoveTunnel("non-existent-tunnel")
	if err != ErrTunnelNotFound {
		t.Errorf("Expected ErrTunnelNotFound, got %v", err)
	}
}

// TestGetTunnelNotFound tests getting a non-existent tunnel.
func TestGetTunnelNotFound(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	_, err := tm.GetTunnel("non-existent-tunnel")
	if err != ErrTunnelNotFound {
		t.Errorf("Expected ErrTunnelNotFound, got %v", err)
	}
}

// TestTunnelInfo tests getting tunnel info.
func TestTunnelInfo(t *testing.T) {
	tunnel := &Tunnel{
		ID:         "test-tunnel",
		ConnID:     "test-conn",
		Type:       LocalToRemote,
		LocalPort:  8080,
		RemotePort: 9090,
		Status:     "active",
		streams:    make(map[string]*Stream),
	}

	info := tunnel.GetInfo()
	if info["id"] != "test-tunnel" {
		t.Errorf("Expected id 'test-tunnel', got %v", info["id"])
	}
	if info["type"] != LocalToRemote {
		t.Errorf("Expected type LocalToRemote, got %v", info["type"])
	}
	if info["local_port"] != 8080 {
		t.Errorf("Expected local_port 8080, got %v", info["local_port"])
	}
	if info["remote_port"] != 9090 {
		t.Errorf("Expected remote_port 9090, got %v", info["remote_port"])
	}
}

// TestConcurrentConnections tests concurrent connection handling.
func TestConcurrentConnections(t *testing.T) {
	tm := NewTunnelManager("")
	defer tm.Close()

	server := httptest.NewServer(http.HandlerFunc(tm.Handler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	const numClients = 5
	var wg sync.WaitGroup
	var mu sync.Mutex
	clientTMs := make([]*TunnelManager, 0, numClients)
	conns := make([]*Conn, 0, numClients)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			clientTM := NewTunnelManager("")
			
			conn, err := clientTM.Connect(wsURL, "")
			if err != nil {
				t.Errorf("Failed to connect: %v", err)
				clientTM.Close()
				return
			}

			mu.Lock()
			clientTMs = append(clientTMs, clientTM)
			conns = append(conns, conn)
			mu.Unlock()
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	// Server should have numClients connections
	serverConns := tm.ListConns()
	if len(serverConns) != numClients {
		t.Errorf("Expected %d server connections, got %d", numClients, len(serverConns))
	}

	// Clean up - close connections first, then TunnelManagers
	for _, conn := range conns {
		conn.Close()
	}
	for _, clientTM := range clientTMs {
		clientTM.Close()
	}
}

// TestLocalToRemoteTunnel tests LocalToRemote tunnel with actual data transfer.
func TestLocalToRemoteTunnel(t *testing.T) {
	// Start a simple echo server first
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start echo server: %v", err)
	}
	targetPort := echoListener.Addr().(*net.TCPAddr).Port
	
	echoCtx, echoCancel := context.WithCancel(context.Background())
	defer echoCancel()

	go func() {
		for {
			select {
			case <-echoCtx.Done():
				echoListener.Close()
				return
			default:
			}
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					_, err = c.Write(buf[:n])
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	// Give echo server time to start
	time.Sleep(50 * time.Millisecond)

	serverTM := NewTunnelManager("")
	defer serverTM.Close()

	server := httptest.NewServer(http.HandlerFunc(serverTM.Handler))
	defer server.Close()

	clientTM := NewTunnelManager("")

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create LocalToRemote tunnel: local port -> remote targetPort
	localPort := getFreePort(t)
	tunnelID, err := clientTM.AddTunnel(conn.ID, "localToRemote", localPort, targetPort)
	if err != nil {
		t.Fatalf("Failed to add tunnel: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify tunnel was created
	tunnels := clientTM.ListTunnels()
	if len(tunnels) == 0 {
		t.Fatal("No tunnels found")
	}

	// Connect to local port and send data
	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		t.Fatalf("Failed to connect to local port: %v", err)
	}

	testData := "Hello, Tunnel!"
	_, err = localConn.Write([]byte(testData))
	if err != nil {
		localConn.Close()
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read response
	localConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, err := localConn.Read(buf)
	localConn.Close()
	
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if string(buf[:n]) != testData {
		t.Errorf("Expected '%s', got '%s'", testData, string(buf[:n]))
	}

	// Remove tunnel
	if err := clientTM.RemoveTunnel(tunnelID); err != nil {
		t.Errorf("Failed to remove tunnel: %v", err)
	}

	// Verify tunnel was removed
	tunnels = clientTM.ListTunnels()
	if len(tunnels) != 0 {
		t.Errorf("Expected 0 tunnels after removal, got %d", len(tunnels))
	}

	// Clean up in correct order
	clientTM.Close()
	echoCancel()
}

// TestRemoteToLocalTunnel tests RemoteToLocal tunnel.
func TestRemoteToLocalTunnel(t *testing.T) {
	// Start a simple echo server first (on client side - local port)
	echoListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start echo server: %v", err)
	}
	localPort := echoListener.Addr().(*net.TCPAddr).Port
	
	echoCtx, echoCancel := context.WithCancel(context.Background())
	defer echoCancel()

	go func() {
		for {
			select {
			case <-echoCtx.Done():
				echoListener.Close()
				return
			default:
			}
			conn, err := echoListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				for {
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					_, err = c.Write(buf[:n])
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()

	// Give echo server time to start
	time.Sleep(50 * time.Millisecond)

	serverTM := NewTunnelManager("")
	defer serverTM.Close()

	server := httptest.NewServer(http.HandlerFunc(serverTM.Handler))
	defer server.Close()

	clientTM := NewTunnelManager("")

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, err := clientTM.Connect(wsURL, "")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Create RemoteToLocal tunnel: remote port -> local port
	remotePort := getFreePort(t)
	tunnelID, err := clientTM.AddTunnel(conn.ID, "remoteToLocal", localPort, remotePort)
	if err != nil {
		t.Fatalf("Failed to add tunnel: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Verify tunnel was created
	tunnels := clientTM.ListTunnels()
	if len(tunnels) == 0 {
		t.Fatal("No tunnels found on client side")
	}

	// Connect to remote port (on server side) and send data
	remoteConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", remotePort))
	if err != nil {
		t.Fatalf("Failed to connect to remote port: %v", err)
	}

	testData := "Hello, Remote Tunnel!"
	_, err = remoteConn.Write([]byte(testData))
	if err != nil {
		remoteConn.Close()
		t.Fatalf("Failed to write data: %v", err)
	}

	// Read response
	remoteConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 1024)
	n, err := remoteConn.Read(buf)
	remoteConn.Close()
	
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if string(buf[:n]) != testData {
		t.Errorf("Expected '%s', got '%s'", testData, string(buf[:n]))
	}

	// Remove tunnel
	if err := clientTM.RemoveTunnel(tunnelID); err != nil {
		t.Errorf("Failed to remove tunnel: %v", err)
	}

	// Clean up in correct order
	clientTM.Close()
	echoCancel()
}

// TestHeartbeatTimeout tests connection heartbeat timeout detection.
func TestHeartbeatTimeout(t *testing.T) {
	// This test is marked as skipped because it requires waiting for timeout
	// which would make the test too slow
	t.Skip("Skipping heartbeat timeout test - too slow for regular testing")
}

// TestStreamClose tests stream close handling.
func TestStreamClose(t *testing.T) {
	stream := &Stream{
		ID:       "test-stream",
		TunnelID: "test-tunnel",
	}

	// First close should succeed
	if err := stream.Close(); err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Second close should not panic
	if err := stream.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

// TestTunnelClose tests tunnel close handling.
func TestTunnelClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	tunnel := &Tunnel{
		ID:      "test-tunnel",
		Status:  "active",
		ctx:     ctx,
		cancel:  cancel,
		streams: make(map[string]*Stream),
	}

	// First close should succeed
	if err := tunnel.Close(); err != nil {
		t.Errorf("First close failed: %v", err)
	}

	if tunnel.Status != "stopped" {
		t.Errorf("Expected status 'stopped', got '%s'", tunnel.Status)
	}
}

// TestTunnelError tests TunnelError implementation.
func TestTunnelError(t *testing.T) {
	err := &TunnelError{Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got '%s'", err.Error())
	}
}

// TestFormatAddr tests the formatAddr function.
func TestFormatAddr(t *testing.T) {
	tests := []struct {
		port     int
		expected string
	}{
		{8080, "127.0.0.1:8080"},
		{0, "127.0.0.1:0"},
		{65535, "127.0.0.1:65535"},
	}

	for _, tt := range tests {
		result := formatAddr(tt.port)
		if result != tt.expected {
			t.Errorf("formatAddr(%d) = %s, expected %s", tt.port, result, tt.expected)
		}
	}
}

// getFreePort returns a free TCP port.
func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

