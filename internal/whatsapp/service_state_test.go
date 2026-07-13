package whatsapp

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	watypes "go.mau.fi/whatsmeow/types"

	"whatsapp-go-api/internal/database/types"
)

func TestLogoutKeepsInstanceActiveForReconnect(t *testing.T) {
	repo := &fakeInstanceRepository{
		found: types.InstanceWithAuth{
			Instance: types.Instance{
				ID:               1,
				Name:             "codechat",
				Status:           types.InstanceStatusOnline,
				ConnectionStatus: types.InstanceConnectionStatusOnline,
			},
			Auth: &types.Auth{Token: "token"},
		},
	}
	svc := &Service{
		instances: repo,
		hub:       NewClientHub(),
		lock:      &fakeConnectionLock{},
		logger:    zerolog.Nop(),
	}

	if _, err := svc.Logout(context.Background(), "codechat", "token"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	instance, err := svc.authenticateInstance(context.Background(), "codechat", "token")
	if err != nil {
		t.Fatalf("expected instance to remain authorized for reconnect, got %v", err)
	}
	if instance.Instance.Status != types.InstanceStatusOnline {
		t.Fatalf("expected base instance status ONLINE after logout, got %s", instance.Instance.Status)
	}
	if got := repo.lastStatus(); got != types.InstanceConnectionStatusLoggedOut {
		t.Fatalf("expected WhatsApp connection status logged_out, got %s", got)
	}
}

func TestManagedConnectionStatusDistinguishesSessionPresence(t *testing.T) {
	if got := managedConnectionStatus(nil); got != types.InstanceConnectionStatusSessionMissing {
		t.Fatalf("nil managed status = %s", got)
	}

	managed := &ManagedWhatsAppClient{Client: &whatsmeow.Client{}}
	if got := managedConnectionStatus(managed); got != types.InstanceConnectionStatusSessionMissing {
		t.Fatalf("client without store ID status = %s", got)
	}

	jid := mustParseJID(t, "553171714339.0:1@s.whatsapp.net")
	managed.Client.Store = &store.Device{ID: &jid}
	if got := managedConnectionStatus(managed); got != types.InstanceConnectionStatusDisconnected {
		t.Fatalf("client with store ID but disconnected status = %s", got)
	}
}

func TestValidateManagedDeviceRejectsDeviceMismatch(t *testing.T) {
	jid := mustParseJID(t, "553171714339.0:1@s.whatsapp.net")
	expected := "553197853327.0:1@s.whatsapp.net"
	managed := &ManagedWhatsAppClient{
		InstanceID:   "1",
		InstanceName: "test_001",
		Client:       &whatsmeow.Client{Store: &store.Device{ID: &jid}},
	}
	svc := &Service{logger: zerolog.Nop()}

	err := svc.validateManagedDevice(types.Instance{
		ID:                1,
		Name:              "test_001",
		WhatsAppDeviceJid: &expected,
	}, managed)
	if !errors.Is(err, ErrDeviceMismatch) {
		t.Fatalf("expected ErrDeviceMismatch, got %v", err)
	}
}

func TestValidateManagedDeviceAcceptsPersistedDeviceAndOwner(t *testing.T) {
	jid := mustParseJID(t, "553171714339.0:1@s.whatsapp.net")
	device := jid.String()
	owner := jid.ToNonAD().String()
	managed := &ManagedWhatsAppClient{
		InstanceID:   "1",
		InstanceName: "test_001",
		Client:       &whatsmeow.Client{Store: &store.Device{ID: &jid}},
	}
	svc := &Service{logger: zerolog.Nop()}

	err := svc.validateManagedDevice(types.Instance{
		ID:                1,
		Name:              "test_001",
		OwnerJid:          &owner,
		WhatsAppDeviceJid: &device,
		WhatsAppOwnerJid:  &owner,
	}, managed)
	if err != nil {
		t.Fatalf("validateManagedDevice() error = %v", err)
	}
	if managed.DeviceJID != device || managed.OwnerJID != owner {
		t.Fatalf("managed JIDs not synchronized: device=%q owner=%q", managed.DeviceJID, managed.OwnerJID)
	}
}

func mustParseJID(t *testing.T, raw string) watypes.JID {
	t.Helper()
	jid, err := watypes.ParseJID(raw)
	if err != nil {
		t.Fatalf("ParseJID(%q): %v", raw, err)
	}
	return jid
}
