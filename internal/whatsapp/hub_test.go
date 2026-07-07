package whatsapp

import (
	"errors"
	"testing"
)

func TestClientHubPreventsDuplicateReserveAndRegister(t *testing.T) {
	hub := NewClientHub()
	if err := hub.Reserve("1", "test_001"); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := hub.Reserve("1", "test_001"); !errors.Is(err, ErrConnectionInProgress) {
		t.Fatalf("expected ErrConnectionInProgress, got %v", err)
	}

	managed := &ManagedWhatsAppClient{InstanceID: "1", InstanceName: "test_001"}
	if err := hub.Register(managed); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := hub.Reserve("1", "test_001"); !errors.Is(err, ErrInstanceConnected) {
		t.Fatalf("expected ErrInstanceConnected, got %v", err)
	}
	if err := hub.Register(managed); !errors.Is(err, ErrInstanceConnected) {
		t.Fatalf("expected duplicate Register ErrInstanceConnected, got %v", err)
	}
}
