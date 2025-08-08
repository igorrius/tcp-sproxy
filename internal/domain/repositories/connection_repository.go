package repositories

import (
	"context"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
)

// ConnectionRepository defines the interface for connection persistence
type ConnectionRepository interface {
	// Store stores a new connection
	Store(ctx context.Context, connection *entities.Connection) error

	// GetByID retrieves a connection by its ID
	GetByID(ctx context.Context, id valueobjects.ConnectionID) (*entities.Connection, error)

	// GetActive retrieves all active connections
	GetActive(ctx context.Context) ([]*entities.Connection, error)

	// GetAll retrieves all connections
	GetAll(ctx context.Context) ([]*entities.Connection, error)

	// Update updates an existing connection
	Update(ctx context.Context, connection *entities.Connection) error

	// Delete removes a connection
	Delete(ctx context.Context, id valueobjects.ConnectionID) error

	// Cleanup removes closed connections older than the specified duration
	Cleanup(ctx context.Context, olderThan interface{}) error
}
