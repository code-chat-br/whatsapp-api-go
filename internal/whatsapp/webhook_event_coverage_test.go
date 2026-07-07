package whatsapp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/types/events"

	dbtypes "whatsapp-go-api/internal/database/types"
)

const auditedWhatsmeowVersion = "v0.0.0-20260630180629-b572e5bcb92b"

func TestWhatsmeowVersionForWebhookCoverage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	want := "go.mau.fi/whatsmeow " + auditedWhatsmeowVersion
	if !strings.Contains(string(data), want) {
		t.Fatalf("whatsmeow version changed from audited %s; refresh webhook coverage matrix", auditedWhatsmeowVersion)
	}
}

type eventCoverageStatus string

const (
	coverageCovered               eventCoverageStatus = "covered"
	coverageHandledWithoutWebhook eventCoverageStatus = "handled_without_webhook"
	coverageIntentionallyIgnored  eventCoverageStatus = "intentionally_ignored"
	coverageDuplicate             eventCoverageStatus = "duplicate"
	coverageInternalOnly          eventCoverageStatus = "internal_only"
	coverageUnsupported           eventCoverageStatus = "unsupported"
)

type eventCoverageEntry struct {
	Name          string
	Handler       string
	WebhookEvent  dbtypes.WebhookEvent
	Status        eventCoverageStatus
	Justification string
}

func TestWebhookEventCoverage(t *testing.T) {
	known := knownWhatsmeowEventTypes()
	if len(known) != 74 {
		t.Fatalf("unexpected whatsmeow event type count: %d", len(known))
	}
	seen := map[string]struct{}{}
	for _, name := range known {
		if _, ok := seen[name]; ok {
			t.Fatalf("duplicate known event type %s", name)
		}
		seen[name] = struct{}{}
	}

	coverage := webhookCoverageMatrix()
	if len(coverage) != len(known) {
		t.Fatalf("coverage matrix count mismatch: known=%d matrix=%d", len(known), len(coverage))
	}
	for _, entry := range coverage {
		if _, ok := seen[entry.Name]; !ok {
			t.Fatalf("coverage entry %s is not in known whatsmeow events", entry.Name)
		}
		if entry.Handler == "" {
			t.Fatalf("coverage entry %s has no handler classification", entry.Name)
		}
		switch entry.Status {
		case coverageCovered:
			if entry.WebhookEvent == "" || !entry.WebhookEvent.IsSupported() {
				t.Fatalf("covered event %s has unsupported webhook %s", entry.Name, entry.WebhookEvent)
			}
		case coverageHandledWithoutWebhook, coverageIntentionallyIgnored, coverageDuplicate, coverageInternalOnly, coverageUnsupported:
			if strings.TrimSpace(entry.Justification) == "" {
				t.Fatalf("%s event %s must have justification", entry.Status, entry.Name)
			}
		default:
			t.Fatalf("event %s has invalid status %s", entry.Name, entry.Status)
		}
	}
}

func TestUnhandledWhatsAppEventLogsSafeMetadata(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	service := &Service{logger: logger}
	service.logUnhandledWhatsAppEvent(&ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, &events.PairPasskeyRequest{})

	output := buf.String()
	if !strings.Contains(output, `"eventType":"*events.PairPasskeyRequest"`) ||
		!strings.Contains(output, `"package":"go.mau.fi/whatsmeow/types/events"`) ||
		!strings.Contains(output, `"instanceId":"42"`) ||
		!strings.Contains(output, `"instanceName":"codechat"`) ||
		!strings.Contains(output, `"message":"unhandled whatsapp event"`) {
		t.Fatalf("unexpected log output: %s", output)
	}
}

func knownWhatsmeowEventTypes() []string {
	return []string{
		"AppState",
		"AppStateSyncComplete",
		"AppStateSyncError",
		"Archive",
		"Blocklist",
		"BlocklistChange",
		"BusinessName",
		"CallAccept",
		"CallOffer",
		"CallOfferNotice",
		"CallPreAccept",
		"CallReject",
		"CallRelayLatency",
		"CallTerminate",
		"CallTransport",
		"CATRefreshError",
		"ChatPresence",
		"ClearChat",
		"ClientOutdated",
		"Connected",
		"ConnectFailure",
		"Contact",
		"DeleteChat",
		"DeleteForMe",
		"Disconnected",
		"FBMessage",
		"GroupInfo",
		"HistorySync",
		"IdentityChange",
		"JoinedGroup",
		"KeepAliveRestored",
		"KeepAliveTimeout",
		"LabelAssociationChat",
		"LabelAssociationMessage",
		"LabelEdit",
		"LoggedOut",
		"ManualLoginReconnect",
		"MarkChatAsRead",
		"MediaRetry",
		"MediaRetryError",
		"Message",
		"MexNotificationData",
		"Mute",
		"NewsletterJoin",
		"NewsletterLeave",
		"NewsletterLiveUpdate",
		"NewsletterMessageMeta",
		"NewsletterMuteChange",
		"NotifyAccountReachoutTimelock",
		"OfflineSyncCompleted",
		"OfflineSyncPreview",
		"PairError",
		"PairPasskeyConfirmation",
		"PairPasskeyError",
		"PairPasskeyRequest",
		"PairSuccess",
		"Picture",
		"Pin",
		"Presence",
		"PrivacySettings",
		"PushName",
		"PushNameSetting",
		"QR",
		"QRScannedWithoutMultidevice",
		"Receipt",
		"Star",
		"StreamError",
		"StreamReplaced",
		"TemporaryBan",
		"UnarchiveChatsSetting",
		"UndecryptableMessage",
		"UnknownCallEvent",
		"UserAbout",
		"UserStatusMute",
	}
}

func webhookCoverageMatrix() []eventCoverageEntry {
	return []eventCoverageEntry{
		{"AppState", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"AppStateSyncComplete", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"AppStateSyncError", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"Archive", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"Blocklist", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"BlocklistChange", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"BusinessName", "EventPersistenceService.HandleBusinessName", dbtypes.WebhookEventContactsUpdated, coverageCovered, ""},
		{"CallAccept", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallOffer", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallOfferNotice", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallPreAccept", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallReject", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallRelayLatency", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallTerminate", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CallTransport", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"CATRefreshError", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"ChatPresence", "registerEventHandlers", dbtypes.WebhookEventPresenceUpdated, coverageCovered, ""},
		{"ClearChat", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"ClientOutdated", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"Connected", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"ConnectFailure", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"Contact", "EventPersistenceService.HandleContact", dbtypes.WebhookEventContactsUpsert, coverageCovered, ""},
		{"DeleteChat", "registerEventHandlers", dbtypes.WebhookEventChatsDeleted, coverageCovered, ""},
		{"DeleteForMe", "registerEventHandlers", dbtypes.WebhookEventMessagesDeleted, coverageCovered, ""},
		{"Disconnected", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"FBMessage", "EventPersistenceService.HandleFBMessage", dbtypes.WebhookEventMessagesUpsert, coverageDuplicate, "same normalized contract as Message, emitted once through persistence for FB messages"},
		{"GroupInfo", "registerEventHandlers", dbtypes.WebhookEventGroupsUpdated, coverageCovered, ""},
		{"HistorySync", "registerEventHandlers", dbtypes.WebhookEventHistorySync, coverageCovered, ""},
		{"IdentityChange", "registerEventHandlers", dbtypes.WebhookEventIdentityUpdated, coverageCovered, ""},
		{"JoinedGroup", "registerEventHandlers", dbtypes.WebhookEventGroupsUpsert, coverageCovered, ""},
		{"KeepAliveRestored", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"KeepAliveTimeout", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"LabelAssociationChat", "registerEventHandlers", dbtypes.WebhookEventLabelsAssociation, coverageCovered, ""},
		{"LabelAssociationMessage", "registerEventHandlers", dbtypes.WebhookEventLabelsAssociation, coverageCovered, ""},
		{"LabelEdit", "registerEventHandlers", dbtypes.WebhookEventLabelsEdit, coverageCovered, ""},
		{"LoggedOut", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"ManualLoginReconnect", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"MarkChatAsRead", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"MediaRetry", "registerEventHandlers", dbtypes.WebhookEventMediaRetry, coverageCovered, ""},
		{"MediaRetryError", "MediaRetry payload member", "", coverageInternalOnly, "support struct nested in MediaRetry, not emitted independently by AddEventHandler"},
		{"Message", "EventPersistenceService.HandleMessage", dbtypes.WebhookEventMessagesUpsert, coverageCovered, ""},
		{"MexNotificationData", "newsletter payload member", "", coverageInternalOnly, "support struct embedded in newsletter events, not emitted independently by AddEventHandler"},
		{"Mute", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"NewsletterJoin", "registerEventHandlers", dbtypes.WebhookEventNewsletter, coverageCovered, ""},
		{"NewsletterLeave", "registerEventHandlers", dbtypes.WebhookEventNewsletter, coverageCovered, ""},
		{"NewsletterLiveUpdate", "registerEventHandlers", dbtypes.WebhookEventNewsletter, coverageCovered, ""},
		{"NewsletterMessageMeta", "Message.NewsletterMeta", "", coverageInternalOnly, "metadata struct attached to Message, not emitted independently"},
		{"NewsletterMuteChange", "registerEventHandlers", dbtypes.WebhookEventNewsletter, coverageCovered, ""},
		{"NotifyAccountReachoutTimelock", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"OfflineSyncCompleted", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"OfflineSyncPreview", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"PairError", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"PairPasskeyConfirmation", "fallback log", "", coverageIntentionallyIgnored, "passkey confirmation code is interactive pairing data and is not sent as webhook"},
		{"PairPasskeyError", "fallback log", "", coverageHandledWithoutWebhook, "safe fallback logs the concrete type; passkey pairing is not part of stable webhook contract"},
		{"PairPasskeyRequest", "fallback log", "", coverageIntentionallyIgnored, "contains passkey public-key challenge details and should not be serialized to webhook"},
		{"PairSuccess", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"Picture", "registerEventHandlers", dbtypes.WebhookEventProfilePictureUpdated, coverageCovered, ""},
		{"Pin", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"Presence", "registerEventHandlers", dbtypes.WebhookEventPresenceUpdated, coverageCovered, ""},
		{"PrivacySettings", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"PushName", "EventPersistenceService.HandlePushName", dbtypes.WebhookEventContactsUpdated, coverageCovered, ""},
		{"PushNameSetting", "registerEventHandlers", dbtypes.WebhookEventSettingsUpdated, coverageCovered, ""},
		{"QR", "QR channel", dbtypes.WebhookEventQRCodeUpdated, coverageCovered, ""},
		{"QRScannedWithoutMultidevice", "QR channel/fallback log", "", coverageHandledWithoutWebhook, "QR channel maps this to pairing failure state; AddEventHandler fallback logs future direct emissions safely"},
		{"Receipt", "EventPersistenceService.HandleReceipt", dbtypes.WebhookEventMessagesUpdated, coverageCovered, ""},
		{"Star", "registerEventHandlers", dbtypes.WebhookEventMessagesStarred, coverageCovered, ""},
		{"StreamError", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"StreamReplaced", "registerEventHandlers", dbtypes.WebhookEventConnectionUpdated, coverageCovered, ""},
		{"TemporaryBan", "registerEventHandlers", dbtypes.WebhookEventStatusInstance, coverageCovered, ""},
		{"UnarchiveChatsSetting", "registerEventHandlers", dbtypes.WebhookEventChatsUpdated, coverageCovered, ""},
		{"UndecryptableMessage", "registerEventHandlers", dbtypes.WebhookEventMessagesUndecryptable, coverageCovered, ""},
		{"UnknownCallEvent", "registerEventHandlers", dbtypes.WebhookEventCallUpsert, coverageCovered, ""},
		{"UserAbout", "registerEventHandlers", dbtypes.WebhookEventUserAboutUpdated, coverageCovered, ""},
		{"UserStatusMute", "registerEventHandlers", dbtypes.WebhookEventSettingsUpdated, coverageCovered, ""},
	}
}
