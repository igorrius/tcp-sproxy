package transport

import (
	"context"
)

// Message represents a transport message
type Message struct {
	ID       string
	Data     []byte
	Metadata map[string]string
}

// Transport defines the interface for abstract transport layer
type Transport interface {
	// Connect establishes a connection to the transport layer
	Connect(ctx context.Context, config interface{}) error

	// Disconnect closes the transport connection
	Disconnect(ctx context.Context) error

	// Send sends a message to a specific subject/topic
	Send(ctx context.Context, subject string, message *Message) error

	// Subscribe subscribes to a subject/topic and returns a channel for receiving messages
	Subscribe(ctx context.Context, subject string) (<-chan *Message, error)

	// Request sends a request and waits for a response
	Request(ctx context.Context, subject string, message *Message) (*Message, error)

	// Publish publishes a message to a subject/topic (fire and forget)
	Publish(ctx context.Context, subject string, message *Message) error

	// Close closes the transport
	Close() error

	// IsConnected returns true if the transport is connected
	IsConnected() bool
}

// TransportConfig represents configuration for transport
type TransportConfig struct {
	URL     string            `json:"url"`
	Options map[string]string `json:"options"`
	Timeout int               `json:"timeout"`
	Retries int               `json:"retries"`
}

// NewMessage creates a new transport message
func NewMessage(id string, data []byte) *Message {
	return &Message{
		ID:       id,
		Data:     data,
		Metadata: make(map[string]string),
	}
}

// NewMessageWithMetadata creates a new transport message with metadata
func NewMessageWithMetadata(id string, data []byte, metadata map[string]string) *Message {
	return &Message{
		ID:       id,
		Data:     data,
		Metadata: metadata,
	}
}
