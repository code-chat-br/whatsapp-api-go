package types

import (
	"encoding/json"
	"time"
)

type InstanceStatus string

const (
	InstanceStatusOnline  InstanceStatus = "ONLINE"
	InstanceStatusOffline InstanceStatus = "OFFLINE"
)

func (s InstanceStatus) IsValid() bool {
	return s == InstanceStatusOnline || s == InstanceStatusOffline
}

type InstanceConnectionStatus string

const (
	InstanceConnectionStatusOffline           InstanceConnectionStatus = "offline"
	InstanceConnectionStatusConnecting        InstanceConnectionStatus = "connecting"
	InstanceConnectionStatusQRCode            InstanceConnectionStatus = "qr_code"
	InstanceConnectionStatusPairingCode       InstanceConnectionStatus = "pairing_code"
	InstanceConnectionStatusPairing           InstanceConnectionStatus = "pairing"
	InstanceConnectionStatusOnline            InstanceConnectionStatus = "online"
	InstanceConnectionStatusReconnecting      InstanceConnectionStatus = "reconnecting"
	InstanceConnectionStatusDisconnected      InstanceConnectionStatus = "disconnected"
	InstanceConnectionStatusConnectionTimeout InstanceConnectionStatus = "connection_timeout"
	InstanceConnectionStatusLoggedOut         InstanceConnectionStatus = "logged_out"
	InstanceConnectionStatusSessionMissing    InstanceConnectionStatus = "session_missing"
	InstanceConnectionStatusStreamReplaced    InstanceConnectionStatus = "stream_replaced"
	InstanceConnectionStatusKeepAliveTimeout  InstanceConnectionStatus = "keepalive_timeout"
	InstanceConnectionStatusClientOutdated    InstanceConnectionStatus = "client_outdated"
	InstanceConnectionStatusTemporaryBan      InstanceConnectionStatus = "temporary_ban"
	InstanceConnectionStatusConnectionError   InstanceConnectionStatus = "connection_error"
)

func (s InstanceConnectionStatus) IsValid() bool {
	switch s {
	case InstanceConnectionStatusOffline,
		InstanceConnectionStatusConnecting,
		InstanceConnectionStatusQRCode,
		InstanceConnectionStatusPairingCode,
		InstanceConnectionStatusPairing,
		InstanceConnectionStatusOnline,
		InstanceConnectionStatusReconnecting,
		InstanceConnectionStatusDisconnected,
		InstanceConnectionStatusConnectionTimeout,
		InstanceConnectionStatusLoggedOut,
		InstanceConnectionStatusSessionMissing,
		InstanceConnectionStatusStreamReplaced,
		InstanceConnectionStatusKeepAliveTimeout,
		InstanceConnectionStatusClientOutdated,
		InstanceConnectionStatusTemporaryBan,
		InstanceConnectionStatusConnectionError:
		return true
	default:
		return false
	}
}

type Instance struct {
	ID                 int32                    `json:"id"`
	Name               string                   `json:"name"`
	Description        *string                  `json:"description"`
	Status             InstanceStatus           `json:"status"`
	ConnectionStatus   InstanceConnectionStatus `json:"connectionStatus"`
	OwnerJid           *string                  `json:"ownerJid"`
	ProfilePicUrl      *string                  `json:"profilePicUrl"`
	WhatsAppDeviceJid  *string                  `json:"whatsappDeviceJid"`
	WhatsAppOwnerJid   *string                  `json:"whatsappOwnerJid"`
	WhatsAppPhone      *string                  `json:"whatsappPhoneNumber"`
	ProfilePicID       *string                  `json:"profilePicId"`
	LastConnectedAt    *time.Time               `json:"lastConnectedAt"`
	LastDisconnectedAt *time.Time               `json:"lastDisconnectedAt"`
	LastAttemptAt      *time.Time               `json:"lastConnectionAttemptAt"`
	LastError          *string                  `json:"lastConnectionError"`
	LastEvent          *string                  `json:"lastConnectionEvent"`
	ConnectionAttempts int32                    `json:"connectionAttempts"`
	CreatedAt          time.Time                `json:"createdAt"`
	UpdatedAt          time.Time                `json:"updatedAt"`
	ExternalAttributes json.RawMessage          `json:"externalAttributes"`
}

type Auth struct {
	ID         int32     `json:"id"`
	Token      string    `json:"token"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	InstanceID int32     `json:"instanceId"`
}

type InstanceWithAuth struct {
	Instance Instance `json:"instance"`
	Auth     *Auth    `json:"auth"`
}

type InstanceDetails struct {
	Instance Instance `json:"instance"`
	Auth     *Auth    `json:"auth"`
	Webhook  *Webhook `json:"webhook"`
}

type CreateInstanceInput struct {
	Name               string
	Description        *string
	Status             *InstanceStatus
	OwnerJid           *string
	ProfilePicUrl      *string
	ExternalAttributes json.RawMessage
}

type UpdateInstanceInput struct {
	Name               *string
	Description        OptionalField[string]
	ProfilePicUrl      OptionalField[string]
	ExternalAttributes OptionalField[json.RawMessage]
}

type CreateAuthInput struct {
	Token      string
	InstanceID int32
}

type UpdateAuthTokenInput struct {
	AuthID   int32
	NewToken string
}

type UpdateAuthTokenConditionInput struct {
	InstanceID int32
	OldToken   string
	NewToken   string
}

type UpdateConnectionStateInput struct {
	InstanceID              int32
	ConnectionStatus        *InstanceConnectionStatus
	LastConnectedAt         *time.Time
	LastDisconnectedAt      *time.Time
	LastConnectionAttemptAt *time.Time
	LastConnectionError     OptionalField[string]
	LastConnectionEvent     OptionalField[string]
	IncrementAttempts       bool
	ResetAttempts           bool
}

type SaveWhatsAppDeviceInput struct {
	InstanceID  int32
	DeviceJID   string
	OwnerJID    string
	PhoneNumber string
}
