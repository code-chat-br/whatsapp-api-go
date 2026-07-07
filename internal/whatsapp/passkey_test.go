package whatsapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsapp-go-api/internal/config"
	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestRequestPasskeyChallengeReturnsStoredChallenge(t *testing.T) {
	svc, session, client := newPasskeyServiceTest(t)
	stored := session.setPasskeyChallenge(testPublicKey(300000))

	result, err := svc.RequestPasskeyChallenge(context.Background(), "codechat", "token")
	if err != nil {
		t.Fatalf("RequestPasskeyChallenge() error = %v", err)
	}
	if result.RequestID != stored.RequestID || result.State != PasskeyStateAwaitingAssertion {
		t.Fatalf("unexpected stored challenge result %#v", result)
	}
	if client.getChallengeCalls != 0 {
		t.Fatalf("expected stored challenge reuse, got %d fetches", client.getChallengeCalls)
	}
}

func TestRequestPasskeyChallengeFetchesExplicitly(t *testing.T) {
	svc, session, client := newPasskeyServiceTest(t)

	result, err := svc.RequestPasskeyChallenge(context.Background(), "codechat", "token")
	if err != nil {
		t.Fatalf("RequestPasskeyChallenge() error = %v", err)
	}
	if result.RequestID == "" || result.PublicKey == nil || result.State != PasskeyStateAwaitingAssertion {
		t.Fatalf("unexpected challenge result %#v", result)
	}
	if client.getChallengeCalls != 1 {
		t.Fatalf("expected one explicit fetch, got %d", client.getChallengeCalls)
	}
	if snapshot, ok := session.currentPasskeyChallenge(); !ok || snapshot.RequestID != result.RequestID {
		t.Fatalf("expected session challenge to be stored, ok=%v snapshot=%#v", ok, snapshot)
	}
}

func TestSubmitPasskeyAssertionConsumesBeforeSendAndRejectsRetry(t *testing.T) {
	svc, session, client := newPasskeyServiceTest(t)
	challenge := session.setPasskeyChallenge(testPublicKey(300000))
	client.onSendResponse = func() {
		snapshot, ok := session.currentPasskeyChallenge()
		if !ok || !snapshot.Consumed || snapshot.State != PasskeyStateSubmittingAssertion {
			t.Fatalf("challenge must be consumed before SendPasskeyResponse, ok=%v snapshot=%#v", ok, snapshot)
		}
	}

	result, err := svc.SubmitPasskeyAssertion(context.Background(), "codechat", "token", SubmitPasskeyAssertionRequest{
		RequestID: challenge.RequestID,
		Assertion: testAssertion(),
	})
	if err != nil {
		t.Fatalf("SubmitPasskeyAssertion() error = %v", err)
	}
	if result.State != PasskeyStateAwaitingConfirmation || client.sendResponseCalls != 1 {
		t.Fatalf("unexpected assertion result=%#v calls=%d", result, client.sendResponseCalls)
	}

	_, err = svc.SubmitPasskeyAssertion(context.Background(), "codechat", "token", SubmitPasskeyAssertionRequest{
		RequestID: challenge.RequestID,
		Assertion: testAssertion(),
	})
	if !errors.Is(err, ErrInvalidPairingState) {
		t.Fatalf("expected retry rejection, got %v", err)
	}
	if client.sendResponseCalls != 1 {
		t.Fatalf("retry must not call SendPasskeyResponse, got %d calls", client.sendResponseCalls)
	}
}

func TestSubmitPasskeyAssertionRejectsMismatchExpiredAndInvalid(t *testing.T) {
	svc, session, _ := newPasskeyServiceTest(t)
	challenge := session.setPasskeyChallenge(testPublicKey(300000))
	if _, err := svc.SubmitPasskeyAssertion(context.Background(), "codechat", "token", SubmitPasskeyAssertionRequest{
		RequestID: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa",
		Assertion: testAssertion(),
	}); !errors.Is(err, ErrPasskeyRequestMismatch) {
		t.Fatalf("expected request mismatch, got %v", err)
	}

	session.mu.Lock()
	session.passkey.ExpiresAt = time.Now().UTC().Add(-time.Second)
	session.mu.Unlock()
	if _, err := svc.SubmitPasskeyAssertion(context.Background(), "codechat", "token", SubmitPasskeyAssertionRequest{
		RequestID: challenge.RequestID,
		Assertion: testAssertion(),
	}); !errors.Is(err, ErrPasskeyChallengeExpired) {
		t.Fatalf("expected expired challenge, got %v", err)
	}

	challenge = session.setPasskeyChallenge(testPublicKey(300000))
	if _, err := svc.SubmitPasskeyAssertion(context.Background(), "codechat", "token", SubmitPasskeyAssertionRequest{
		RequestID: challenge.RequestID,
		Assertion: watypes.WebAuthnResponse{Type: "public-key"},
	}); !errors.Is(err, ErrInvalidPasskeyAssertion) {
		t.Fatalf("expected invalid assertion, got %v", err)
	}
}

func TestHandlePasskeyConfirmationHonorsSkipHandoff(t *testing.T) {
	svc, session, client := newPasskeyServiceTest(t)

	if err := svc.handlePasskeyConfirmation(context.Background(), session, testManagedClient(), "ABCD-EFGH", true); err != nil {
		t.Fatalf("skip confirmation error = %v", err)
	}
	if client.sendConfirmationCalls != 0 {
		t.Fatalf("skip handoff must not call confirmation, got %d", client.sendConfirmationCalls)
	}

	if err := svc.handlePasskeyConfirmation(context.Background(), session, testManagedClient(), "ABCD-EFGH", false); err != nil {
		t.Fatalf("confirmation error = %v", err)
	}
	if client.sendConfirmationCalls != 1 {
		t.Fatalf("expected one confirmation call, got %d", client.sendConfirmationCalls)
	}
	if session.passkey.State != PasskeyStateConfirmationSent {
		t.Fatalf("expected confirmation sent state, got %s", session.passkey.State)
	}
}

func TestConsumeQRCodeChannelStoresPasskeyChallengeAndIgnoresQRCodeLimit(t *testing.T) {
	svc, session, managed, qrChannel, _, firstErr := newQRConsumerTest(t, 1)

	qrChannel <- whatsmeow.QRChannelItem{
		Event:          whatsmeow.QRChannelEventPasskeyRequest,
		PasskeyRequest: &events.PairPasskeyRequest{PublicKey: testPublicKey(300000)},
	}
	qrChannel <- whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "ignored", Timeout: time.Millisecond}
	qrChannel <- whatsmeow.QRChannelSuccess

	assertNoFirstError(t, firstErr)
	waitForPairingRemoved(t, svc, managed.InstanceID)
	if snapshot, ok := session.currentPasskeyChallenge(); !ok || snapshot.RequestID == "" {
		t.Fatalf("expected passkey challenge from QR channel, ok=%v snapshot=%#v", ok, snapshot)
	}
	if currentQR := session.getCurrentQR(); currentQR != nil {
		t.Fatalf("QR update during passkey must be ignored, got %#v", currentQR)
	}
}

func TestConsumeQRCodeChannelRejectsPasskeyRequestWithoutPublicKey(t *testing.T) {
	svc, _, managed, qrChannel, _, firstErr := newQRConsumerTest(t, 5)

	qrChannel <- whatsmeow.QRChannelItem{
		Event:          whatsmeow.QRChannelEventPasskeyRequest,
		PasskeyRequest: &events.PairPasskeyRequest{},
	}

	err := readFirstError(t, firstErr)
	if !errors.Is(err, ErrPasskeyNotAvailable) {
		t.Fatalf("expected ErrPasskeyNotAvailable, got %v", err)
	}
	waitForPairingRemoved(t, svc, managed.InstanceID)
}

func newPasskeyServiceTest(t *testing.T) (*Service, *pairingSession, *fakePasskeyClient) {
	t.Helper()
	appCtx, appCancel := context.WithCancel(context.Background())
	t.Cleanup(appCancel)
	repo := &fakeInstanceRepository{
		found: dbtypes.InstanceWithAuth{
			Instance: dbtypes.Instance{ID: 1, Name: "codechat", Status: dbtypes.InstanceStatusOnline},
			Auth:     &dbtypes.Auth{Token: "token"},
		},
	}
	client := &fakePasskeyClient{connected: true, challenge: testPublicKey(300000)}
	svc := &Service{
		config:        config.WhatsAppConfig{PairingTimeout: time.Minute},
		instances:     repo,
		hub:           NewClientHub(),
		lock:          &fakeConnectionLock{},
		logger:        zerolog.Nop(),
		passkeyClient: func(*ManagedWhatsAppClient) PasskeyClient { return client },
		appCtx:        appCtx,
		appCancel:     appCancel,
		pairings:      newPairingManager(),
	}
	ctx, cancel := context.WithCancel(appCtx)
	t.Cleanup(cancel)
	session := &pairingSession{cancel: cancel, ctx: ctx, startedAt: time.Now()}
	if !svc.pairings.add("1", session) {
		t.Fatal("failed to add pairing session")
	}
	if err := svc.hub.Register(testManagedClient()); err != nil {
		t.Fatalf("register managed client: %v", err)
	}
	return svc, session, client
}

func testManagedClient() *ManagedWhatsAppClient {
	return &ManagedWhatsAppClient{
		InstanceID:      "1",
		InstanceName:    "codechat",
		ConnectedSignal: make(chan struct{}),
	}
}

func testPublicKey(timeout int) *watypes.WebAuthnPublicKey {
	return &watypes.WebAuthnPublicKey{
		Challenge:        []byte("challenge"),
		Timeout:          timeout,
		RelyingPartID:    "whatsapp.com",
		AllowCredentials: []watypes.AllowedCredential{{ID: []byte("credential"), Type: "public-key", Transports: []string{"internal", "hybrid"}}},
		UserVerification: "required",
		Extensions:       map[string]any{},
	}
}

func testAssertion() watypes.WebAuthnResponse {
	return watypes.WebAuthnResponse{
		ID:    "credential-id",
		RawID: []byte("raw-id"),
		Type:  "public-key",
		Response: watypes.WebAuthnResponseData{
			ClientDataJSON:    []byte("client-data"),
			AuthenticatorData: []byte("authenticator-data"),
			Signature:         []byte("signature"),
		},
	}
}

type fakePasskeyClient struct {
	connected bool
	loggedIn  bool
	linked    bool

	challenge *watypes.WebAuthnPublicKey

	getChallengeCalls     int
	sendResponseCalls     int
	sendConfirmationCalls int
	getChallengeErr       error
	sendResponseErr       error
	sendConfirmationErr   error
	onSendResponse        func()
}

func (c *fakePasskeyClient) IsConnected() bool { return c.connected }
func (c *fakePasskeyClient) IsLoggedIn() bool  { return c.loggedIn }
func (c *fakePasskeyClient) HasLinkedDevice() bool {
	return c.linked
}
func (c *fakePasskeyClient) GetPasskeyRequestOptions(context.Context) (*watypes.WebAuthnPublicKey, error) {
	c.getChallengeCalls++
	return c.challenge, c.getChallengeErr
}
func (c *fakePasskeyClient) SendPasskeyResponse(context.Context, *watypes.WebAuthnResponse) error {
	c.sendResponseCalls++
	if c.onSendResponse != nil {
		c.onSendResponse()
	}
	return c.sendResponseErr
}
func (c *fakePasskeyClient) SendPasskeyConfirmation(context.Context) error {
	c.sendConfirmationCalls++
	return c.sendConfirmationErr
}
