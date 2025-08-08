package usecases

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/services"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/igorrius/tcp-sproxy/internal/test/mocks"
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

func TestNewProxyUseCase(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080

	// Execute
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	// Verify
	if useCase == nil {
		t.Fatal("NewProxyUseCase returned nil")
	}

	if useCase.proxyService != proxyService {
		t.Error("ProxyService not set correctly")
	}

	if useCase.connectionRepo != repo {
		t.Error("ConnectionRepository not set correctly")
	}

	if useCase.transport != tr {
		t.Error("Transport not set correctly")
	}

	if useCase.remoteHost != remoteHost {
		t.Errorf("Expected remoteHost %s, got %s", remoteHost, useCase.remoteHost)
	}

	if useCase.remotePort != remotePort {
		t.Errorf("Expected remotePort %d, got %d", remotePort, useCase.remotePort)
	}
}

func TestProxyUseCase_HandleNewConnection(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	clientConn := newMockNetConn(clientAddr, clientAddr)

	// Test successful connection
	err := useCase.HandleNewConnection(context.Background(), clientConn)
	if err != nil {
		t.Errorf("HandleNewConnection returned error: %v", err)
	}

	// Verify repository was called to store the connection
	if repo.GetStoreCallCount() != 1 {
		t.Errorf("Expected Store to be called once, got %d", repo.GetStoreCallCount())
	}

	// Test error case
	repo.Reset()
	repo.StoreError = errors.New("store error")
	err = useCase.HandleNewConnection(context.Background(), clientConn)
	if err == nil {
		t.Error("HandleNewConnection should return error when Store fails")
	}
}

func TestProxyUseCase_GetConnectionStats(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	// Test successful stats retrieval
	_, err := useCase.GetConnectionStats(context.Background())
	if err != nil {
		t.Errorf("GetConnectionStats returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetGetAllCallCount() != 1 {
		t.Errorf("Expected GetAll to be called once, got %d", repo.GetGetAllCallCount())
	}

	// Test error case
	repo.Reset()
	repo.GetAllError = errors.New("get all error")
	_, err = useCase.GetConnectionStats(context.Background())
	if err == nil {
		t.Error("GetConnectionStats should return error when GetAll fails")
	}
}

func TestProxyUseCase_CloseConnection(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	// Create a test connection ID and connection
	connID := valueobjects.NewConnectionID()
	clientAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	serverAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 8080}
	conn := entities.NewConnection(connID, clientAddr, serverAddr)
	conn.SetStatus(entities.ConnectionStatusActive)

	// Store the connection in the repository
	repo.Store(context.Background(), conn)

	// Test successful connection closure
	err := useCase.CloseConnection(context.Background(), connID)
	if err != nil {
		t.Errorf("CloseConnection returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetGetByIDCallCount() != 1 {
		t.Errorf("Expected GetByID to be called once, got %d", repo.GetGetByIDCallCount())
	}

	// Test error case
	repo.Reset()
	repo.GetByIDError = errors.New("get by id error")
	err = useCase.CloseConnection(context.Background(), connID)
	if err == nil {
		t.Error("CloseConnection should return error when GetByID fails")
	}
}

func TestProxyUseCase_CleanupOldConnections(t *testing.T) {
	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	// Test successful cleanup
	err := useCase.CleanupOldConnections(context.Background(), 1*time.Hour)
	if err != nil {
		t.Errorf("CleanupOldConnections returned error: %v", err)
	}

	// Verify repository was called
	if repo.GetCleanupCallCount() != 1 {
		t.Errorf("Expected Cleanup to be called once, got %d", repo.GetCleanupCallCount())
	}

	// Test error case
	repo.Reset()
	repo.CleanupError = errors.New("cleanup error")
	err = useCase.CleanupOldConnections(context.Background(), 1*time.Hour)
	if err == nil {
		t.Error("CleanupOldConnections should return error when repository fails")
	}
}

func TestProxyUseCase_StartCleanupScheduler(t *testing.T) {
	// This is a bit tricky to test since it starts a goroutine
	// We'll just verify it doesn't panic

	// Setup
	repo := mocks.NewMockConnectionRepository()
	tr := mocks.NewMockTransport()
	proxyService := services.NewProxyService(repo, tr)
	remoteHost := "192.168.1.1"
	remotePort := 8080
	useCase := NewProxyUseCase(proxyService, repo, tr, remoteHost, remotePort)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the scheduler
	useCase.StartCleanupScheduler(ctx, 100*time.Millisecond)

	// Wait a bit to let it run
	time.Sleep(250 * time.Millisecond)

	// Cancel the context to stop the scheduler
	cancel()

	// Wait a bit more to let it stop
	time.Sleep(100 * time.Millisecond)

	// Verify repository was called at least once
	if repo.GetCleanupCallCount() < 1 {
		t.Errorf("Expected Cleanup to be called at least once, got %d", repo.GetCleanupCallCount())
	}
}
