package webhook

import (
	"encoding/json"
	"testing"
	"time"

	"whatsapp-go-api/internal/database/types"
)

func TestNewWebhookInstanceUsesMinimalContractAndExternalAttributesObject(t *testing.T) {
	owner := "5531999999999@s.whatsapp.net"
	instance := NewWebhookInstance(types.Instance{
		ID:                 1,
		Name:               "codechat",
		ConnectionStatus:   types.InstanceConnectionStatusOnline,
		WhatsAppOwnerJid:   &owner,
		ExternalAttributes: json.RawMessage(`{"tenantId":"019f"}`),
	})
	if instance.ID != 1 || instance.Name != "codechat" || instance.ConnectionStatus != "online" || instance.OwnerJID == nil || *instance.OwnerJID != owner {
		t.Fatalf("unexpected instance dto: %#v", instance)
	}
	if got := instance.ExternalAttributes["tenantId"]; got != "019f" {
		t.Fatalf("externalAttributes tenantId = %#v", got)
	}
	body, err := json.Marshal(WebhookPayload{Event: types.WebhookEventMessagesUpsert, Instance: instance, Data: map[string]any{}, Timestamp: time.Now().UTC()})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	dto := decoded["instance"].(map[string]any)
	if _, ok := dto["description"]; ok {
		t.Fatal("instance dto must not expose description")
	}
	if _, ok := dto["externalAttributes"].(map[string]any); !ok {
		t.Fatalf("externalAttributes must be object, got %#v", dto["externalAttributes"])
	}
}

func TestNewWebhookInstanceInvalidExternalAttributesDefaultsToEmptyObject(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage(`null`), json.RawMessage(`invalid`)} {
		instance := NewWebhookInstance(types.Instance{ExternalAttributes: raw})
		if instance.ExternalAttributes == nil || len(instance.ExternalAttributes) != 0 {
			t.Fatalf("expected empty externalAttributes object, got %#v", instance.ExternalAttributes)
		}
	}
}

func TestEventNormalizerLowerCamelCaseAndTextValues(t *testing.T) {
	type source struct {
		MessageID string
		RemoteJID string
		IsFromMe  bool
	}
	got, err := NewEventNormalizer().ToJSONMap(source{MessageID: "m1", RemoteJID: "5511@s.whatsapp.net", IsFromMe: true})
	if err != nil {
		t.Fatalf("ToJSONMap() error = %v", err)
	}
	if got["messageId"] != "m1" || got["remoteJid"] != "5511@s.whatsapp.net" || got["isFromMe"] != true {
		t.Fatalf("unexpected normalized map: %#v", got)
	}
	if _, ok := got["messageid"]; ok {
		t.Fatalf("unexpected lowercase-only key in %#v", got)
	}
}

func TestMergeEventDataReservedFieldsPrevail(t *testing.T) {
	dateTime := time.Date(2026, 7, 4, 13, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	got := MergeEventData("archive", map[string]any{"type": "wrong", "dateTime": "wrong", "chatJid": "5511@s.whatsapp.net"}, dateTime)
	if got["type"] != "archive" {
		t.Fatalf("type = %#v", got["type"])
	}
	if got["dateTime"] != dateTime.UTC() {
		t.Fatalf("dateTime = %#v", got["dateTime"])
	}
}

func TestMessageUpsertWebhookDataDefaultsJSONObjects(t *testing.T) {
	message := types.Message{
		ID:               10,
		MessageType:      "conversation",
		Content:          nil,
		MessageTimestamp: 1783170000,
		Device:           types.DeviceMessageUnknown,
	}
	data := NewMessageUpsertWebhookData(message)
	if data.ID != 10 || data.MessageTimestamp != 1783170000 {
		t.Fatalf("unexpected data: %#v", data)
	}
	if data.Content == nil || len(data.Content) != 0 {
		t.Fatalf("content must default to empty object, got %#v", data.Content)
	}
	if data.Metadata != nil {
		t.Fatalf("metadata should stay nullable, got %#v", data.Metadata)
	}
}

func TestNormalizeConnectionWebhookDataMappings(t *testing.T) {
	tests := []struct {
		internal   string
		wantType   string
		wantStatus string
	}{
		{ConnectionInternalPairSuccess, "pair.success", "connecting"},
		{ConnectionInternalConnected, "connected", "open"},
		{ConnectionInternalDisconnected, "disconnected", "close"},
		{ConnectionInternalLoggedOut, "logged.out", "close"},
		{ConnectionInternalStreamReplaced, "stream.replaced", "replaced"},
		{ConnectionInternalKeepAliveTimeout, "keepalive.timeout", "timeout"},
		{ConnectionInternalKeepAliveRestored, "keepalive.restored", "open"},
		{ConnectionInternalConnectFailure, "connect.failure", "close"},
		{ConnectionInternalManualLoginReconnect, "manual.reconnect", "connecting"},
		{ConnectionInternalPairError, "pair.error", "close"},
		{ConnectionInternalStreamError, "stream.error", "close"},
		{ConnectionInternalCATRefreshError, "cat.refresh.error", "close"},
	}

	for _, tt := range tests {
		t.Run(tt.internal, func(t *testing.T) {
			got, ok := NormalizeConnectionWebhookData(tt.internal, 0, nil, "")
			if !ok {
				t.Fatalf("expected mapping for %s", tt.internal)
			}
			if got.Type != tt.wantType || got.Connection != tt.wantStatus {
				t.Fatalf("expected %s/%s, got %#v", tt.wantType, tt.wantStatus, got)
			}
		})
	}
}

func TestNormalizeInstanceStatusWebhookDataMappings(t *testing.T) {
	tests := []struct {
		internal string
		wantType string
	}{
		{StatusInternalClientOutdated, "client.outdated"},
		{StatusInternalTemporaryBan, "temporary.ban"},
		{StatusInternalOfflineSyncPreview, "offline.sync.preview"},
		{StatusInternalOfflineSyncCompleted, "offline.sync.completed"},
		{StatusInternalPrivacySettings, "privacy.settings"},
		{StatusInternalAppState, "app.state"},
		{StatusInternalAppStateSyncComplete, "app.state.sync.completed"},
		{StatusInternalAppStateSyncError, "app.state.sync.error"},
		{StatusInternalAccountTimelock, "account.reachout.timelock"},
	}

	for _, tt := range tests {
		t.Run(tt.internal, func(t *testing.T) {
			got, ok := NormalizeInstanceStatusWebhookData(tt.internal, "completed", "", nil)
			if !ok {
				t.Fatalf("expected mapping for %s", tt.internal)
			}
			if got.Type != tt.wantType {
				t.Fatalf("expected type %s, got %#v", tt.wantType, got)
			}
		})
	}
}
