package repositories

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/sirupsen/logrus"
)

// InMemoryConnectionRepository implements ConnectionRepository using in-memory storage
type InMemoryConnectionRepository struct {
	connections map[string]*entities.Connection
	mu          sync.RWMutex
	logger      *logrus.Entry
}

// NewInMemoryConnectionRepository creates a new in-memory connection repository
func NewInMemoryConnectionRepository() *InMemoryConnectionRepository {
	return &InMemoryConnectionRepository{
		connections: make(map[string]*entities.Connection),
		logger:      logrus.WithField("repository", "inmemory_connection"),
	}
}

// Store stores a new connection
func (r *InMemoryConnectionRepository) Store(ctx context.Context, connection *entities.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	connID := connection.ID.String()
	if _, exists := r.connections[connID]; exists {
		return fmt.Errorf("connection with ID %s already exists", connID)
	}

	r.connections[connID] = connection
	r.logger.WithField("connection_id", connID).Debug("Connection stored")

	return nil
}

// GetByID retrieves a connection by its ID
func (r *InMemoryConnectionRepository) GetByID(ctx context.Context, id valueobjects.ConnectionID) (*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connID := id.String()
	connection, exists := r.connections[connID]
	if !exists {
		return nil, fmt.Errorf("connection with ID %s not found", connID)
	}

	return connection, nil
}

// GetActive retrieves all active connections
func (r *InMemoryConnectionRepository) GetActive(ctx context.Context) ([]*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var activeConnections []*entities.Connection
	for _, connection := range r.connections {
		if connection.IsActive() {
			activeConnections = append(activeConnections, connection)
		}
	}

	return activeConnections, nil
}

// GetAll retrieves all connections
func (r *InMemoryConnectionRepository) GetAll(ctx context.Context) ([]*entities.Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	connections := make([]*entities.Connection, 0, len(r.connections))
	for _, connection := range r.connections {
		connections = append(connections, connection)
	}

	return connections, nil
}

// Update updates an existing connection
func (r *InMemoryConnectionRepository) Update(ctx context.Context, connection *entities.Connection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	connID := connection.ID.String()
	if _, exists := r.connections[connID]; !exists {
		return fmt.Errorf("connection with ID %s not found", connID)
	}

	r.connections[connID] = connection
	r.logger.WithField("connection_id", connID).Debug("Connection updated")

	return nil
}

// Delete removes a connection
func (r *InMemoryConnectionRepository) Delete(ctx context.Context, id valueobjects.ConnectionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	connID := id.String()
	if _, exists := r.connections[connID]; !exists {
		return fmt.Errorf("connection with ID %s not found", connID)
	}

	delete(r.connections, connID)
	r.logger.WithField("connection_id", connID).Debug("Connection deleted")

	return nil
}

// Cleanup removes closed connections older than the specified duration
func (r *InMemoryConnectionRepository) Cleanup(ctx context.Context, olderThan interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	duration, ok := olderThan.(time.Duration)
	if !ok {
		return fmt.Errorf("olderThan parameter must be time.Duration")
	}

	cutoffTime := time.Now().Add(-duration)
	var deletedCount int

	for connID, connection := range r.connections {
		if connection.IsClosed() && connection.CreatedAt.Before(cutoffTime) {
			delete(r.connections, connID)
			deletedCount++
		}
	}

	if deletedCount > 0 {
		r.logger.WithField("deleted_count", deletedCount).Info("Cleaned up old connections")
	}

	return nil
}

// GetStats returns repository statistics
func (r *InMemoryConnectionRepository) GetStats() *RepositoryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &RepositoryStats{
		TotalConnections:  len(r.connections),
		ActiveConnections: 0,
		ClosedConnections: 0,
		ErrorConnections:  0,
		NewConnections:    0,
	}

	for _, connection := range r.connections {
		switch connection.GetStatus() {
		case entities.ConnectionStatusActive:
			stats.ActiveConnections++
		case entities.ConnectionStatusClosed:
			stats.ClosedConnections++
		case entities.ConnectionStatusError:
			stats.ErrorConnections++
		case entities.ConnectionStatusNew:
			stats.NewConnections++
		}
	}

	return stats
}

// RepositoryStats represents repository statistics
type RepositoryStats struct {
	TotalConnections  int `json:"total_connections"`
	ActiveConnections int `json:"active_connections"`
	ClosedConnections int `json:"closed_connections"`
	ErrorConnections  int `json:"error_connections"`
	NewConnections    int `json:"new_connections"`
}
