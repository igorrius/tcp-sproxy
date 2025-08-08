package valueobjects

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// ConnectionID represents a unique identifier for a connection
type ConnectionID struct {
	value string
}

// NewConnectionID creates a new connection ID
func NewConnectionID() ConnectionID {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return ConnectionID{
		value: hex.EncodeToString(bytes),
	}
}

// NewConnectionIDFromString creates a connection ID from a string
func NewConnectionIDFromString(value string) (ConnectionID, error) {
	if len(value) == 0 {
		return ConnectionID{}, fmt.Errorf("connection ID cannot be empty")
	}
	return ConnectionID{value: value}, nil
}

// String returns the string representation of the connection ID
func (c ConnectionID) String() string {
	return c.value
}

// Equals checks if two connection IDs are equal
func (c ConnectionID) Equals(other ConnectionID) bool {
	return c.value == other.value
}

// IsZero checks if the connection ID is zero value
func (c ConnectionID) IsZero() bool {
	return c.value == ""
}
