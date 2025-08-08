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
func NewProxyService(connectionRepo repositories.ConnectionRepository, transport transport.Transport, logger *logrus.Entry) *ProxyService {
	return &ProxyService{
		connectionRepo: connectionRepo,
		transport:      transport,
		logger:         logger.WithField("service", "proxy"),
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

// HandleNATSProxyRequests subscribes to a NATS subject and handles incoming proxy requests.
func (s *ProxyService) HandleNATSProxyRequests(ctx context.Context, subject string) error {
	msgChan, err := s.transport.Subscribe(ctx, subject)
	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
	}

	s.logger.WithField("subject", subject).Info("Listening for NATS proxy requests")

	for {
		select {
		case msg := <-msgChan:
			go func(m *transport.Message) {
				if err := s.handleIndividualProxyRequest(ctx, m); err != nil {
					s.logger.WithError(err).WithField("message_id", m.ID).Error("Failed to handle proxy request")
					if m.Reply != "" {
						errorMsg := transport.NewMessage(m.ID, []byte(err.Error()))
						errorMsg.Metadata = map[string]string{"error": "true"}
						if pubErr := s.transport.Publish(ctx, m.Reply, errorMsg); pubErr != nil {
							s.logger.WithError(pubErr).Error("Failed to publish error response")
						}
					}
				}
			}(msg)
		case <-ctx.Done():
			s.logger.Info("Stopping NATS proxy request handler")
			return nil
		}
	}
}

// handleIndividualProxyRequest handles individual proxy requests
func (s *ProxyService) handleIndividualProxyRequest(ctx context.Context, msg *transport.Message) error {
	s.logger.WithField("message_id", msg.ID).Info("Handling new NATS proxy request")

	remoteHost, ok := msg.Metadata["remote_host"]
	if !ok {
		return fmt.Errorf("remote_host not found in message metadata")
	}

	remotePortStr, ok := msg.Metadata["remote_port"]
	if !ok {
		return fmt.Errorf("remote_port not found in message metadata")
	}

	remotePort, err := strconv.Atoi(remotePortStr)
	if err != nil {
		return fmt.Errorf("invalid remote_port: %w", err)
	}

	ips, err := net.LookupIP(remoteHost)
	if err != nil {
		return fmt.Errorf("failed to resolve remote host: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no IP address found for remote host")
	}

	connID := valueobjects.NewConnectionID()
	connection := entities.NewConnection(connID, &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}, &net.TCPAddr{
		IP:   ips[0],
		Port: remotePort,
	})

	if err := s.connectionRepo.Store(ctx, connection); err != nil {
		s.logger.WithError(err).Error("Failed to store connection")
		return fmt.Errorf("failed to store connection: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"connection_id": connID.String(),
		"remote_host":   remoteHost,
		"remote_port":   remotePort,
	}).Info("New NATS proxy connection established")

	go s.handleNATSConnection(ctx, connection, msg.Reply)

	responseMsg := transport.NewMessage(msg.ID, []byte(connID.String()))
	if msg.Reply != "" {
		if err := s.transport.Publish(ctx, msg.Reply, responseMsg); err != nil {
			return fmt.Errorf("failed to send response: %w", err)
		}
	}

	return nil
}

// handleNATSConnection manages the data flow for NATS-based connections
func (ps *ProxyService) handleNATSConnection(ctx context.Context, connection *entities.Connection, replySubject string) {
	defer func() {
		connection.Close()
		ps.logger.WithField("connection_id", connection.ID.String()).Info("NATS connection handler finished")
	}()

	// Extract remote host and port from connection entity
	remoteHost := connection.ServerAddr.(*net.TCPAddr).IP.String()
	remotePort := connection.ServerAddr.(*net.TCPAddr).Port

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

	// Create channels for coordination
	errChan := make(chan error, 2)

	// Start bidirectional data copying
	go func() {
		// remote -> nats
		buf := make([]byte, 32*1024)
		for {
			n, err := remoteConn.Read(buf)
			if err != nil {
				if err != io.EOF {
					ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Read error in remote->nats")
				}
				errChan <- err
				return
			}
			ps.logger.WithField("connection_id", connection.ID.String()).Infof("remote->nats: read %d bytes", n)
			data := buf[:n]
			msg := transport.NewMessage(connection.ID.String(), data)
			subject := "p.data.to_client." + connection.ID.String()
			if err := ps.transport.Publish(ctx, subject, msg); err != nil {
				ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Failed to publish to nats")
			}
		}
	}()

	go func() {
		// nats -> remote
		subject := "p.data.to_server." + connection.ID.String()
		msgChan, err := ps.transport.Subscribe(ctx, subject)
		if err != nil {
			ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Failed to subscribe to nats")
			errChan <- err
			return
		}

		for {
			select {
			case msg := <-msgChan:
				ps.logger.WithField("connection_id", connection.ID.String()).Infof("nats->remote: got message with %d bytes", len(msg.Data))
				if _, err := remoteConn.Write(msg.Data); err != nil {
					if err != io.EOF {
						ps.logger.WithError(err).WithField("connection_id", connection.ID.String()).Error("Write error in nats->remote")
					}
					errChan <- err
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for either direction to finish or error
	<-errChan
	connection.SetStatus(entities.ConnectionStatusClosed)
	ps.logger.WithField("connection_id", connection.ID.String()).Info("Connection closed")
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
			// Set read deadline
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

// ConnectionStats represents connection statistics
type ConnectionStats struct {
	Total     int       `json:"total"`
	Active    int       `json:"active"`
	Closed    int       `json:"closed"`
	Error     int       `json:"error"`
	New       int       `json:"new"`
	CreatedAt time.Time `json:"created_at"`
}
