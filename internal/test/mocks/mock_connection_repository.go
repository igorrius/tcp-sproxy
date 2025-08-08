package mocks

import (
	"context"
	"sync"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
)

// MockConnectionRepository is a mock implementation of the ConnectionRepository interface
type MockConnectionRepository struct {
	connections map[string]*entities.Connection
	mu          sync.RWMutex

	// Function call tracking for testing (protected by callCountMu)
	callCountMu        sync.RWMutex
	StoreCallCount     int
	GetByIDCallCount   int
	GetActiveCallCount int
	GetAllCallCount    int
	UpdateCallCount    int
	DeleteCallCount    int
	CleanupCallCount   int

	// Error simulation
	StoreError     error
	GetByIDError   error
	GetActiveError error
	GetAllError    error
	UpdateError    error
	DeleteError    error
	CleanupError   error
}

// NewMockConnectionRepository creates a new mock connection repository
func NewMockConnectionRepository() *MockConnectionRepository {
	return &MockConnectionRepository{
		connections: make(map[string]*entities.Connection),
	}
}

// Store stores a new connection
func (r *MockConnectionRepository) Store(ctx context.Context, connection *entities.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callCountMu.Lock()
	r.StoreCallCount++
	r.callCountMu.Unlock()

	if r.StoreError != nil {
		return r.StoreError
	}

	r.connections[connection.ID.String()] = connection
	return nil
}

// GetByID retrieves a connection by its ID
func (r *MockConnectionRepository) GetByID(ctx context.Context, id valueobjects.ConnectionID) (*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.callCountMu.Lock()
	r.GetByIDCallCount++
	r.callCountMu.Unlock()

	if r.GetByIDError != nil {
		return nil, r.GetByIDError
	}

	conn, exists := r.connections[id.String()]
	if !exists {
		return nil, nil
	}

	return conn, nil
}

// GetActive retrieves all active connections
func (r *MockConnectionRepository) GetActive(ctx context.Context) ([]*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.callCountMu.Lock()
	r.GetActiveCallCount++
	r.callCountMu.Unlock()

	if r.GetActiveError != nil {
		return nil, r.GetActiveError
	}

	var activeConnections []*entities.Connection
	for _, conn := range r.connections {
		if conn.IsActive() {
			activeConnections = append(activeConnections, conn)
		}
	}

	return activeConnections, nil
}

// GetAll retrieves all connections
func (r *MockConnectionRepository) GetAll(ctx context.Context) ([]*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.callCountMu.Lock()
	r.GetAllCallCount++
	r.callCountMu.Unlock()

	if r.GetAllError != nil {
		return nil, r.GetAllError
	}

	connections := make([]*entities.Connection, 0, len(r.connections))
	for _, conn := range r.connections {
		connections = append(connections, conn)
	}

	return connections, nil
}

// Update updates an existing connection
func (r *MockConnectionRepository) Update(ctx context.Context, connection *entities.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callCountMu.Lock()
	r.UpdateCallCount++
	r.callCountMu.Unlock()

	if r.UpdateError != nil {
		return r.UpdateError
	}

	r.connections[connection.ID.String()] = connection
	return nil
}

// Delete removes a connection
func (r *MockConnectionRepository) Delete(ctx context.Context, id valueobjects.ConnectionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callCountMu.Lock()
	r.DeleteCallCount++
	r.callCountMu.Unlock()

	if r.DeleteError != nil {
		return r.DeleteError
	}

	delete(r.connections, id.String())
	return nil
}

// Cleanup removes closed connections older than the specified duration
func (r *MockConnectionRepository) Cleanup(ctx context.Context, olderThan interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callCountMu.Lock()
	r.CleanupCallCount++
	r.callCountMu.Unlock()

	if r.CleanupError != nil {
		return r.CleanupError
	}

	// In the mock, we don't actually need to implement the cleanup logic
	// unless specific tests require it

	return nil
}

// Reset resets the mock state
func (r *MockConnectionRepository) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callCountMu.Lock()
	defer r.callCountMu.Unlock()

	r.connections = make(map[string]*entities.Connection)
	r.StoreCallCount = 0
	r.GetByIDCallCount = 0
	r.GetActiveCallCount = 0
	r.GetAllCallCount = 0
	r.UpdateCallCount = 0
	r.DeleteCallCount = 0
	r.CleanupCallCount = 0

	r.StoreError = nil
	r.GetByIDError = nil
	r.GetActiveError = nil
	r.GetAllError = nil
	r.UpdateError = nil
	r.DeleteError = nil
	r.CleanupError = nil
}

// Thread-safe getter methods for call counts
func (r *MockConnectionRepository) GetStoreCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.StoreCallCount
}

func (r *MockConnectionRepository) GetGetByIDCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.GetByIDCallCount
}

func (r *MockConnectionRepository) GetGetActiveCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.GetActiveCallCount
}

func (r *MockConnectionRepository) GetGetAllCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.GetAllCallCount
}

func (r *MockConnectionRepository) GetUpdateCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.UpdateCallCount
}

func (r *MockConnectionRepository) GetDeleteCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.DeleteCallCount
}

func (r *MockConnectionRepository) GetCleanupCallCount() int {
	r.callCountMu.RLock()
	defer r.callCountMu.RUnlock()
	return r.CleanupCallCount
}
