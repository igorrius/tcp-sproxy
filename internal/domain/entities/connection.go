package entities

import (
	"net"
	"sync"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/sirupsen/logrus"
)

// Connection represents a TCP connection between client and server
type Connection struct {
	ID           valueobjects.ConnectionID
	ClientAddr   net.Addr
	ServerAddr   net.Addr
	CreatedAt    time.Time
	LastActivity time.Time
	Status       ConnectionStatus
	mu           sync.RWMutex
	logger       *logrus.Entry
}

// ConnectionStatus represents the state of a connection
type ConnectionStatus string

const (
	ConnectionStatusNew    ConnectionStatus = "new"
	ConnectionStatusActive ConnectionStatus = "active"
	ConnectionStatusClosed ConnectionStatus = "closed"
	ConnectionStatusError  ConnectionStatus = "error"
)

// NewConnection creates a new connection entity
func NewConnection(id valueobjects.ConnectionID, clientAddr, serverAddr net.Addr) *Connection {
	return &Connection{
		ID:           id,
		ClientAddr:   clientAddr,
		ServerAddr:   serverAddr,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		Status:       ConnectionStatusNew,
		logger:       logrus.WithField("connection_id", id.String()),
	}
}

// UpdateActivity updates the last activity timestamp
func (c *Connection) UpdateActivity() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LastActivity = time.Now()
}

// SetStatus updates the connection status
func (c *Connection) SetStatus(status ConnectionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = status
	c.logger.WithField("status", status).Debug("Connection status updated")
}

// GetStatus returns the current connection status
func (c *Connection) GetStatus() ConnectionStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status
}

// IsActive returns true if the connection is active
func (c *Connection) IsActive() bool {
	return c.GetStatus() == ConnectionStatusActive
}

// IsClosed returns true if the connection is closed
func (c *Connection) IsClosed() bool {
	return c.GetStatus() == ConnectionStatusClosed
}

// Close marks the connection as closed
func (c *Connection) Close() {
	c.SetStatus(ConnectionStatusClosed)
	c.logger.Info("Connection closed")
}

// GetDuration returns the duration since the connection was created
func (c *Connection) GetDuration() time.Duration {
	return time.Since(c.CreatedAt)
}

// GetIdleTime returns the duration since the last activity
func (c *Connection) GetIdleTime() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return time.Since(c.LastActivity)
}
