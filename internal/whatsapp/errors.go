package whatsapp

import "errors"

var (
	ErrInvalidInstanceToken = errors.New("invalid instance token")
	ErrConnectionInProgress = errors.New("connection in progress")
	ErrInstanceConnected    = errors.New("instance already connected")
	ErrQRCodeTimeout        = errors.New("qr code timeout")
	ErrQRChannelClosed      = errors.New("qr channel closed")
	ErrPairingFailed        = errors.New("whatsapp pairing failed")
	ErrClientOutdated       = errors.New("whatsapp client outdated")
	ErrWhatsAppUnavailable  = errors.New("whatsapp unavailable")
	ErrInvalidPhoneNumber   = errors.New("invalid phone number")
	ErrSessionMissing       = errors.New("session missing")
	ErrClientNotConnected   = errors.New("client not connected")
	ErrInstanceInactive     = errors.New("instance inactive")
	ErrDeviceMismatch       = errors.New("whatsapp device does not match instance")

	ErrPasskeyInstanceNotFound     = errors.New("passkey instance not found")
	ErrPairingSessionNotFound      = errors.New("pairing session not found")
	ErrPairingSessionNotActive     = errors.New("pairing session not active")
	ErrInvalidPairingState         = errors.New("invalid pairing state")
	ErrPasskeyRequestMismatch      = errors.New("passkey request mismatch")
	ErrPasskeyChallengeAlreadyUsed = errors.New("passkey challenge already used")
	ErrPasskeyChallengeExpired     = errors.New("passkey challenge expired")
	ErrInvalidPasskeyAssertion     = errors.New("invalid passkey assertion")
	ErrPasskeyNotAvailable         = errors.New("passkey not available")
	ErrPasskeyServiceUnavailable   = errors.New("passkey service unavailable")
)
