package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/igorrius/tcp-sproxy/pkg/transport"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// NATSTransport implements the Transport interface using NATS
type NATSTransport struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	config  *NATSConfig
	logger  *logrus.Entry
	mu      sync.RWMutex
	subs    map[string]*nats.Subscription
	msgChan chan *transport.Message
}

// NATSConfig represents NATS-specific configuration
type NATSConfig struct {
	URL     string            `json:"url"`
	Options map[string]string `json:"options"`
	Timeout time.Duration     `json:"timeout"`
	Retries int               `json:"retries"`
	Stream  string            `json:"stream"`
	Subject string            `json:"subject"`
}

// NewNATSTransport creates a new NATS transport instance
func NewNATSTransport(logger *logrus.Entry) *NATSTransport {
	return &NATSTransport{
		subs:    make(map[string]*nats.Subscription),
		msgChan: make(chan *transport.Message, 100),
		logger:  logger,
	}
}

// Connect establishes a connection to NATS server
func (nt *NATSTransport) Connect(ctx context.Context, config interface{}) error {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	cfg, ok := config.(*NATSConfig)
	if !ok {
		return fmt.Errorf("invalid config type for NATS transport")
	}
	nt.config = cfg

	// Set default timeout if not provided
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Connect to NATS
	opts := []nats.Option{
		nats.Timeout(cfg.Timeout),
		nats.ReconnectWait(1 * time.Second),
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			nt.logger.WithError(err).Warn("NATS disconnected")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			nt.logger.Info("NATS reconnected")
		}),
	}

	conn, err := nats.Connect(cfg.URL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}

	nt.conn = conn
	nt.logger.WithField("url", cfg.URL).Info("Connected to NATS server")

	// Initialize JetStream if stream is configured
	if cfg.Stream != "" {
		js, err := conn.JetStream()
		if err != nil {
			return fmt.Errorf("failed to get JetStream context: %w", err)
		}
		nt.js = js
		nt.logger.WithField("stream", cfg.Stream).Info("JetStream initialized")
	}

	return nil
}

// Disconnect closes the NATS connection
func (nt *NATSTransport) Disconnect(ctx context.Context) error {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	if nt.conn != nil {
		nt.conn.Close()
		nt.conn = nil
		nt.logger.Info("Disconnected from NATS server")
	}

	return nil
}

// Send sends a message to a specific subject
func (nt *NATSTransport) Send(ctx context.Context, subject string, message *transport.Message) error {
	if !nt.IsConnected() {
		return fmt.Errorf("not connected to NATS")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = nt.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	nt.logger.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": message.ID,
	}).Debug("Message sent")

	return nil
}

// Subscribe subscribes to a subject and returns a channel for receiving messages
func (nt *NATSTransport) Subscribe(ctx context.Context, subject string) (<-chan *transport.Message, error) {
	if !nt.IsConnected() {
		return nil, fmt.Errorf("not connected to NATS")
	}

	// Check if already subscribed
	nt.mu.RLock()
	if _, exists := nt.subs[subject]; exists {
		nt.mu.RUnlock()
		return nil, fmt.Errorf("already subscribed to subject: %s", subject)
	}
	nt.mu.RUnlock()

	// Create subscription
	sub, err := nt.conn.Subscribe(subject, func(msg *nats.Msg) {
		var transportMsg transport.Message
		if err := json.Unmarshal(msg.Data, &transportMsg); err != nil {
			nt.logger.WithError(err).Error("Failed to unmarshal message")
			return
		}
		transportMsg.Reply = msg.Reply

		// Manually decode headers if they exist
		if msg.Header != nil {
			meta := make(map[string]string)
			for key, values := range msg.Header {
				if len(values) > 0 {
					meta[key] = values[0]
				}
			}
			transportMsg.Metadata = meta
		}

		select {
		case nt.msgChan <- &transportMsg:
		case <-ctx.Done():
			nt.logger.Debug("Context cancelled, dropping message")
		default:
			nt.logger.Warn("Message channel full, dropping message")
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	// Store subscription
	nt.mu.Lock()
	nt.subs[subject] = sub
	nt.mu.Unlock()

	nt.logger.WithField("subject", subject).Info("Subscribed to subject")

	return nt.msgChan, nil
}

// Request sends a request and waits for a response
func (nt *NATSTransport) Request(ctx context.Context, subject string, message *transport.Message) (*transport.Message, error) {
	if !nt.IsConnected() {
		return nil, fmt.Errorf("not connected to NATS")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create a NATS message and set headers from metadata
	natsMsg := nats.NewMsg(subject)
	natsMsg.Data = data
	if message.Metadata != nil {
		natsMsg.Header = make(nats.Header)
		for k, v := range message.Metadata {
			natsMsg.Header.Set(k, v)
		}
	}

	// Send request with timeout
	timeout := nt.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	response, err := nt.conn.RequestMsg(natsMsg, timeout)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var transportMsg transport.Message
	if err := json.Unmarshal(response.Data, &transportMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	nt.logger.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": message.ID,
	}).Debug("Request completed")

	return &transportMsg, nil
}

// Publish publishes a message to a subject (fire and forget)
func (nt *NATSTransport) Publish(ctx context.Context, subject string, message *transport.Message) error {
	return nt.Send(ctx, subject, message)
}

// Close closes the transport and cleans up resources
func (nt *NATSTransport) Close() error {
	nt.mu.Lock()
	defer nt.mu.Unlock()

	// Unsubscribe from all subjects
	for subject, sub := range nt.subs {
		sub.Unsubscribe()
		nt.logger.WithField("subject", subject).Debug("Unsubscribed from subject")
	}
	nt.subs = make(map[string]*nats.Subscription)

	// Close connection
	if nt.conn != nil {
		nt.conn.Close()
		nt.conn = nil
	}

	// Close message channel
	close(nt.msgChan)

	nt.logger.Info("NATS transport closed")
	return nil
}

// IsConnected returns true if the transport is connected
func (nt *NATSTransport) IsConnected() bool {
	nt.mu.RLock()
	defer nt.mu.RUnlock()
	return nt.conn != nil && nt.conn.IsConnected()
}

// NATSConn is a net.Conn implementation for a NATS-based connection
type NATSConn struct {
	transport  *NATSTransport
	id         string
	localAddr  net.Addr
	remoteAddr net.Addr
	readCh     chan []byte
	closeCh    chan struct{}
	closed     bool
	mu         sync.Mutex
}

func (c *NATSConn) Read(b []byte) (n int, err error) {
	select {
	case data := <-c.readCh:
		n = copy(b, data)
		return n, nil
	case <-c.closeCh:
		return 0, io.EOF
	}
}

func (c *NATSConn) Write(b []byte) (n int, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}

	msg := &transport.Message{
		ID:   c.id,
		Data: b,
	}

	// The subject should be something like "p.data.to_server.{connID}"
	subject := "p.data.to_server." + c.id
	err = c.transport.Publish(context.Background(), subject, msg)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *NATSConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.closeCh)
	// Also need to notify the other side
	return nil
}

func (c *NATSConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *NATSConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *NATSConn) SetDeadline(t time.Time) error {
	return nil // Not implemented
}

func (c *NATSConn) SetReadDeadline(t time.Time) error {
	return nil // Not implemented
}

func (c *NATSConn) SetWriteDeadline(t time.Time) error {
	return nil // Not implemented
}

func (nt *NATSTransport) Dial(ctx context.Context, proxyAddr, remoteAddr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid remote address: %w", err)
	}

	reqMsg := &transport.Message{
		Metadata: map[string]string{
			"remote_host": host,
			"remote_port": port,
		},
	}

	// The server listens on "proxy.request"
	subject := "proxy.request"

	respMsg, err := nt.Request(ctx, subject, reqMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to request proxy connection: %w", err)
	}

	connID := string(respMsg.Data)

	// Create a NATSConn
	conn := &NATSConn{
		transport:  nt,
		id:         connID,
		readCh:     make(chan []byte, 100),
		closeCh:    make(chan struct{}),
		localAddr:  &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}, // Dummy addr
		remoteAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}, // Dummy addr
	}

	// Subscribe to data messages for this connection
	dataSubject := "p.data.to_client." + connID
	sub, err := nt.conn.Subscribe(dataSubject, func(msg *nats.Msg) {
		var transportMsg transport.Message
		if err := json.Unmarshal(msg.Data, &transportMsg); err != nil {
			nt.logger.WithError(err).Error("Failed to unmarshal message")
			return
		}
		conn.readCh <- transportMsg.Data
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to data subject: %w", err)
	}

	// When connection is closed, unsubscribe
	go func() {
		<-conn.closeCh
		sub.Unsubscribe()

		// Also send a close notification to the server
		closeMsg := &transport.Message{
			ID:       conn.id,
			Metadata: map[string]string{"type": "close"},
		}
		closeSubject := "p.data.to_server." + conn.id
		nt.Publish(context.Background(), closeSubject, closeMsg)
	}()

	return conn, nil
}

func (nt *NATSTransport) Proxy(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
