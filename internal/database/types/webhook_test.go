package types

import (
	"encoding/json"
	"testing"
)

func TestWebhookEventsIsEnabled(t *testing.T) {
	events := WebhookEvents{
		QRCodeUpdated:             true,
		HistorySync:               true,
		MessagesUpsert:            true,
		MessagesUpdated:           true,
		MessagesDeleted:           true,
		MessagesStarred:           true,
		MessagesUndecryptable:     true,
		SendMessage:               true,
		ContactsUpsert:            true,
		ContactsUpdated:           true,
		ChatsUpdated:              true,
		ChatsDeleted:              true,
		PresenceUpdated:           true,
		GroupsUpsert:              true,
		GroupsUpdated:             true,
		GroupsParticipantsUpdated: true,
		ConnectionUpdated:         true,
		StatusInstance:            true,
		Newsletter:                true,
		CallUpsert:                true,
		LabelsAssociation:         true,
		LabelsEdit:                true,
		ProfilePictureUpdated:     true,
		UserAboutUpdated:          true,
		IdentityUpdated:           true,
		MediaRetry:                true,
		SettingsUpdated:           true,
	}

	fields := WebhookEventFields()
	if len(fields) != len(SupportedWebhookEvents()) {
		t.Fatalf("field/event count mismatch: fields=%d events=%d", len(fields), len(SupportedWebhookEvents()))
	}
	for field, event := range fields {
		t.Run(field, func(t *testing.T) {
			if !events.IsEnabled(event) {
				t.Fatalf("expected %s to be enabled by field %s", event, field)
			}
		})
	}
	for _, event := range SupportedWebhookEvents() {
		t.Run(string(event), func(t *testing.T) {
			found := false
			for _, mapped := range fields {
				if mapped == event {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("supported event %s has no config field", event)
			}
		})
	}
}

func TestWebhookEventsSpecificMappings(t *testing.T) {
	events := WebhookEvents{
		HistorySync:               true,
		MessagesUpsert:            true,
		MessagesUpdated:           true,
		MessagesDeleted:           true,
		MessagesStarred:           true,
		MessagesUndecryptable:     true,
		ContactsUpsert:            true,
		ContactsUpdated:           true,
		ChatsUpdated:              true,
		ChatsDeleted:              true,
		PresenceUpdated:           true,
		GroupsUpsert:              true,
		GroupsUpdated:             true,
		GroupsParticipantsUpdated: true,
		StatusInstance:            true,
		Newsletter:                true,
		CallUpsert:                true,
		LabelsAssociation:         true,
		LabelsEdit:                true,
		ProfilePictureUpdated:     true,
		UserAboutUpdated:          true,
		IdentityUpdated:           true,
		MediaRetry:                true,
		SettingsUpdated:           true,
	}

	tests := map[WebhookEvent]string{
		WebhookEventHistorySync:               "history.sync must map to historySync",
		WebhookEventMessagesUpsert:            "messages.upsert must map to messagesUpsert",
		WebhookEventMessagesUpdated:           "messages.update must map to messagesUpdated",
		WebhookEventMessagesDeleted:           "messages.delete must map to messagesDeleted",
		WebhookEventMessagesStarred:           "messages.star must map to messagesStarred",
		WebhookEventMessagesUndecryptable:     "messages.undecryptable must map to messagesUndecryptable",
		WebhookEventContactsUpsert:            "contacts.upsert must map to contactsUpsert",
		WebhookEventContactsUpdated:           "contacts.update must map to contactsUpdated",
		WebhookEventChatsUpdated:              "chats.updated must map to chatsUpdated",
		WebhookEventChatsDeleted:              "chats.delete must map to chatsDeleted",
		WebhookEventPresenceUpdated:           "presence.updated must map to presenceUpdated",
		WebhookEventGroupsUpsert:              "groups.upsert must map to groupsUpsert",
		WebhookEventGroupsUpdated:             "groups.update must map to groupsUpdated",
		WebhookEventGroupsParticipantsUpdated: "groups.participants.update must map to groupsParticipantsUpdated",
		WebhookEventStatusInstance:            "status.instance must map to statusInstance",
		WebhookEventNewsletter:                "news.letter must map to newsLetter",
		WebhookEventCallUpsert:                "call.upsert must map to callUpsert",
		WebhookEventLabelsAssociation:         "labels.association must map to labelsAssociation",
		WebhookEventLabelsEdit:                "labels.edit must map to labelsEdit",
		WebhookEventProfilePictureUpdated:     "profile.picture.update must map to profilePictureUpdated",
		WebhookEventUserAboutUpdated:          "user.about.update must map to userAboutUpdated",
		WebhookEventIdentityUpdated:           "identity.update must map to identityUpdated",
		WebhookEventMediaRetry:                "media.retry must map to mediaRetry",
		WebhookEventSettingsUpdated:           "settings.update must map to settingsUpdated",
	}
	for event, message := range tests {
		if !events.IsEnabled(event) {
			t.Fatal(message)
		}
	}
}

func TestValidateWebhookEventFieldsRejectsLegacyUnknownFields(t *testing.T) {
	if err := ValidateWebhookEventFields(map[string]bool{"connectionUpdated": true, "statusInstance": false, "newsLetter": true}); err != nil {
		t.Fatalf("expected official fields to be valid, got %v", err)
	}
	if err := ValidateWebhookEventFields(map[string]bool{"status.instance": true}); err == nil {
		t.Fatal("expected external event names to be rejected as config fields")
	}
	if WebhookEvent("instance.status").IsSupported() {
		t.Fatal("expected legacy instance.status event to be unsupported")
	}
	if WebhookEvent("hystory.sync").IsSupported() {
		t.Fatal("expected misspelled hystory.sync event to be unsupported")
	}
	if WebhookEvent("chats.deleted").IsSupported() {
		t.Fatal("expected legacy chats.deleted event to be unsupported")
	}
	for _, event := range []WebhookEvent{"label.ssociation", "group-participants.update", "groups.updated", "groups.deleted"} {
		if event.IsSupported() {
			t.Fatalf("expected invalid event %s to be unsupported", event)
		}
	}
	if err := ValidateWebhookEventFields(map[string]bool{"contactsSet": true}); err == nil {
		t.Fatal("expected legacy non-official field to be rejected")
	}
	if err := ValidateWebhookEventFields(map[string]bool{"groupUpsert": true, "groupsParticipantsUpdated==": true}); err == nil {
		t.Fatal("expected malformed event fields to be rejected")
	}
}

func TestParseWebhookEvents(t *testing.T) {
	raw := json.RawMessage(`{"connectionUpdated":true,"statusInstance":true,"groupsParticipantsUpdated":true,"historySync":true,"contactsUpdated":true,"groupsUpdated":true,"callUpsert":true,"labelsAssociation":true,"labelsEdit":true,"messagesDeleted":true,"messagesStarred":true,"messagesUndecryptable":true,"profilePictureUpdated":true,"userAboutUpdated":true,"identityUpdated":true,"mediaRetry":true,"settingsUpdated":true}`)
	events, err := ParseWebhookEvents(raw)
	if err != nil {
		t.Fatalf("ParseWebhookEvents() error = %v", err)
	}
	if !events.IsEnabled(WebhookEventConnectionUpdated) ||
		!events.IsEnabled(WebhookEventStatusInstance) ||
		!events.IsEnabled(WebhookEventGroupsParticipantsUpdated) ||
		!events.IsEnabled(WebhookEventHistorySync) ||
		!events.IsEnabled(WebhookEventContactsUpdated) ||
		!events.IsEnabled(WebhookEventGroupsUpdated) ||
		!events.IsEnabled(WebhookEventCallUpsert) ||
		!events.IsEnabled(WebhookEventLabelsAssociation) ||
		!events.IsEnabled(WebhookEventLabelsEdit) ||
		!events.IsEnabled(WebhookEventMessagesDeleted) ||
		!events.IsEnabled(WebhookEventMessagesStarred) ||
		!events.IsEnabled(WebhookEventMessagesUndecryptable) ||
		!events.IsEnabled(WebhookEventProfilePictureUpdated) ||
		!events.IsEnabled(WebhookEventUserAboutUpdated) ||
		!events.IsEnabled(WebhookEventIdentityUpdated) ||
		!events.IsEnabled(WebhookEventMediaRetry) ||
		!events.IsEnabled(WebhookEventSettingsUpdated) {
		t.Fatalf("parsed events not enabled: %#v", events)
	}
}
