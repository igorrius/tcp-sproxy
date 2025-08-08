package entities

import (
	"net"
	"testing"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
)

func TestNewConnection(t *testing.T) {
	// Setup
	id := valueobjects.NewConnectionID()
	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	serverAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}

	// Execute
	conn := NewConnection(id, clientAddr, serverAddr)

	// Verify
	if conn == nil {
		t.Fatal("NewConnection returned nil")
	}

	if !conn.ID.Equals(id) {
		t.Errorf("Expected connection ID %s, got %s", id.String(), conn.ID.String())
	}

	if conn.ClientAddr.String() != clientAddr.String() {
		t.Errorf("Expected client address %s, got %s", clientAddr.String(), conn.ClientAddr.String())
	}

	if conn.ServerAddr.String() != serverAddr.String() {
		t.Errorf("Expected server address %s, got %s", serverAddr.String(), conn.ServerAddr.String())
	}

	if conn.Status != ConnectionStatusNew {
		t.Errorf("Expected status %s, got %s", ConnectionStatusNew, conn.Status)
	}

	// CreatedAt and LastActivity should be set to current time (approximately)
	now := time.Now()
	if conn.CreatedAt.After(now) || conn.CreatedAt.Before(now.Add(-time.Second)) {
		t.Errorf("CreatedAt time is not within expected range: %v", conn.CreatedAt)
	}

	if conn.LastActivity.After(now) || conn.LastActivity.Before(now.Add(-time.Second)) {
		t.Errorf("LastActivity time is not within expected range: %v", conn.LastActivity)
	}
}

func TestConnection_UpdateActivity(t *testing.T) {
	// Setup
	conn := createTestConnection()
	oldLastActivity := conn.LastActivity

	// Wait a small amount of time to ensure the timestamp changes
	time.Sleep(10 * time.Millisecond)

	// Execute
	conn.UpdateActivity()

	// Verify
	if !conn.LastActivity.After(oldLastActivity) {
		t.Errorf("LastActivity was not updated. Old: %v, New: %v", oldLastActivity, conn.LastActivity)
	}
}

func TestConnection_SetStatus(t *testing.T) {
	// Setup
	conn := createTestConnection()

	// Execute
	conn.SetStatus(ConnectionStatusActive)

	// Verify
	if conn.Status != ConnectionStatusActive {
		t.Errorf("Expected status %s, got %s", ConnectionStatusActive, conn.Status)
	}

	// Execute again with different status
	conn.SetStatus(ConnectionStatusClosed)

	// Verify
	if conn.Status != ConnectionStatusClosed {
		t.Errorf("Expected status %s, got %s", ConnectionStatusClosed, conn.Status)
	}
}

func TestConnection_GetStatus(t *testing.T) {
	// Setup
	conn := createTestConnection()
	conn.Status = ConnectionStatusActive

	// Execute
	status := conn.GetStatus()

	// Verify
	if status != ConnectionStatusActive {
		t.Errorf("Expected status %s, got %s", ConnectionStatusActive, status)
	}
}

func TestConnection_IsActive(t *testing.T) {
	// Setup
	conn := createTestConnection()

	// Test when not active
	conn.SetStatus(ConnectionStatusNew)
	if conn.IsActive() {
		t.Error("IsActive() should return false for ConnectionStatusNew")
	}

	// Test when active
	conn.SetStatus(ConnectionStatusActive)
	if !conn.IsActive() {
		t.Error("IsActive() should return true for ConnectionStatusActive")
	}

	// Test other statuses
	conn.SetStatus(ConnectionStatusClosed)
	if conn.IsActive() {
		t.Error("IsActive() should return false for ConnectionStatusClosed")
	}

	conn.SetStatus(ConnectionStatusError)
	if conn.IsActive() {
		t.Error("IsActive() should return false for ConnectionStatusError")
	}
}

func TestConnection_IsClosed(t *testing.T) {
	// Setup
	conn := createTestConnection()

	// Test when not closed
	conn.SetStatus(ConnectionStatusNew)
	if conn.IsClosed() {
		t.Error("IsClosed() should return false for ConnectionStatusNew")
	}

	// Test when closed
	conn.SetStatus(ConnectionStatusClosed)
	if !conn.IsClosed() {
		t.Error("IsClosed() should return true for ConnectionStatusClosed")
	}

	// Test other statuses
	conn.SetStatus(ConnectionStatusActive)
	if conn.IsClosed() {
		t.Error("IsClosed() should return false for ConnectionStatusActive")
	}

	conn.SetStatus(ConnectionStatusError)
	if conn.IsClosed() {
		t.Error("IsClosed() should return false for ConnectionStatusError")
	}
}

func TestConnection_Close(t *testing.T) {
	// Setup
	conn := createTestConnection()
	conn.SetStatus(ConnectionStatusActive)

	// Execute
	conn.Close()

	// Verify
	if !conn.IsClosed() {
		t.Error("Connection should be closed after calling Close()")
	}

	if conn.Status != ConnectionStatusClosed {
		t.Errorf("Expected status %s, got %s", ConnectionStatusClosed, conn.Status)
	}
}

func TestConnection_GetDuration(t *testing.T) {
	// Setup
	conn := createTestConnection()

	// Set CreatedAt to a specific time in the past
	conn.CreatedAt = time.Now().Add(-5 * time.Second)

	// Execute
	duration := conn.GetDuration()

	// Verify
	if duration < 5*time.Second || duration > 6*time.Second {
		t.Errorf("Expected duration around 5 seconds, got %v", duration)
	}
}

func TestConnection_GetIdleTime(t *testing.T) {
	// Setup
	conn := createTestConnection()

	// Set LastActivity to a specific time in the past
	conn.LastActivity = time.Now().Add(-3 * time.Second)

	// Execute
	idleTime := conn.GetIdleTime()

	// Verify
	if idleTime < 3*time.Second || idleTime > 4*time.Second {
		t.Errorf("Expected idle time around 3 seconds, got %v", idleTime)
	}
}

// Helper function to create a test connection
func createTestConnection() *Connection {
	id := valueobjects.NewConnectionID()
	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	serverAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
	return NewConnection(id, clientAddr, serverAddr)
}
