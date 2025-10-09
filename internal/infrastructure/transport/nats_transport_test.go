package transport

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

// TestNewNATSTransport tests creating a new NATS transport
func TestNewNATSTransport(t *testing.T) {
	// Setup
	logger := logrus.NewEntry(logrus.New())

	// Execute
	tr := NewNATSTransport(logger)

	// Verify
	if tr == nil {
		t.Fatal("NewNATSTransport returned nil")
	}

	if tr.subs == nil {
		t.Error("subs map not initialized")
	}

	if tr.subChans == nil {
		t.Error("subChans map not initialized")
	}
}

// TestNATSTransport_Connect tests connecting to NATS
func TestNATSTransport_Connect(t *testing.T) {
	// Skip this test as it requires a real NATS server
	t.Skip("Skipping test that requires a real NATS server")
}

// TestNATSTransport_Disconnect tests disconnecting from NATS
func TestNATSTransport_Disconnect(t *testing.T) {
	// Setup
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)

	// Test when not connected
	tr.conn = nil
	err := tr.Disconnect(context.Background())
	if err != nil {
		t.Errorf("Disconnect should not return error when not connected: %v", err)
	}
}

// TestNATSTransport_Send tests sending a message
func TestNATSTransport_Send(t *testing.T) {
	// Skip this test as it requires a real NATS connection
	t.Skip("Skipping test that requires a real NATS connection")
}

// TestNATSTransport_Subscribe tests subscribing to a subject
func TestNATSTransport_Subscribe(t *testing.T) {
	// Skip this test as it requires a real NATS connection
	t.Skip("Skipping test that requires a real NATS connection")
}

// TestNATSTransport_Request tests sending a request and waiting for a response
func TestNATSTransport_Request(t *testing.T) {
	// Skip this test as it requires a real NATS connection
	t.Skip("Skipping test that requires a real NATS connection")
}

// TestNATSTransport_Publish tests publishing a message
func TestNATSTransport_Publish(t *testing.T) {
	// Skip this test as it requires a real NATS connection
	t.Skip("Skipping test that requires a real NATS connection")
}

// TestNATSTransport_Close tests closing the transport
func TestNATSTransport_Close(t *testing.T) {
	// Skip this test as it requires a real NATS connection
	t.Skip("Skipping test that requires a real NATS connection")
}

// TestNATSTransport_IsConnected tests checking if the transport is connected
func TestNATSTransport_IsConnected(t *testing.T) {
	// Setup
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)

	// Test when not connected
	if tr.IsConnected() {
		t.Error("IsConnected should return false when not connected")
	}
}

// TestNATSConn_SetDeadline tests setting both read and write deadlines
func TestNATSConn_SetDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	deadline := time.Now().Add(5 * time.Second)
	err := conn.SetDeadline(deadline)
	if err != nil {
		t.Errorf("SetDeadline returned error: %v", err)
	}
	
	conn.mu.Lock()
	if !conn.readDeadline.Equal(deadline) {
		t.Errorf("Read deadline not set correctly: expected %v, got %v", deadline, conn.readDeadline)
	}
	if !conn.writeDeadline.Equal(deadline) {
		t.Errorf("Write deadline not set correctly: expected %v, got %v", deadline, conn.writeDeadline)
	}
	conn.mu.Unlock()
}

// TestNATSConn_SetReadDeadline tests setting read deadline
func TestNATSConn_SetReadDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	deadline := time.Now().Add(5 * time.Second)
	err := conn.SetReadDeadline(deadline)
	if err != nil {
		t.Errorf("SetReadDeadline returned error: %v", err)
	}
	
	conn.mu.Lock()
	if !conn.readDeadline.Equal(deadline) {
		t.Errorf("Read deadline not set correctly: expected %v, got %v", deadline, conn.readDeadline)
	}
	conn.mu.Unlock()
}

// TestNATSConn_SetWriteDeadline tests setting write deadline
func TestNATSConn_SetWriteDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	deadline := time.Now().Add(5 * time.Second)
	err := conn.SetWriteDeadline(deadline)
	if err != nil {
		t.Errorf("SetWriteDeadline returned error: %v", err)
	}
	
	conn.mu.Lock()
	if !conn.writeDeadline.Equal(deadline) {
		t.Errorf("Write deadline not set correctly: expected %v, got %v", deadline, conn.writeDeadline)
	}
	conn.mu.Unlock()
}

// TestNATSConn_ReadWithDeadline tests reading with a deadline
func TestNATSConn_ReadWithDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	// Test timeout
	deadline := time.Now().Add(100 * time.Millisecond)
	err := conn.SetReadDeadline(deadline)
	if err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	// Check if it's a timeout error
	if netErr, ok := err.(*net.OpError); ok {
		if netErr.Op != "read" {
			t.Errorf("Expected Op to be 'read', got: %s", netErr.Op)
		}
	} else {
		t.Errorf("Expected net.OpError, got: %T - %v", err, err)
	}
}

// TestNATSConn_ReadWithPastDeadline tests reading when deadline already passed
func TestNATSConn_ReadWithPastDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	// Set deadline in the past
	deadline := time.Now().Add(-1 * time.Second)
	err := conn.SetReadDeadline(deadline)
	if err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

// TestNATSConn_ReadSuccessBeforeDeadline tests successful read before deadline
func TestNATSConn_ReadSuccessBeforeDeadline(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	tr := NewNATSTransport(logger)
	
	conn := &NATSConn{
		transport:  tr,
		id:         "test-conn",
		readCh:     make(chan []byte, 10),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
	}
	
	// Set deadline far in the future
	deadline := time.Now().Add(10 * time.Second)
	err := conn.SetReadDeadline(deadline)
	if err != nil {
		t.Fatalf("SetReadDeadline failed: %v", err)
	}
	
	// Send data to readCh
	testData := []byte("test data")
	go func() {
		conn.readCh <- testData
	}()
	
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		t.Errorf("Read failed: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Expected to read %d bytes, got %d", len(testData), n)
	}
	if string(buffer[:n]) != string(testData) {
		t.Errorf("Expected data %q, got %q", testData, buffer[:n])
	}
}

