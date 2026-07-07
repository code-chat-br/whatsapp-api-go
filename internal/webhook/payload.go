package webhook

import (
	"encoding/json"
	"time"

	"whatsapp-go-api/internal/database/types"
)

type WebhookInstance struct {
	ID                 int64          `json:"id"`
	Name               string         `json:"name"`
	ConnectionStatus   string         `json:"connectionStatus"`
	OwnerJID           *string        `json:"ownerJid"`
	ExternalAttributes map[string]any `json:"externalAttributes"`
}

type WebhookPayload struct {
	Event     types.WebhookEvent `json:"event"`
	Instance  WebhookInstance    `json:"instance"`
	Data      any                `json:"data"`
	Timestamp time.Time          `json:"timestamp"`
}

func NewWebhookInstance(instance types.Instance) WebhookInstance {
	return WebhookInstance{
		ID:                 int64(instance.ID),
		Name:               instance.Name,
		ConnectionStatus:   string(instance.ConnectionStatus),
		OwnerJID:           instance.WhatsAppOwnerJid,
		ExternalAttributes: externalAttributesMap(instance.ExternalAttributes),
	}
}

func externalAttributesMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded == nil {
		return map[string]any{}
	}
	return decoded
}

type ConnectionWebhookData struct {
	Type           string     `json:"type"`
	Connection     string     `json:"connection"`
	StatusReason   int        `json:"statusReason,omitempty"`
	LastConnection *time.Time `json:"lastConnection,omitempty"`
	Message        string     `json:"message,omitempty"`
}

const (
	ConnectionInternalPairSuccess          = "PairSuccess"
	ConnectionInternalConnected            = "Connected"
	ConnectionInternalDisconnected         = "Disconnected"
	ConnectionInternalLoggedOut            = "LoggedOut"
	ConnectionInternalStreamReplaced       = "StreamReplaced"
	ConnectionInternalKeepAliveTimeout     = "KeepAliveTimeout"
	ConnectionInternalKeepAliveRestored    = "KeepAliveRestored"
	ConnectionInternalConnectFailure       = "ConnectFailure"
	ConnectionInternalManualLoginReconnect = "ManualLoginReconnect"
	ConnectionInternalPairError            = "PairError"
	ConnectionInternalStreamError          = "StreamError"
	ConnectionInternalCATRefreshError      = "CATRefreshError"
)

func NormalizeConnectionWebhookData(internalEvent string, statusReason int, lastConnection *time.Time, message string) (ConnectionWebhookData, bool) {
	data := ConnectionWebhookData{
		StatusReason:   statusReason,
		LastConnection: lastConnection,
		Message:        message,
	}
	switch internalEvent {
	case ConnectionInternalPairSuccess:
		data.Type = "pair.success"
		data.Connection = "connecting"
	case ConnectionInternalConnected:
		data.Type = "connected"
		data.Connection = "open"
	case ConnectionInternalDisconnected:
		data.Type = "disconnected"
		data.Connection = "close"
	case ConnectionInternalLoggedOut:
		data.Type = "logged.out"
		data.Connection = "close"
	case ConnectionInternalStreamReplaced:
		data.Type = "stream.replaced"
		data.Connection = "replaced"
	case ConnectionInternalKeepAliveTimeout:
		data.Type = "keepalive.timeout"
		data.Connection = "timeout"
	case ConnectionInternalKeepAliveRestored:
		data.Type = "keepalive.restored"
		data.Connection = "open"
	case ConnectionInternalConnectFailure:
		data.Type = "connect.failure"
		data.Connection = "close"
	case ConnectionInternalManualLoginReconnect:
		data.Type = "manual.reconnect"
		data.Connection = "connecting"
	case ConnectionInternalPairError:
		data.Type = "pair.error"
		data.Connection = "close"
	case ConnectionInternalStreamError:
		data.Type = "stream.error"
		data.Connection = "close"
	case ConnectionInternalCATRefreshError:
		data.Type = "cat.refresh.error"
		data.Connection = "close"
	default:
		return ConnectionWebhookData{}, false
	}
	return data, true
}

type InstanceStatusWebhookData struct {
	Type    string `json:"type"`
	Status  string `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

type QRCodeUpdatedWebhookData struct {
	Count            int       `json:"count"`
	Code             string    `json:"code"`
	Base64           string    `json:"base64"`
	ExpiresInSeconds int64     `json:"expiresInSeconds"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type MessageUpsertWebhookData struct {
	ID                int64          `json:"id"`
	KeyRemoteJID      *string        `json:"keyRemoteJid"`
	KeyLID            *string        `json:"keyLid"`
	KeyFromMe         bool           `json:"keyFromMe"`
	KeyParticipant    *string        `json:"keyParticipant"`
	KeyParticipantLID *string        `json:"keyParticipantLid"`
	PushName          *string        `json:"pushName"`
	MessageType       string         `json:"messageType"`
	Content           map[string]any `json:"content"`
	MessageTimestamp  int64          `json:"messageTimestamp"`
	Device            *string        `json:"device"`
	IsGroup           bool           `json:"isGroup"`
	Metadata          map[string]any `json:"metadata"`
}

type MessageUpdateWebhookData struct {
	MessageID int64     `json:"messageId"`
	Status    string    `json:"status"`
	DateTime  time.Time `json:"dateTime"`
}

type MessageDeletedWebhookData struct {
	ChatJID      string     `json:"chatJid"`
	SenderJID    *string    `json:"senderJid,omitempty"`
	KeyFromMe    bool       `json:"keyFromMe"`
	MessageID    string     `json:"messageId"`
	DeleteMedia  bool       `json:"deleteMedia"`
	FromFullSync bool       `json:"fromFullSync"`
	DateTime     time.Time  `json:"dateTime"`
	MessageTime  *time.Time `json:"messageTime,omitempty"`
}

type MessageStarredWebhookData struct {
	ChatJID      string    `json:"chatJid"`
	SenderJID    *string   `json:"senderJid,omitempty"`
	KeyFromMe    bool      `json:"keyFromMe"`
	MessageID    string    `json:"messageId"`
	Starred      bool      `json:"starred"`
	FromFullSync bool      `json:"fromFullSync"`
	DateTime     time.Time `json:"dateTime"`
}

type MessageUndecryptableWebhookData struct {
	KeyID           string    `json:"keyId"`
	ChatJID         string    `json:"chatJid"`
	SenderJID       *string   `json:"senderJid,omitempty"`
	KeyFromMe       bool      `json:"keyFromMe"`
	IsUnavailable   bool      `json:"isUnavailable"`
	UnavailableType string    `json:"unavailableType,omitempty"`
	DecryptFailMode string    `json:"decryptFailMode,omitempty"`
	DateTime        time.Time `json:"dateTime"`
}

type ContactUpsertWebhookData struct {
	ID            int64   `json:"id"`
	RemoteJID     string  `json:"remoteJid"`
	LID           *string `json:"lid"`
	PushName      *string `json:"pushName"`
	ProfilePicURL *string `json:"profilePicUrl"`
	Action        string  `json:"action"`
}

type WebhookCallStatus string

const (
	WebhookCallStatusOffer        WebhookCallStatus = "offer"
	WebhookCallStatusRinging      WebhookCallStatus = "ringing"
	WebhookCallStatusPreAccept    WebhookCallStatus = "preaccept"
	WebhookCallStatusTransport    WebhookCallStatus = "transport"
	WebhookCallStatusRelayLatency WebhookCallStatus = "relaylatency"
	WebhookCallStatusTimeout      WebhookCallStatus = "timeout"
	WebhookCallStatusReject       WebhookCallStatus = "reject"
	WebhookCallStatusAccept       WebhookCallStatus = "accept"
	WebhookCallStatusTerminate    WebhookCallStatus = "terminate"
	WebhookCallStatusUnknown      WebhookCallStatus = "unknown"
)

type CallUpsertWebhookData struct {
	ChatID   string            `json:"chatId"`
	From     string            `json:"from"`
	CallerPN *string           `json:"callerPn"`
	IsGroup  *bool             `json:"isGroup"`
	GroupJID *string           `json:"groupJid"`
	ID       string            `json:"id"`
	Date     time.Time         `json:"date"`
	IsVideo  *bool             `json:"isVideo"`
	Status   WebhookCallStatus `json:"status"`
	Offline  bool              `json:"offline"`
	Latency  *int64            `json:"latencyMs"`
}

type ContactUpdateWebhookData struct {
	ID           int64   `json:"id"`
	RemoteJID    string  `json:"remoteJid"`
	LID          *string `json:"lid"`
	PushName     *string `json:"pushName"`
	BusinessName *string `json:"businessName,omitempty"`
	Action       string  `json:"action"`
	Source       string  `json:"source"`
}

type ProfilePictureUpdatedWebhookData struct {
	JID       string    `json:"jid"`
	Author    string    `json:"author,omitempty"`
	DateTime  time.Time `json:"dateTime"`
	Remove    bool      `json:"remove"`
	PictureID string    `json:"pictureId,omitempty"`
	IsGroup   bool      `json:"isGroup"`
}

type UserAboutUpdatedWebhookData struct {
	JID      string    `json:"jid"`
	Status   string    `json:"status"`
	DateTime time.Time `json:"dateTime"`
}

type IdentityUpdatedWebhookData struct {
	JID      string    `json:"jid"`
	DateTime time.Time `json:"dateTime"`
	Implicit bool      `json:"implicit"`
}

type MediaRetryWebhookData struct {
	MessageID     string    `json:"messageId"`
	ChatJID       string    `json:"chatJid"`
	SenderJID     *string   `json:"senderJid,omitempty"`
	KeyFromMe     bool      `json:"keyFromMe"`
	HasCiphertext bool      `json:"hasCiphertext"`
	ErrorCode     *int      `json:"errorCode,omitempty"`
	DateTime      time.Time `json:"dateTime"`
}

type SettingsUpdatedWebhookData struct {
	Type         string    `json:"type"`
	JID          *string   `json:"jid,omitempty"`
	Name         *string   `json:"name,omitempty"`
	Muted        *bool     `json:"muted,omitempty"`
	FromFullSync bool      `json:"fromFullSync"`
	DateTime     time.Time `json:"dateTime"`
}

type GroupParticipantAdmin string

const (
	GroupParticipantAdminAdmin      GroupParticipantAdmin = "admin"
	GroupParticipantAdminSuperAdmin GroupParticipantAdmin = "superadmin"
)

type GroupParticipantWebhookData struct {
	ID           *string `json:"id,omitempty"`
	LID          *string `json:"lid,omitempty"`
	IsAdmin      bool    `json:"isAdmin"`
	IsSuperAdmin bool    `json:"isSuperAdmin"`
	Admin        *string `json:"admin"`
}

type GroupPartialWebhookData struct {
	ID                   string  `json:"id"`
	Notify               *string `json:"notify,omitempty"`
	AddressingMode       *string `json:"addressingMode,omitempty"`
	Owner                *string `json:"owner,omitempty"`
	OwnerPN              *string `json:"ownerPn,omitempty"`
	OwnerUsername        *string `json:"ownerUsername,omitempty"`
	OwnerCountryCode     *string `json:"ownerCountryCode,omitempty"`
	Subject              *string `json:"subject,omitempty"`
	SubjectOwner         *string `json:"subjectOwner,omitempty"`
	SubjectOwnerPN       *string `json:"subjectOwnerPn,omitempty"`
	SubjectOwnerUsername *string `json:"subjectOwnerUsername,omitempty"`
	SubjectTime          *int64  `json:"subjectTime,omitempty"`
	Creation             *int64  `json:"creation,omitempty"`
	Description          *string `json:"desc,omitempty"`
	DescriptionOwner     *string `json:"descOwner,omitempty"`
	DescriptionOwnerPN   *string `json:"descOwnerPn,omitempty"`
	DescriptionOwnerUser *string `json:"descOwnerUsername,omitempty"`
	DescriptionID        *string `json:"descId,omitempty"`
	DescriptionTime      *int64  `json:"descTime,omitempty"`
	LinkedParent         *string `json:"linkedParent,omitempty"`
	Restrict             *bool   `json:"restrict,omitempty"`
	Announce             *bool   `json:"announce,omitempty"`
	MemberAddMode        *bool   `json:"memberAddMode,omitempty"`
	JoinApprovalMode     *bool   `json:"joinApprovalMode,omitempty"`
	IsCommunity          *bool   `json:"isCommunity,omitempty"`
	IsCommunityAnnounce  *bool   `json:"isCommunityAnnounce,omitempty"`
	Size                 *int    `json:"size,omitempty"`
	EphemeralDuration    *int64  `json:"ephemeralDuration,omitempty"`
	InviteCode           *string `json:"inviteCode,omitempty"`
	Author               *string `json:"author,omitempty"`
	AuthorPN             *string `json:"authorPn,omitempty"`
	AuthorUsername       *string `json:"authorUsername,omitempty"`
}

type GroupUpdateWebhookData struct {
	Partial GroupPartialWebhookData `json:"partial"`
}

type GroupUpsertWebhookData struct {
	ID                   string                        `json:"id"`
	Notify               *string                       `json:"notify,omitempty"`
	AddressingMode       *string                       `json:"addressingMode,omitempty"`
	Owner                *string                       `json:"owner,omitempty"`
	OwnerPN              *string                       `json:"ownerPn,omitempty"`
	OwnerUsername        *string                       `json:"ownerUsername,omitempty"`
	OwnerCountryCode     *string                       `json:"ownerCountryCode,omitempty"`
	Subject              string                        `json:"subject"`
	SubjectOwner         *string                       `json:"subjectOwner,omitempty"`
	SubjectOwnerPN       *string                       `json:"subjectOwnerPn,omitempty"`
	SubjectOwnerUsername *string                       `json:"subjectOwnerUsername,omitempty"`
	SubjectTime          *int64                        `json:"subjectTime,omitempty"`
	Creation             *int64                        `json:"creation,omitempty"`
	Description          *string                       `json:"desc,omitempty"`
	DescriptionOwner     *string                       `json:"descOwner,omitempty"`
	DescriptionOwnerPN   *string                       `json:"descOwnerPn,omitempty"`
	DescriptionOwnerUser *string                       `json:"descOwnerUsername,omitempty"`
	DescriptionID        *string                       `json:"descId,omitempty"`
	DescriptionTime      *int64                        `json:"descTime,omitempty"`
	LinkedParent         *string                       `json:"linkedParent,omitempty"`
	Restrict             *bool                         `json:"restrict,omitempty"`
	Announce             *bool                         `json:"announce,omitempty"`
	MemberAddMode        *bool                         `json:"memberAddMode,omitempty"`
	JoinApprovalMode     *bool                         `json:"joinApprovalMode,omitempty"`
	IsCommunity          *bool                         `json:"isCommunity,omitempty"`
	IsCommunityAnnounce  *bool                         `json:"isCommunityAnnounce,omitempty"`
	Size                 *int                          `json:"size,omitempty"`
	Participants         []GroupParticipantWebhookData `json:"participants"`
	EphemeralDuration    *int64                        `json:"ephemeralDuration,omitempty"`
	InviteCode           *string                       `json:"inviteCode,omitempty"`
	Author               *string                       `json:"author,omitempty"`
	AuthorPN             *string                       `json:"authorPn,omitempty"`
	AuthorUsername       *string                       `json:"authorUsername,omitempty"`
}

type GroupParticipantAction string

const (
	GroupParticipantActionAdd     GroupParticipantAction = "add"
	GroupParticipantActionRemove  GroupParticipantAction = "remove"
	GroupParticipantActionPromote GroupParticipantAction = "promote"
	GroupParticipantActionDemote  GroupParticipantAction = "demote"
)

type GroupParticipantsUpdatedWebhookData struct {
	ID           string                        `json:"id"`
	Author       string                        `json:"author"`
	AuthorPN     *string                       `json:"authorPn,omitempty"`
	Participants []GroupParticipantWebhookData `json:"participants"`
	Action       GroupParticipantAction        `json:"action"`
}

func NewMessageUpsertWebhookData(message types.Message) MessageUpsertWebhookData {
	device := string(message.Device)
	var devicePtr *string
	if device != "" {
		devicePtr = &device
	}
	isGroup := false
	if message.IsGroup != nil {
		isGroup = *message.IsGroup
	}
	return MessageUpsertWebhookData{
		ID:                int64(message.ID),
		KeyRemoteJID:      message.KeyRemoteJid,
		KeyLID:            message.KeyLid,
		KeyFromMe:         message.KeyFromMe,
		KeyParticipant:    message.KeyParticipant,
		KeyParticipantLID: message.KeyParticipantLid,
		PushName:          message.PushName,
		MessageType:       message.MessageType,
		Content:           jsonObjectMap(message.Content),
		MessageTimestamp:  int64(message.MessageTimestamp),
		Device:            devicePtr,
		IsGroup:           isGroup,
		Metadata:          nullableJSONObjectMap(message.Metadata),
	}
}

func NewMessageUpdateWebhookData(messageID int32, status string, dateTime time.Time) MessageUpdateWebhookData {
	return MessageUpdateWebhookData{
		MessageID: int64(messageID),
		Status:    status,
		DateTime:  dateTime.UTC(),
	}
}

func NewContactUpsertWebhookData(contact types.Contact, lid *string, action string) ContactUpsertWebhookData {
	if action == "" {
		action = "upserted"
	}
	return ContactUpsertWebhookData{
		ID:            int64(contact.ID),
		RemoteJID:     contact.RemoteJid,
		LID:           lid,
		PushName:      contact.PushName,
		ProfilePicURL: contact.ProfilePicUrl,
		Action:        action,
	}
}

func jsonObjectMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || !json.Valid(raw) || string(raw) == "null" {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded == nil {
		return map[string]any{}
	}
	return decoded
}

func nullableJSONObjectMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || !json.Valid(raw) || string(raw) == "null" {
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	return decoded
}

const (
	StatusInternalClientOutdated       = "ClientOutdated"
	StatusInternalTemporaryBan         = "TemporaryBan"
	StatusInternalOfflineSyncPreview   = "OfflineSyncPreview"
	StatusInternalOfflineSyncCompleted = "OfflineSyncCompleted"
	StatusInternalPrivacySettings      = "PrivacySettings"
	StatusInternalAppState             = "AppState"
	StatusInternalAppStateSyncComplete = "AppStateSyncComplete"
	StatusInternalAppStateSyncError    = "AppStateSyncError"
	StatusInternalAccountTimelock      = "NotifyAccountReachoutTimelock"
)

func NormalizeInstanceStatusWebhookData(internalEvent string, status string, message string, data any) (InstanceStatusWebhookData, bool) {
	output := InstanceStatusWebhookData{
		Status:  status,
		Message: message,
		Data:    data,
	}
	switch internalEvent {
	case StatusInternalClientOutdated:
		output.Type = "client.outdated"
	case StatusInternalTemporaryBan:
		output.Type = "temporary.ban"
	case StatusInternalOfflineSyncPreview:
		output.Type = "offline.sync.preview"
	case StatusInternalOfflineSyncCompleted:
		output.Type = "offline.sync.completed"
	case StatusInternalPrivacySettings:
		output.Type = "privacy.settings"
	case StatusInternalAppState:
		output.Type = "app.state"
	case StatusInternalAppStateSyncComplete:
		output.Type = "app.state.sync.completed"
	case StatusInternalAppStateSyncError:
		output.Type = "app.state.sync.error"
	case StatusInternalAccountTimelock:
		output.Type = "account.reachout.timelock"
	default:
		return InstanceStatusWebhookData{}, false
	}
	return output, true
}
