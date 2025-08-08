package repositories

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
)

func TestNewInMemoryConnectionRepository(t *testing.T) {
	// Execute
	repo := NewInMemoryConnectionRepository()

	// Verify
	if repo == nil {
		t.Fatal("NewInMemoryConnectionRepository returned nil")
	}

	if repo.connections == nil {
		t.Error("connections map not initialized")
	}
}

func TestInMemoryConnectionRepository_Store(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()
	conn := createTestConnection()

	// Execute
	err := repo.Store(context.Background(), conn)

	// Verify
	if err != nil {
		t.Errorf("Store returned error: %v", err)
	}

	// Verify connection was stored
	storedConn, exists := repo.connections[conn.ID.String()]
	if !exists {
		t.Error("Connection was not stored")
	}
	if storedConn != conn {
		t.Error("Stored connection is not the same as the original")
	}

	// Test storing a connection with the same ID
	err = repo.Store(context.Background(), conn)
	if err == nil {
		t.Error("Store should return error when connection with same ID already exists")
	}
}

func TestInMemoryConnectionRepository_GetByID(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()
	conn := createTestConnection()
	_ = repo.Store(context.Background(), conn)

	// Execute - existing connection
	retrievedConn, err := repo.GetByID(context.Background(), conn.ID)

	// Verify
	if err != nil {
		t.Errorf("GetByID returned error: %v", err)
	}
	if retrievedConn != conn {
		t.Error("Retrieved connection is not the same as the original")
	}

	// Execute - non-existing connection
	nonExistingID, _ := valueobjects.NewConnectionIDFromString("non-existing")
	_, err = repo.GetByID(context.Background(), nonExistingID)

	// Verify
	if err == nil {
		t.Error("GetByID should return error for non-existing connection")
	}
}

func TestInMemoryConnectionRepository_GetActive(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()

	// Create connections with different statuses
	activeConn1 := createTestConnection()
	activeConn1.SetStatus(entities.ConnectionStatusActive)
	_ = repo.Store(context.Background(), activeConn1)

	activeConn2 := createTestConnection()
	activeConn2.SetStatus(entities.ConnectionStatusActive)
	_ = repo.Store(context.Background(), activeConn2)

	closedConn := createTestConnection()
	closedConn.SetStatus(entities.ConnectionStatusClosed)
	_ = repo.Store(context.Background(), closedConn)

	// Execute
	activeConns, err := repo.GetActive(context.Background())

	// Verify
	if err != nil {
		t.Errorf("GetActive returned error: %v", err)
	}

	if len(activeConns) != 2 {
		t.Errorf("Expected 2 active connections, got %d", len(activeConns))
	}

	// Verify all returned connections are active
	for _, conn := range activeConns {
		if !conn.IsActive() {
			t.Error("GetActive returned a non-active connection")
		}
	}
}

func TestInMemoryConnectionRepository_GetAll(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()

	// Create some connections
	conn1 := createTestConnection()
	_ = repo.Store(context.Background(), conn1)

	conn2 := createTestConnection()
	_ = repo.Store(context.Background(), conn2)

	conn3 := createTestConnection()
	_ = repo.Store(context.Background(), conn3)

	// Execute
	allConns, err := repo.GetAll(context.Background())

	// Verify
	if err != nil {
		t.Errorf("GetAll returned error: %v", err)
	}

	if len(allConns) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(allConns))
	}
}

func TestInMemoryConnectionRepository_Update(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()
	conn := createTestConnection()
	_ = repo.Store(context.Background(), conn)

	// Modify connection
	conn.SetStatus(entities.ConnectionStatusClosed)

	// Execute
	err := repo.Update(context.Background(), conn)

	// Verify
	if err != nil {
		t.Errorf("Update returned error: %v", err)
	}

	// Verify connection was updated
	updatedConn, _ := repo.GetByID(context.Background(), conn.ID)
	if updatedConn.GetStatus() != entities.ConnectionStatusClosed {
		t.Errorf("Expected status %s, got %s", entities.ConnectionStatusClosed, updatedConn.GetStatus())
	}

	// Test updating a non-existing connection
	nonExistingConn := createTestConnection()
	err = repo.Update(context.Background(), nonExistingConn)
	if err == nil {
		t.Error("Update should return error for non-existing connection")
	}
}

func TestInMemoryConnectionRepository_Delete(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()
	conn := createTestConnection()
	_ = repo.Store(context.Background(), conn)

	// Execute
	err := repo.Delete(context.Background(), conn.ID)

	// Verify
	if err != nil {
		t.Errorf("Delete returned error: %v", err)
	}

	// Verify connection was deleted
	_, err = repo.GetByID(context.Background(), conn.ID)
	if err == nil {
		t.Error("Connection was not deleted")
	}

	// Test deleting a non-existing connection
	nonExistingID, _ := valueobjects.NewConnectionIDFromString("non-existing")
	err = repo.Delete(context.Background(), nonExistingID)
	if err == nil {
		t.Error("Delete should return error for non-existing connection")
	}
}

func TestInMemoryConnectionRepository_Cleanup(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()

	// Create some closed connections with different creation times
	oldConn := createTestConnection()
	oldConn.SetStatus(entities.ConnectionStatusClosed)
	oldConn.CreatedAt = time.Now().Add(-2 * time.Hour)
	_ = repo.Store(context.Background(), oldConn)

	recentConn := createTestConnection()
	recentConn.SetStatus(entities.ConnectionStatusClosed)
	recentConn.CreatedAt = time.Now().Add(-30 * time.Minute)
	_ = repo.Store(context.Background(), recentConn)

	activeConn := createTestConnection()
	activeConn.SetStatus(entities.ConnectionStatusActive)
	activeConn.CreatedAt = time.Now().Add(-2 * time.Hour)
	_ = repo.Store(context.Background(), activeConn)

	// Execute - cleanup connections older than 1 hour
	err := repo.Cleanup(context.Background(), 1*time.Hour)

	// Verify
	if err != nil {
		t.Errorf("Cleanup returned error: %v", err)
	}

	// Verify old closed connection was removed
	_, err = repo.GetByID(context.Background(), oldConn.ID)
	if err == nil {
		t.Error("Old closed connection was not removed")
	}

	// Verify recent closed connection was not removed
	_, err = repo.GetByID(context.Background(), recentConn.ID)
	if err != nil {
		t.Error("Recent closed connection was removed")
	}

	// Verify active connection was not removed
	_, err = repo.GetByID(context.Background(), activeConn.ID)
	if err != nil {
		t.Error("Active connection was removed")
	}

	// Test with invalid olderThan parameter
	err = repo.Cleanup(context.Background(), "invalid")
	if err == nil {
		t.Error("Cleanup should return error for invalid olderThan parameter")
	}
}

func TestInMemoryConnectionRepository_GetStats(t *testing.T) {
	// Setup
	repo := NewInMemoryConnectionRepository()

	// Create connections with different statuses
	activeConn := createTestConnection()
	activeConn.SetStatus(entities.ConnectionStatusActive)
	_ = repo.Store(context.Background(), activeConn)

	closedConn := createTestConnection()
	closedConn.SetStatus(entities.ConnectionStatusClosed)
	_ = repo.Store(context.Background(), closedConn)

	errorConn := createTestConnection()
	errorConn.SetStatus(entities.ConnectionStatusError)
	_ = repo.Store(context.Background(), errorConn)

	newConn := createTestConnection()
	newConn.SetStatus(entities.ConnectionStatusNew)
	_ = repo.Store(context.Background(), newConn)

	// Execute
	stats := repo.GetStats()

	// Verify
	if stats.TotalConnections != 4 {
		t.Errorf("Expected 4 total connections, got %d", stats.TotalConnections)
	}

	if stats.ActiveConnections != 1 {
		t.Errorf("Expected 1 active connection, got %d", stats.ActiveConnections)
	}

	if stats.ClosedConnections != 1 {
		t.Errorf("Expected 1 closed connection, got %d", stats.ClosedConnections)
	}

	if stats.ErrorConnections != 1 {
		t.Errorf("Expected 1 error connection, got %d", stats.ErrorConnections)
	}

	if stats.NewConnections != 1 {
		t.Errorf("Expected 1 new connection, got %d", stats.NewConnections)
	}
}

// Helper function to create a test connection
func createTestConnection() *entities.Connection {
	id := valueobjects.NewConnectionID()
	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	serverAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
	return entities.NewConnection(id, clientAddr, serverAddr)
}
