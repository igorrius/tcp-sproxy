package services

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/igorrius/tcp-sproxy/internal/test/mocks"
	"github.com/igorrius/tcp-sproxy/pkg/transport"
)

// mockNetConn is a mock implementation of net.Conn for testing
type mockNetConn struct {
	localAddr  net.Addr
	remoteAddr net.Addr
	readData   []byte
	readErr    error
	writeErr   error
	closed     bool
}

func newMockNetConn(localAddr, remoteAddr net.Addr) *mockNetConn {
	return &mockNetConn{
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		readData:   []byte{},
	}
}

func (c *mockNetConn) Read(b []byte) (n int, err error) {
	if c.readErr != nil {
		return 0, c.readErr
	}
	if len(c.readData) == 0 {
		return 0, nil
	}
	n = copy(b, c.readData)
	c.readData = c.readData[n:]
	return n, nil
}

func (c *mockNetConn) Write(b []byte) (n int, err error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(b), nil
}

func (c *mockNetConn) Close() error {
	c.closed = true
	return nil
}

func (c *mockNetConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *mockNetConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *mockNetConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *mockNetConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *mockNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func TestNewProxyService(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()

	// Execute
	service := NewProxyService(repo, tr)

	// Verify
	if service == nil {
		t.Fatal("NewProxyService returned nil")
	}

	if service.connectionRepo != repo {
		t.Error("ConnectionRepository not set correctly")
	}

	if service.transport != tr {
		t.Error("Transport not set correctly")
	}
}

func TestProxyService_ProxyConnection(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	service := NewProxyService(repo, tr)

	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	clientConn := newMockNetConn(clientAddr, clientAddr)
	remoteHost := "192.168.1.1"
	remotePort := 8080

	// Test successful connection
	err := service.ProxyConnection(context.Background(), clientConn, remoteHost, remotePort)
	if err != nil {
		t.Errorf("ProxyConnection returned error: %v", err)
	}

	// Verify connection was stored
	if repo.StoreCallCount != 1 {
		t.Errorf("Expected Store to be called once, got %d", repo.StoreCallCount)
	}

	// Test error when storing connection
	repo.Reset()
	repo.StoreError = errors.New("store error")
	err = service.ProxyConnection(context.Background(), clientConn, remoteHost, remotePort)
	if err == nil {
		t.Error("ProxyConnection should return error when Store fails")
	}
	if repo.StoreCallCount != 1 {
		t.Errorf("Expected Store to be called once, got %d", repo.StoreCallCount)
	}
}

func TestProxyService_HandleNATSProxyRequest(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	service := NewProxyService(repo, tr)

	// Create a valid message
	msg := transport.NewMessage("test-id", []byte("test-data"))
	msg.Metadata = map[string]string{
		"remote_host":      "192.168.1.1",
		"remote_port":      "8080",
		"response_subject": "response.subject",
	}

	// Test successful request
	response, err := service.HandleNATSProxyRequest(context.Background(), msg)
	if err != nil {
		t.Errorf("HandleNATSProxyRequest returned error: %v", err)
	}
	if response == nil {
		t.Error("HandleNATSProxyRequest returned nil response")
	}
	if repo.StoreCallCount != 1 {
		t.Errorf("Expected Store to be called once, got %d", repo.StoreCallCount)
	}

	// Test missing remote_host
	repo.Reset()
	invalidMsg := transport.NewMessage("test-id", []byte("test-data"))
	invalidMsg.Metadata = map[string]string{
		"remote_port":      "8080",
		"response_subject": "response.subject",
	}
	_, err = service.HandleNATSProxyRequest(context.Background(), invalidMsg)
	if err == nil {
		t.Error("HandleNATSProxyRequest should return error when remote_host is missing")
	}

	// Test missing remote_port
	repo.Reset()
	invalidMsg = transport.NewMessage("test-id", []byte("test-data"))
	invalidMsg.Metadata = map[string]string{
		"remote_host":      "192.168.1.1",
		"response_subject": "response.subject",
	}
	_, err = service.HandleNATSProxyRequest(context.Background(), invalidMsg)
	if err == nil {
		t.Error("HandleNATSProxyRequest should return error when remote_port is missing")
	}

	// Test invalid remote_port
	repo.Reset()
	invalidMsg = transport.NewMessage("test-id", []byte("test-data"))
	invalidMsg.Metadata = map[string]string{
		"remote_host":      "192.168.1.1",
		"remote_port":      "invalid",
		"response_subject": "response.subject",
	}
	_, err = service.HandleNATSProxyRequest(context.Background(), invalidMsg)
	if err == nil {
		t.Error("HandleNATSProxyRequest should return error when remote_port is invalid")
	}

	// Test error when storing connection
	repo.Reset()
	repo.StoreError = errors.New("store error")
	_, err = service.HandleNATSProxyRequest(context.Background(), msg)
	if err == nil {
		t.Error("HandleNATSProxyRequest should return error when Store fails")
	}
	if repo.StoreCallCount != 1 {
		t.Errorf("Expected Store to be called once, got %d", repo.StoreCallCount)
	}
}

func TestProxyService_GetActiveConnections(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	service := NewProxyService(repo, tr)

	// Create some test connections
	_ = createTestConnection(entities.ConnectionStatusActive)
	_ = createTestConnection(entities.ConnectionStatusActive)

	// Mock repository to return active connections
	repo.GetActiveError = nil

	// Execute
	_, err := service.GetActiveConnections(context.Background())
	if err != nil {
		t.Errorf("GetActiveConnections returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetActiveCallCount != 1 {
		t.Errorf("Expected GetActive to be called once, got %d", repo.GetActiveCallCount)
	}

	// Test error case
	repo.Reset()
	repo.GetActiveError = errors.New("get active error")
	_, err = service.GetActiveConnections(context.Background())
	if err == nil {
		t.Error("GetActiveConnections should return error when GetActive fails")
	}
}

func TestProxyService_CloseConnection(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	service := NewProxyService(repo, tr)

	// Create a test connection ID and connection
	connID := valueobjects.NewConnectionID()
	conn := createTestConnectionWithID(connID, entities.ConnectionStatusActive)

	// Mock repository to return the connection
	repo.GetByIDError = nil
	// Store the connection in the mock repository's internal map
	repo.Store(context.Background(), conn)

	// Execute
	err := service.CloseConnection(context.Background(), connID)
	if err != nil {
		t.Errorf("CloseConnection returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetByIDCallCount != 1 {
		t.Errorf("Expected GetByID to be called once, got %d", repo.GetByIDCallCount)
	}
	if repo.UpdateCallCount != 1 {
		t.Errorf("Expected Update to be called once, got %d", repo.UpdateCallCount)
	}

	// Test error when getting connection
	repo.Reset()
	repo.GetByIDError = errors.New("get by id error")
	err = service.CloseConnection(context.Background(), connID)
	if err == nil {
		t.Error("CloseConnection should return error when GetByID fails")
	}

	// Test error when updating connection
	repo.Reset()
	// Need to store the connection again after reset
	repo.Store(context.Background(), conn)
	repo.GetByIDError = nil
	repo.UpdateError = errors.New("update error")
	err = service.CloseConnection(context.Background(), connID)
	if err == nil {
		t.Error("CloseConnection should return error when Update fails")
	}
}

func TestProxyService_GetConnectionStats(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	service := NewProxyService(repo, tr)

	// Create some test connections with different statuses
	_ = createTestConnection(entities.ConnectionStatusActive)
	_ = createTestConnection(entities.ConnectionStatusClosed)
	_ = createTestConnection(entities.ConnectionStatusError)
	_ = createTestConnection(entities.ConnectionStatusNew)

	// Mock repository to return all connections
	repo.GetAllError = nil

	// Execute
	_, err := service.GetConnectionStats(context.Background())
	if err != nil {
		t.Errorf("GetConnectionStats returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetAllCallCount != 1 {
		t.Errorf("Expected GetAll to be called once, got %d", repo.GetAllCallCount)
	}

	// Test error case
	repo.Reset()
	repo.GetAllError = errors.New("get all error")
	_, err = service.GetConnectionStats(context.Background())
	if err == nil {
		t.Error("GetConnectionStats should return error when GetAll fails")
	}
}

// Helper function to create a test connection with a specific status
func createTestConnection(status entities.ConnectionStatus) *entities.Connection {
	id := valueobjects.NewConnectionID()
	return createTestConnectionWithID(id, status)
}

// Helper function to create a test connection with a specific ID and status
func createTestConnectionWithID(id valueobjects.ConnectionID, status entities.ConnectionStatus) *entities.Connection {
	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	serverAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
	conn := entities.NewConnection(id, clientAddr, serverAddr)
	conn.SetStatus(status)
	return conn
}
