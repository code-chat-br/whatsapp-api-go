package whatsapp

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	watypes "go.mau.fi/whatsmeow/types"
)

type PasskeyPairingState string

const (
	PasskeyStateIdle                 PasskeyPairingState = "IDLE"
	PasskeyStateFetchingChallenge    PasskeyPairingState = "FETCHING_CHALLENGE"
	PasskeyStateAwaitingAssertion    PasskeyPairingState = "AWAITING_ASSERTION"
	PasskeyStateSubmittingAssertion  PasskeyPairingState = "SUBMITTING_ASSERTION"
	PasskeyStateAwaitingConfirmation PasskeyPairingState = "AWAITING_CONFIRMATION"
	PasskeyStateConfirmationSent     PasskeyPairingState = "CONFIRMATION_SENT"
	PasskeyStateCompleted            PasskeyPairingState = "COMPLETED"
	PasskeyStateFailed               PasskeyPairingState = "FAILED"
	PasskeyStateExpired              PasskeyPairingState = "EXPIRED"
)

type passkeyPairingData struct {
	State PasskeyPairingState

	RequestID string

	PublicKey *watypes.WebAuthnPublicKey

	CreatedAt time.Time
	ExpiresAt time.Time

	Consumed bool

	ConfirmationCode string
	SkipHandoffUX    bool

	LastError string
}

type passkeyChallengeSnapshot struct {
	RequestID string
	State     PasskeyPairingState
	PublicKey *watypes.WebAuthnPublicKey
	ExpiresAt time.Time
	Consumed  bool
}

type pairingSession struct {
	cancel    context.CancelFunc
	ctx       context.Context
	startedAt time.Time

	passkeyCommandMu sync.Mutex

	mu        sync.RWMutex
	currentQR *QRCodeConnectionResult
	passkey   passkeyPairingData
}

func (s *pairingSession) setCurrentQR(result QRCodeConnectionResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentQR = &result
}

func (s *pairingSession) getCurrentQR() *QRCodeConnectionResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentQR == nil {
		return nil
	}
	result := *s.currentQR
	return &result
}

func (s *pairingSession) setPasskeyChallenge(publicKey *watypes.WebAuthnPublicKey) passkeyChallengeSnapshot {
	now := time.Now().UTC()
	expiresAt := now.Add(getPasskeyTTL(publicKey))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey = passkeyPairingData{
		State:     PasskeyStateAwaitingAssertion,
		RequestID: uuid.NewString(),
		PublicKey: publicKey,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	return passkeyChallengeSnapshot{
		RequestID: s.passkey.RequestID,
		State:     s.passkey.State,
		PublicKey: s.passkey.PublicKey,
		ExpiresAt: s.passkey.ExpiresAt,
		Consumed:  s.passkey.Consumed,
	}
}

func (s *pairingSession) beginPasskeyChallengeFetch() (passkeyChallengeSnapshot, bool, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.passkey.PublicKey != nil && s.passkey.State == PasskeyStateAwaitingAssertion && !s.passkey.Consumed {
		if now.Before(s.passkey.ExpiresAt) {
			return passkeyChallengeSnapshot{
				RequestID: s.passkey.RequestID,
				State:     s.passkey.State,
				PublicKey: s.passkey.PublicKey,
				ExpiresAt: s.passkey.ExpiresAt,
				Consumed:  s.passkey.Consumed,
			}, true, nil
		}
		s.passkey.State = PasskeyStateExpired
	}
	switch s.passkey.State {
	case PasskeyStateIdle, PasskeyStateExpired, PasskeyStateFailed, "":
		s.passkey = passkeyPairingData{State: PasskeyStateFetchingChallenge}
		return passkeyChallengeSnapshot{}, false, nil
	default:
		return passkeyChallengeSnapshot{}, false, ErrInvalidPairingState
	}
}

func (s *pairingSession) currentPasskeyChallenge() (passkeyChallengeSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.passkey.PublicKey == nil || s.passkey.RequestID == "" {
		return passkeyChallengeSnapshot{}, false
	}
	return passkeyChallengeSnapshot{
		RequestID: s.passkey.RequestID,
		State:     s.passkey.State,
		PublicKey: s.passkey.PublicKey,
		ExpiresAt: s.passkey.ExpiresAt,
		Consumed:  s.passkey.Consumed,
	}, true
}

func (s *pairingSession) reservePasskeyAssertion(requestID string, assertion watypes.WebAuthnResponse) error {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.passkey.RequestID == "" || s.passkey.PublicKey == nil {
		return ErrPairingSessionNotActive
	}
	if s.passkey.RequestID != requestID {
		return ErrPasskeyRequestMismatch
	}
	if s.passkey.State != PasskeyStateAwaitingAssertion {
		return ErrInvalidPairingState
	}
	if s.passkey.Consumed {
		return ErrPasskeyChallengeAlreadyUsed
	}
	if !now.Before(s.passkey.ExpiresAt) {
		s.passkey.State = PasskeyStateExpired
		return ErrPasskeyChallengeExpired
	}
	if !validPasskeyAssertion(assertion) {
		return ErrInvalidPasskeyAssertion
	}
	s.passkey.Consumed = true
	s.passkey.State = PasskeyStateSubmittingAssertion
	return nil
}

func (s *pairingSession) markPasskeyAwaitingConfirmation() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey.State = PasskeyStateAwaitingConfirmation
}

func (s *pairingSession) setPasskeyConfirmation(code string, skipHandoffUX bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey.State = PasskeyStateAwaitingConfirmation
	s.passkey.ConfirmationCode = code
	s.passkey.SkipHandoffUX = skipHandoffUX
}

func (s *pairingSession) markPasskeyConfirmationSent() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey.State = PasskeyStateConfirmationSent
}

func (s *pairingSession) markPasskeyCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey.State = PasskeyStateCompleted
}

func (s *pairingSession) markPasskeyFailed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passkey.State = PasskeyStateFailed
	if err != nil {
		s.passkey.LastError = err.Error()
	}
}

func (s *pairingSession) isPasskeyInProgress() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch s.passkey.State {
	case PasskeyStateFetchingChallenge,
		PasskeyStateAwaitingAssertion,
		PasskeyStateSubmittingAssertion,
		PasskeyStateAwaitingConfirmation,
		PasskeyStateConfirmationSent:
		return true
	default:
		return false
	}
}

func getPasskeyTTL(publicKey *watypes.WebAuthnPublicKey) time.Duration {
	if publicKey == nil {
		return 10 * time.Minute
	}
	ttl := time.Duration(publicKey.Timeout) * time.Millisecond
	if ttl <= 0 {
		return 10 * time.Minute
	}
	if ttl > 15*time.Minute {
		return 15 * time.Minute
	}
	return ttl
}

func validPasskeyAssertion(assertion watypes.WebAuthnResponse) bool {
	return assertion.Type == "public-key" &&
		assertion.ID != "" &&
		len(assertion.RawID) > 0 &&
		len(assertion.Response.ClientDataJSON) > 0 &&
		len(assertion.Response.AuthenticatorData) > 0 &&
		len(assertion.Response.Signature) > 0
}

type pairingManager struct {
	mu       sync.RWMutex
	sessions map[string]*pairingSession
}

func newPairingManager() *pairingManager {
	return &pairingManager{sessions: make(map[string]*pairingSession)}
}

func (m *pairingManager) add(instanceID string, session *pairingSession) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sessions[instanceID]; ok {
		return false
	}
	m.sessions[instanceID] = session
	return true
}

func (m *pairingManager) remove(instanceID string, session *pairingSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if current, ok := m.sessions[instanceID]; ok && current == session {
		delete(m.sessions, instanceID)
	}
}

func (m *pairingManager) get(instanceID string) (*pairingSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[instanceID]
	return session, ok
}

func (m *pairingManager) exists(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.sessions[instanceID]
	return ok
}

func (m *pairingManager) cancelAll() {
	m.mu.RLock()
	sessions := make([]*pairingSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()
	for _, session := range sessions {
		if session.cancel != nil {
			session.cancel()
		}
	}
}
