package response

import (
	"encoding/json"
	"time"

	watypes "go.mau.fi/whatsmeow/types"

	"whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/whatsapp"
)

type CreateInstanceResponse struct {
	ID          int32                      `json:"id"`
	Name        string                     `json:"name"`
	Description *string                    `json:"description"`
	CreatedAt   *time.Time                 `json:"createdAt"`
	UpdatedAt   *time.Time                 `json:"updatedAt"`
	Auth        CreateInstanceAuthResponse `json:"Auth"`
}

type CreateInstanceAuthResponse struct {
	ID    int32  `json:"id"`
	Token string `json:"token"`
}

type InstanceListItemResponse struct {
	ID               int32                          `json:"id"`
	Name             string                         `json:"name"`
	Description      *string                        `json:"description"`
	Status           types.InstanceStatus           `json:"status"`
	ConnectionStatus types.InstanceConnectionStatus `json:"connectionStatus"`
	OwnerJid         *string                        `json:"ownerJid"`
	ProfilePicURL    *string                        `json:"profilePicUrl"`
	CreatedAt        *time.Time                     `json:"createdAt"`
	UpdatedAt        *time.Time                     `json:"updatedAt"`
	Auth             *InstanceAuthResponse          `json:"Auth"`
	Webhook          *WebhookResponse               `json:"Webhook"`
}

type InstanceFetchResponse struct {
	ID               int32                          `json:"id"`
	Name             string                         `json:"name"`
	Description      *string                        `json:"description"`
	Status           types.InstanceStatus           `json:"status"`
	ConnectionStatus types.InstanceConnectionStatus `json:"connectionStatus"`
	OwnerJid         *string                        `json:"ownerJid"`
	ProfilePicURL    *string                        `json:"profilePicUrl"`
	CreatedAt        *time.Time                     `json:"createdAt"`
	UpdatedAt        *time.Time                     `json:"updatedAt"`
	Webhook          *WebhookResponse               `json:"Webhook"`
}

type InstanceAuthResponse struct {
	ID        int32     `json:"id"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WebhookResponse struct {
	ID         int32           `json:"id"`
	URL        string          `json:"url"`
	Enabled    bool            `json:"enabled"`
	Events     json.RawMessage `json:"events"`
	CreatedAt  *time.Time      `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	InstanceID int32           `json:"instanceId"`
}

type RefreshInstanceTokenResponse struct {
	ID         int32     `json:"id"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Token      string    `json:"token"`
	InstanceID int32     `json:"instanceId"`
}

type QRCodeConnectionResponse struct {
	Count             int     `json:"count,omitempty"`
	Code              string  `json:"code,omitempty"`
	Base64            string  `json:"base64,omitempty"`
	InstanceName      string  `json:"instanceName,omitempty"`
	ConnectionStatus  string  `json:"connectionStatus,omitempty"`
	AlreadyConnected  bool    `json:"alreadyConnected,omitempty"`
	AlreadyConnecting bool    `json:"alreadyConnecting,omitempty"`
	OwnerJid          *string `json:"ownerJid,omitempty"`
}

type PhonePairingResponse struct {
	Code string `json:"code"`
}

type PasskeyChallengeResponse struct {
	RequestID string                       `json:"requestId"`
	State     whatsapp.PasskeyPairingState `json:"state"`
	ExpiresAt time.Time                    `json:"expiresAt"`
	PublicKey *watypes.WebAuthnPublicKey   `json:"publicKey"`
}

type PasskeyAssertionResponse struct {
	State   whatsapp.PasskeyPairingState `json:"state"`
	Message string                       `json:"message"`
}

type ConnectionStateResponse struct {
	State            string  `json:"state"`
	StatusReason     int     `json:"statusReason"`
	InstanceName     string  `json:"instanceName"`
	ConnectionStatus string  `json:"connectionStatus"`
	Connected        bool    `json:"connected"`
	LoggedIn         bool    `json:"loggedIn"`
	OwnerJid         *string `json:"ownerJid"`
}

type InstanceLogoutResponse struct {
	InstanceName     string `json:"instanceName"`
	State            string `json:"state"`
	ConnectionStatus string `json:"connectionStatus"`
	Message          string `json:"message"`
}

type InstanceDeleteResponse struct {
	InstanceName string `json:"instanceName"`
	Deleted      bool   `json:"deleted"`
	Forced       bool   `json:"forced"`
	Message      string `json:"message"`
}
