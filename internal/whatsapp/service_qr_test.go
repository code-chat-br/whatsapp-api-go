package whatsapp

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"

	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/database/repository"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

func TestConsumeQRCodeChannelReturnsFirstQRAndKeepsConsuming(t *testing.T) {
	svc, session, managed, qrChannel, firstQR, firstErr := newQRConsumerTest(t, 5)

	qrChannel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "first", Timeout: time.Minute}
	result := readFirstQR(t, firstQR)
	if result.Count != 1 || result.Code != "first" || !strings.HasPrefix(result.Base64, "data:image/png;base64,") {
		t.Fatalf("unexpected first QR result %#v", result)
	}
	if current := session.getCurrentQR(); current == nil || current.Code != "first" {
		t.Fatal("expected current QR to be stored in session")
	}

	qrChannel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "second", Timeout: time.Minute}
	qrChannel <- whatsmeow.QRChannelSuccess
	assertNoFirstError(t, firstErr)
	waitForPairingRemoved(t, svc, managed.InstanceID)

	if current := session.getCurrentQR(); current == nil || current.Code != "second" {
		t.Fatal("expected current QR to be updated by later QR events")
	}
	if got := svc.instances.(*fakeInstanceRepository).lastStatus(); got != types.InstanceConnectionStatusOnline {
		t.Fatalf("expected online after success, got %s", got)
	}
}

func TestConsumeQRCodeChannelTimeoutAfterFirstQRDoesNotBlockFirstError(t *testing.T) {
	svc, _, managed, qrChannel, firstQR, firstErr := newQRConsumerTest(t, 5)

	qrChannel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "first", Timeout: time.Minute}
	_ = readFirstQR(t, firstQR)
	qrChannel <- whatsmeow.QRChannelTimeout
	assertNoFirstError(t, firstErr)
	waitForPairingRemoved(t, svc, managed.InstanceID)

	if got := svc.instances.(*fakeInstanceRepository).lastStatus(); got != types.InstanceConnectionStatusConnectionTimeout {
		t.Fatalf("expected timeout status, got %s", got)
	}
}

func TestConsumeQRCodeChannelErrorBeforeFirstQRReturnsRealError(t *testing.T) {
	svc, _, managed, qrChannel, _, firstErr := newQRConsumerTest(t, 5)

	cause := errors.New("pair rejected")
	qrChannel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventError, Error: cause}

	err := readFirstError(t, firstErr)
	if !errors.Is(err, ErrPairingFailed) || !errors.Is(err, cause) {
		t.Fatalf("expected wrapped pairing error, got %v", err)
	}
	waitForPairingRemoved(t, svc, managed.InstanceID)

	repo := svc.instances.(*fakeInstanceRepository)
	if got := repo.lastStatus(); got != types.InstanceConnectionStatusConnectionError {
		t.Fatalf("expected connection error, got %s", got)
	}
	if repo.lastError() != cause.Error() {
		t.Fatalf("expected stored cause %q, got %q", cause.Error(), repo.lastError())
	}
}

func TestConsumeQRCodeChannelClosedBeforeFirstQRReturnsClosedError(t *testing.T) {
	svc, _, managed, qrChannel, _, firstErr := newQRConsumerTest(t, 5)

	close(qrChannel)

	err := readFirstError(t, firstErr)
	if !errors.Is(err, ErrQRChannelClosed) {
		t.Fatalf("expected ErrQRChannelClosed, got %v", err)
	}
	waitForPairingRemoved(t, svc, managed.InstanceID)
	if got := svc.instances.(*fakeInstanceRepository).lastStatus(); got != types.InstanceConnectionStatusConnectionError {
		t.Fatalf("expected connection error, got %s", got)
	}
}

func TestConsumeQRCodeChannelClientOutdatedMapsStatus(t *testing.T) {
	svc, _, managed, qrChannel, _, firstErr := newQRConsumerTest(t, 5)

	qrChannel <- whatsmeow.QRChannelClientOutdated

	err := readFirstError(t, firstErr)
	if !errors.Is(err, ErrClientOutdated) {
		t.Fatalf("expected ErrClientOutdated, got %v", err)
	}
	waitForPairingRemoved(t, svc, managed.InstanceID)
	if got := svc.instances.(*fakeInstanceRepository).lastStatus(); got != types.InstanceConnectionStatusClientOutdated {
		t.Fatalf("expected client_outdated, got %s", got)
	}
}

func newQRConsumerTest(t *testing.T, limit int) (*Service, *pairingSession, *ManagedWhatsAppClient, chan whatsmeow.QRChannelItem, chan QRCodeConnectionResult, chan error) {
	t.Helper()
	qr, err := NewQRGenerator("#ffffff", "#198754")
	if err != nil {
		t.Fatalf("NewQRGenerator: %v", err)
	}
	appCtx, appCancel := context.WithCancel(context.Background())
	t.Cleanup(appCancel)
	svc := &Service{
		config: config.WhatsAppConfig{
			QRCodeLimit:          limit,
			QRCodeExpirationTime: time.Second,
			PairingTimeout:       time.Minute,
		},
		instances:     &fakeInstanceRepository{},
		hub:           NewClientHub(),
		lock:          &fakeConnectionLock{},
		qr:            qr,
		logger:        zerolog.Nop(),
		passkeyClient: newWhatsmeowPasskeyClient,
		appCtx:        appCtx,
		appCancel:     appCancel,
		pairings:      newPairingManager(),
	}
	ctx, cancel := context.WithCancel(appCtx)
	session := &pairingSession{cancel: cancel, ctx: ctx, startedAt: time.Now()}
	managed := &ManagedWhatsAppClient{
		InstanceID:      "1",
		InstanceName:    "codechat",
		Context:         ctx,
		Cancel:          cancel,
		ConnectedSignal: make(chan struct{}),
	}
	if !svc.pairings.add(managed.InstanceID, session) {
		t.Fatal("failed to add pairing session")
	}
	if err := svc.hub.Register(managed); err != nil {
		t.Fatalf("register managed client: %v", err)
	}
	qrChannel := make(chan whatsmeow.QRChannelItem, 8)
	firstQR := make(chan QRCodeConnectionResult, 1)
	firstErr := make(chan error, 1)
	go svc.consumeQRCodeChannel(ctx, cancel, session, managed, qrChannel, firstQR, firstErr)
	return svc, session, managed, qrChannel, firstQR, firstErr
}

func readFirstQR(t *testing.T, ch <-chan QRCodeConnectionResult) QRCodeConnectionResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first QR")
		return QRCodeConnectionResult{}
	}
}

func readFirstError(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first error")
		return nil
	}
}

func assertNoFirstError(t *testing.T, ch <-chan error) {
	t.Helper()
	select {
	case err := <-ch:
		t.Fatalf("unexpected first error: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
}

func waitForPairingRemoved(t *testing.T, svc *Service, instanceID string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pairing cleanup")
		case <-ticker.C:
			if !svc.pairings.exists(instanceID) {
				return
			}
		}
	}
}

type fakeInstanceRepository struct {
	mu      sync.Mutex
	updates []types.UpdateConnectionStateInput
	found   types.InstanceWithAuth
	findErr error
}

func (r *fakeInstanceRepository) Create(context.Context, types.CreateInstanceInput) (types.InstanceWithAuth, error) {
	return types.InstanceWithAuth{}, nil
}
func (r *fakeInstanceRepository) CreateTx(context.Context, *db.Queries, types.CreateInstanceInput) (types.InstanceWithAuth, error) {
	return types.InstanceWithAuth{}, nil
}
func (r *fakeInstanceRepository) FindByName(context.Context, string) (types.InstanceWithAuth, error) {
	if r.findErr != nil {
		return types.InstanceWithAuth{}, r.findErr
	}
	if r.found.Instance.Name != "" {
		return r.found, nil
	}
	return types.InstanceWithAuth{}, repository.ErrInstanceNotFound
}
func (r *fakeInstanceRepository) FindByNameTx(context.Context, *db.Queries, string) (types.InstanceWithAuth, error) {
	return types.InstanceWithAuth{}, repository.ErrInstanceNotFound
}
func (r *fakeInstanceRepository) ListDetails(context.Context, *string) ([]types.InstanceDetails, error) {
	return nil, nil
}
func (r *fakeInstanceRepository) FetchDetailsByName(context.Context, string) (types.InstanceDetails, error) {
	return types.InstanceDetails{}, nil
}
func (r *fakeInstanceRepository) FindAutoConnectInstances(context.Context) ([]types.Instance, error) {
	return nil, nil
}
func (r *fakeInstanceRepository) List(context.Context) ([]types.InstanceWithAuth, error) {
	return nil, nil
}
func (r *fakeInstanceRepository) Update(context.Context, int32, types.UpdateInstanceInput) (types.InstanceWithAuth, error) {
	return types.InstanceWithAuth{}, nil
}
func (r *fakeInstanceRepository) UpdateStatus(context.Context, int32, types.InstanceStatus) error {
	return nil
}
func (r *fakeInstanceRepository) UpdateConnectionState(_ context.Context, input types.UpdateConnectionStateInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.updates = append(r.updates, input)
	return nil
}
func (r *fakeInstanceRepository) SaveWhatsAppDevice(context.Context, types.SaveWhatsAppDeviceInput) error {
	return nil
}
func (r *fakeInstanceRepository) ClearWhatsAppDevice(context.Context, int32) error { return nil }
func (r *fakeInstanceRepository) UpdateProfilePicture(context.Context, int32, *string, *string) error {
	return nil
}
func (r *fakeInstanceRepository) TryAcquireConnectionLock(context.Context, string) (bool, error) {
	return true, nil
}
func (r *fakeInstanceRepository) ReleaseConnectionLock(context.Context, string) error { return nil }
func (r *fakeInstanceRepository) EnsureDeletable(context.Context, int32) error        { return nil }
func (r *fakeInstanceRepository) Delete(context.Context, int32, bool) error           { return nil }

func (r *fakeInstanceRepository) lastStatus() types.InstanceConnectionStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.updates) - 1; i >= 0; i-- {
		if r.updates[i].ConnectionStatus != nil {
			return *r.updates[i].ConnectionStatus
		}
	}
	return ""
}

func (r *fakeInstanceRepository) lastError() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.updates) - 1; i >= 0; i-- {
		if r.updates[i].LastConnectionError.Set && r.updates[i].LastConnectionError.Value != nil {
			return *r.updates[i].LastConnectionError.Value
		}
	}
	return ""
}

type fakeConnectionLock struct{}

func (l *fakeConnectionLock) TryAcquire(context.Context, string) (bool, error) { return true, nil }
func (l *fakeConnectionLock) Release(context.Context, string) error            { return nil }
