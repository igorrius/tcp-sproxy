package valueobjects

import (
	"testing"
)

func TestNewConnectionID(t *testing.T) {
	id1 := NewConnectionID()
	id2 := NewConnectionID()

	if id1.IsZero() {
		t.Error("NewConnectionID should not return zero value")
	}

	if id2.IsZero() {
		t.Error("NewConnectionID should not return zero value")
	}

	if id1.Equals(id2) {
		t.Error("NewConnectionID should generate unique IDs")
	}
}

func TestNewConnectionIDFromString(t *testing.T) {
	// Test valid string
	id, err := NewConnectionIDFromString("test123")
	if err != nil {
		t.Errorf("NewConnectionIDFromString should not error for valid string: %v", err)
	}
	if id.String() != "test123" {
		t.Errorf("Expected 'test123', got '%s'", id.String())
	}

	// Test empty string
	_, err = NewConnectionIDFromString("")
	if err == nil {
		t.Error("NewConnectionIDFromString should error for empty string")
	}
}

func TestConnectionID_Equals(t *testing.T) {
	id1, _ := NewConnectionIDFromString("test1")
	id2, _ := NewConnectionIDFromString("test2")
	id3, _ := NewConnectionIDFromString("test1")

	if !id1.Equals(id1) {
		t.Error("ConnectionID should equal itself")
	}

	if id1.Equals(id2) {
		t.Error("Different ConnectionIDs should not be equal")
	}

	if !id1.Equals(id3) {
		t.Error("ConnectionIDs with same value should be equal")
	}
}

func TestConnectionID_IsZero(t *testing.T) {
	id := NewConnectionID()
	if id.IsZero() {
		t.Error("NewConnectionID should not be zero")
	}

	var zeroID ConnectionID
	if !zeroID.IsZero() {
		t.Error("Zero ConnectionID should be zero")
	}
}

func TestConnectionID_String(t *testing.T) {
	expected := "test123"
	id, _ := NewConnectionIDFromString(expected)
	if id.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, id.String())
	}
}
