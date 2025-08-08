package mocks

import (
	"context"
	"sync"

	"github.com/igorrius/tcp-sproxy/pkg/transport"
)

// MockTransport is a mock implementation of the Transport interface
type MockTransport struct {
	mu sync.RWMutex

	// Message channels for testing
	messageChannels map[string]chan *transport.Message

	// Function call tracking for testing
	ConnectCallCount     int
	DisconnectCallCount  int
	SendCallCount        int
	SubscribeCallCount   int
	RequestCallCount     int
	PublishCallCount     int
	CloseCallCount       int
	IsConnectedCallCount int

	// Return values for testing
	Connected       bool
	RequestResponse *transport.Message

	// Error simulation
	ConnectError    error
	DisconnectError error
	SendError       error
	SubscribeError  error
	RequestError    error
	PublishError    error
	CloseError      error

	// Capture parameters for verification
	LastConnectConfig    interface{}
	LastSendSubject      string
	LastSendMessage      *transport.Message
	LastSubscribeSubject string
	LastRequestSubject   string
	LastRequestMessage   *transport.Message
	LastPublishSubject   string
	LastPublishMessage   *transport.Message
}

// NewMockTransport creates a new mock transport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		messageChannels: make(map[string]chan *transport.Message),
		Connected:       true, // Default to connected
	}
}

// Connect implements Transport.Connect
func (mt *MockTransport) Connect(ctx context.Context, config interface{}) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.ConnectCallCount++
	mt.LastConnectConfig = config

	if mt.ConnectError != nil {
		return mt.ConnectError
	}

	mt.Connected = true
	return nil
}

// Disconnect implements Transport.Disconnect
func (mt *MockTransport) Disconnect(ctx context.Context) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.DisconnectCallCount++

	if mt.DisconnectError != nil {
		return mt.DisconnectError
	}

	mt.Connected = false
	return nil
}

// Send implements Transport.Send
func (mt *MockTransport) Send(ctx context.Context, subject string, message *transport.Message) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.SendCallCount++
	mt.LastSendSubject = subject
	mt.LastSendMessage = message

	if mt.SendError != nil {
		return mt.SendError
	}

	// If there's a channel for this subject, send the message
	if ch, ok := mt.messageChannels[subject]; ok {
		select {
		case ch <- message:
		default:
			// Channel is full or closed, ignore
		}
	}

	return nil
}

// Subscribe implements Transport.Subscribe
func (mt *MockTransport) Subscribe(ctx context.Context, subject string) (<-chan *transport.Message, error) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.SubscribeCallCount++
	mt.LastSubscribeSubject = subject

	if mt.SubscribeError != nil {
		return nil, mt.SubscribeError
	}

	// Create a new channel for this subject if it doesn't exist
	if _, ok := mt.messageChannels[subject]; !ok {
		mt.messageChannels[subject] = make(chan *transport.Message, 100)
	}

	return mt.messageChannels[subject], nil
}

// Request implements Transport.Request
func (mt *MockTransport) Request(ctx context.Context, subject string, message *transport.Message) (*transport.Message, error) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.RequestCallCount++
	mt.LastRequestSubject = subject
	mt.LastRequestMessage = message

	if mt.RequestError != nil {
		return nil, mt.RequestError
	}

	return mt.RequestResponse, nil
}

// Publish implements Transport.Publish
func (mt *MockTransport) Publish(ctx context.Context, subject string, message *transport.Message) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.PublishCallCount++
	mt.LastPublishSubject = subject
	mt.LastPublishMessage = message

	if mt.PublishError != nil {
		return mt.PublishError
	}

	// If there's a channel for this subject, send the message
	if ch, ok := mt.messageChannels[subject]; ok {
		select {
		case ch <- message:
		default:
			// Channel is full or closed, ignore
		}
	}

	return nil
}

// Close implements Transport.Close
func (mt *MockTransport) Close() error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.CloseCallCount++

	if mt.CloseError != nil {
		return mt.CloseError
	}

	mt.Connected = false

	// Close all message channels
	for subject, ch := range mt.messageChannels {
		close(ch)
		delete(mt.messageChannels, subject)
	}

	return nil
}

// IsConnected implements Transport.IsConnected
func (mt *MockTransport) IsConnected() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	mt.IsConnectedCallCount++
	return mt.Connected
}

// Reset resets the mock state
func (mt *MockTransport) Reset() {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Close all existing channels
	for subject, ch := range mt.messageChannels {
		close(ch)
		delete(mt.messageChannels, subject)
	}

	mt.messageChannels = make(map[string]chan *transport.Message)

	mt.ConnectCallCount = 0
	mt.DisconnectCallCount = 0
	mt.SendCallCount = 0
	mt.SubscribeCallCount = 0
	mt.RequestCallCount = 0
	mt.PublishCallCount = 0
	mt.CloseCallCount = 0
	mt.IsConnectedCallCount = 0

	mt.Connected = true
	mt.RequestResponse = nil

	mt.ConnectError = nil
	mt.DisconnectError = nil
	mt.SendError = nil
	mt.SubscribeError = nil
	mt.RequestError = nil
	mt.PublishError = nil
	mt.CloseError = nil

	mt.LastConnectConfig = nil
	mt.LastSendSubject = ""
	mt.LastSendMessage = nil
	mt.LastSubscribeSubject = ""
	mt.LastRequestSubject = ""
	mt.LastRequestMessage = nil
	mt.LastPublishSubject = ""
	mt.LastPublishMessage = nil
}

// SendMessageToSubscribers sends a message to all subscribers of a subject
func (mt *MockTransport) SendMessageToSubscribers(subject string, message *transport.Message) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	if ch, ok := mt.messageChannels[subject]; ok {
		select {
		case ch <- message:
		default:
			// Channel is full or closed, ignore
		}
	}
}
