package transport

import (
	"context"
	"encoding/json"
	"fmt"
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
func NewNATSTransport() *NATSTransport {
	return &NATSTransport{
		subs:    make(map[string]*nats.Subscription),
		msgChan: make(chan *transport.Message, 100),
		logger:  logrus.WithField("transport", "nats"),
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

	// Send request with timeout
	timeout := nt.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	response, err := nt.conn.Request(subject, data, timeout)
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
