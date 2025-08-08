package services

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/igorrius/tcp-sproxy/internal/domain/entities"
	"github.com/igorrius/tcp-sproxy/internal/domain/repositories"
	"github.com/igorrius/tcp-sproxy/internal/domain/valueobjects"
	"github.com/igorrius/tcp-sproxy/pkg/transport"
	"github.com/sirupsen/logrus"
)

// ProxyService handles the core proxy business logic
type ProxyService struct {
	connectionRepo repositories.ConnectionRepository
	transport      transport.Transport
	logger         *logrus.Entry
}

// NewProxyService creates a new proxy service
func NewProxyService(connectionRepo repositories.ConnectionRepository, transport transport.Transport) *ProxyService {
	return &ProxyService{
		connectionRepo: connectionRepo,
		transport:      transport,
		logger:         logrus.WithField("service", "proxy"),
	}
}

// ProxyConnection handles a new connection request
func (ps *ProxyService) ProxyConnection(ctx context.Context, clientConn net.Conn, remoteHost string, remotePort int) error {
	// Create new connection entity
	connID := valueobjects.NewConnectionID()
	connection := entities.NewConnection(connID, clientConn.RemoteAddr(), &net.TCPAddr{
		IP:   net.ParseIP(remoteHost),
		Port: remotePort,
	})

	// Store connection
	if err := ps.connectionRepo.Store(ctx, connection); err != nil {
		ps.logger.WithError(err).Error("Failed to store connection")
		return fmt.Errorf("failed to store connection: %w", err)
	}

	ps.logger.WithFields(logrus.Fields{
		"connection_id": connID.String(),
		"client_addr":   clientConn.RemoteAddr(),
		"remote_host":   remoteHost,
		"remote_port":   remotePort,
	}).Info("New connection established")

	// Handle the connection in a goroutine
	// Status will be set to active inside the goroutine after successful connection
	go ps.handleConnection(ctx, connection, clientConn, remoteHost, remotePort)

	return nil
}

// HandleNATSProxyRequest handles proxy requests from NATS messages
func (ps *ProxyService) HandleNATSProxyRequest(ctx context.Context, msg *transport.Message) (*transport.Message, error) {
	// Extract remote host and port from message metadata
	remoteHost, ok := msg.Metadata["remote_host"]
	if !ok {
		return nil, fmt.Errorf("remote_host not found in message metadata")
	}

	remotePortStr, ok := msg.Metadata["remote_port"]
	if !ok {
		return nil, fmt.Errorf("remote_port not found in message metadata")
	}

	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil {
		return nil, fmt.Errorf("invalid remote_port: %w", err)
	}

	// Create virtual connection for NATS-based proxy
	connID := valueobjects.NewConnectionID()
	connection := entities.NewConnection(connID, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}, &net.TCPAddr{
		IP:   net.ParseIP(remoteHost),
		Port: remotePort,
	})

	// Store connection
	if err := ps.connectionRepo.Store(ctx, connection); err != nil {
		ps.logger.WithError(err).Error("Failed to store connection")
		return nil, fmt.Errorf("failed to store connection: %w", err)
	}

	ps.logger.WithFields(logrus.Fields{
		"connection_id": connID.String(),
		"remote_host":   remoteHost,
		"remote_port":   remotePort,
	}).Info("New NATS proxy connection established")

	// Handle the NATS-based connection in a separate worker
	// Status will be set to active inside the goroutine after successful connection
	go ps.handleNATSConnection(ctx, connection, msg)

	return transport.NewMessage(msg.ID, []byte("OK")), nil
}

// handleNATSConnection manages the data flow for NATS-based connections
func (ps *ProxyService) handleNATSConnection(ctx context.Context, connection *entities.Connection, initialMsg *transport.Message) {
	defer func() {
		connection.Close()
		ps.logger.WithField("connection_id", connection.ID.String()).Info("NATS connection handler finished")
	}()

	// Extract remote host and port from initial message
	remoteHost := initialMsg.Metadata["remote_host"]
	remotePortStr := initialMsg.Metadata["remote_port"]
	remotePort, _ := strconv.Atoi(remotePortStr)

	// Connect to remote server
	remoteAddr := net.JoinHostPort(remoteHost, fmt.Sprintf("%d", remotePort))
	remoteConn, err := net.DialTimeout("tcp", remoteAddr, 30*time.Second)
	if err != nil {
		ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Failed to connect to remote server")
		connection.SetStatus(entities.ConnectionStatusError)
		return
	}
	defer remoteConn.Close()

	// Set connection as active only after successful connection
	connection.SetStatus(entities.ConnectionStatusActive)
	ps.logger.WithField("connection_id", connection.ID.String()).Info("Connected to remote server via NATS")

	// Create a virtual connection for data flow
	virtualConn := &VirtualConnection{
		connID:          connection.ID.String(),
		transport:       ps.transport,
		responseSubject: initialMsg.Metadata["response_subject"],
		logger:          ps.logger,
	}

	// Create channels for coordination
	errChan := make(chan error, 2)
	doneChan := make(chan struct{})

	// Start bidirectional data copying
	go ps.copyData(ctx, connection, virtualConn, remoteConn, "nats->remote", errChan)
	go ps.copyData(ctx, connection, remoteConn, virtualConn, "remote->nats", errChan)

	// Wait for either direction to finish or error
	select {
	case err := <-errChan:
		if err != nil && err != io.EOF {
			ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("NATS connection error")
		}
	case <-ctx.Done():
		ps.logger.WithField("connection_id", connection.ID.String()).Info("NATS connection cancelled by context")
	case <-doneChan:
		ps.logger.WithField("connection_id", connection.ID.String()).Info("NATS connection completed normally")
	}
}

// handleConnection manages the data flow between client and remote server
func (ps *ProxyService) handleConnection(ctx context.Context, connection *entities.Connection, clientConn net.Conn, remoteHost string, remotePort int) {
	defer func() {
		clientConn.Close()
		connection.Close()
		ps.logger.WithField("connection_id", connection.ID.String()).Info("Connection handler finished")
	}()

	// Connect to remote server
	remoteAddr := net.JoinHostPort(remoteHost, fmt.Sprintf("%d", remotePort))
	remoteConn, err := net.DialTimeout("tcp", remoteAddr, 30*time.Second)
	if err != nil {
		ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Failed to connect to remote server")
		connection.SetStatus(entities.ConnectionStatusError)
		return
	}
	defer remoteConn.Close()

	// Set connection as active only after successful connection
	connection.SetStatus(entities.ConnectionStatusActive)
	ps.logger.WithField("connection_id", connection.ID.String()).Info("Connected to remote server")

	// Create channels for coordination
	errChan := make(chan error, 2)
	doneChan := make(chan struct{})

	// Start bidirectional data copying
	go ps.copyData(ctx, connection, clientConn, remoteConn, "client->remote", errChan)
	go ps.copyData(ctx, connection, remoteConn, clientConn, "remote->client", errChan)

	// Wait for either direction to finish or error
	select {
	case err := <-errChan:
		if err != nil && err != io.EOF {
			ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Connection error")
		}
	case <-ctx.Done():
		ps.logger.WithField("connection_id", connection.ID.String()).Info("Connection cancelled by context")
	case <-doneChan:
		ps.logger.WithField("connection_id", connection.ID.String()).Info("Connection completed normally")
	}
}

// copyData copies data from src to dst while updating connection activity
func (ps *ProxyService) copyData(ctx context.Context, connection *entities.Connection, src, dst net.Conn, direction string, errChan chan<- error) {
	defer func() {
		errChan <- io.EOF
	}()

	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Set read timeout
			src.SetReadDeadline(time.Now().Add(30 * time.Second))

			n, err := src.Read(buffer)
			if err != nil {
				if err != io.EOF {
					ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Errorf("Read error in %s", direction)
				}
				return
			}

			if n > 0 {
				// Update connection activity
				connection.UpdateActivity()

				// Write data to destination
				dst.SetWriteDeadline(time.Now().Add(30 * time.Second))
				_, writeErr := dst.Write(buffer[:n])
				if writeErr != nil {
					ps.logger.WithError(writeErr).WithField("connection_id", connection.ID.String()).Errorf("Write error in %s", direction)
					return
				}

				ps.logger.WithFields(logrus.Fields{
					"connection_id": connection.ID.String(),
					"direction":     direction,
					"bytes":         n,
				}).Debug("Data copied")
			}
		}
	}
}

// GetActiveConnections returns all active connections
func (ps *ProxyService) GetActiveConnections(ctx context.Context) ([]*entities.Connection, error) {
	return ps.connectionRepo.GetActive(ctx)
}

// CloseConnection closes a specific connection
func (ps *ProxyService) CloseConnection(ctx context.Context, connID valueobjects.ConnectionID) error {
	connection, err := ps.connectionRepo.GetByID(ctx, connID)
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}

	connection.Close()
	return ps.connectionRepo.Update(ctx, connection)
}

// GetConnectionStats returns statistics about connections
func (ps *ProxyService) GetConnectionStats(ctx context.Context) (*ConnectionStats, error) {
	connections, err := ps.connectionRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connections: %w", err)
	}

	stats := &ConnectionStats{
		Total:     len(connections),
		Active:    0,
		Closed:    0,
		Error:     0,
		New:       0,
		CreatedAt: time.Now(),
	}

	for _, conn := range connections {
		switch conn.GetStatus() {
		case entities.ConnectionStatusActive:
			stats.Active++
		case entities.ConnectionStatusClosed:
			stats.Closed++
		case entities.ConnectionStatusError:
			stats.Error++
		case entities.ConnectionStatusNew:
			stats.New++
		}
	}

	return stats, nil
}

// VirtualConnection represents a virtual connection for NATS-based data flow
type VirtualConnection struct {
	connID          string
	transport       transport.Transport
	responseSubject string
	logger          *logrus.Entry
}

// Read implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) Read(b []byte) (n int, err error) {
	// This is a placeholder - actual implementation would need to handle NATS message reading
	// For now, return EOF to indicate end of stream
	return 0, io.EOF
}

// Write implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) Write(b []byte) (n int, err error) {
	// Send data through NATS
	msg := transport.NewMessage(vc.connID, b)
	err = vc.transport.Publish(context.Background(), vc.responseSubject, msg)
	if err != nil {
		vc.logger.WithError(err).Error("Failed to send data through NATS")
		return 0, err
	}
	return len(b), nil
}

// Close implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) Close() error {
	// Send close message
	closeMsg := transport.NewMessage(vc.connID, []byte("CLOSE"))
	return vc.transport.Publish(context.Background(), vc.responseSubject, closeMsg)
}

// LocalAddr implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// RemoteAddr implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// SetDeadline implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline implements net.Conn interface for VirtualConnection
func (vc *VirtualConnection) SetWriteDeadline(t time.Time) error {
	return nil
}

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	Total     int       `json:"total"`
	Active    int       `json:"active"`
	Closed    int       `json:"closed"`
	Error     int       `json:"error"`
	New       int       `json:"new"`
	CreatedAt time.Time `json:"created_at"`
}
