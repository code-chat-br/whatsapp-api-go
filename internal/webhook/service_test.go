package webhook

import (
	"errors"
	"testing"

	"whatsapp-go-api/internal/database/repository"
)

func TestNormalizeWebhookURL(t *testing.T) {
	for _, value := range []string{
		"https://example.com/webhook",
		"http://internal.local/hook",
	} {
		if _, err := normalizeWebhookURL(value); err != nil {
			t.Fatalf("expected valid URL %q, got %v", value, err)
		}
	}

	for _, value := range []string{
		"",
		"not-a-url",
		"ftp://example.com/webhook",
		"https:///missing-host",
	} {
		if _, err := normalizeWebhookURL(value); !errors.Is(err, repository.ErrInvalidWebhookURL) {
			t.Fatalf("expected invalid URL for %q, got %v", value, err)
		}
	}
}

func TestValidateEventsRejectsUnknownEventAndPreservesFalse(t *testing.T) {
	if err := validateEvents(map[string]bool{
		"messagesUpsert":            false,
		"connectionUpdated":         true,
		"historySync":               true,
		"contactsUpdated":           true,
		"groupsUpdated":             true,
		"callUpsert":                true,
		"labelsAssociation":         true,
		"labelsEdit":                true,
		"groupsParticipantsUpdated": true,
		"groupsUpsert":              true,
		"newsLetter":                true,
		"messagesDeleted":           true,
		"profilePictureUpdated":     true,
		"settingsUpdated":           true,
	}); err != nil {
		t.Fatalf("expected valid events, got %v", err)
	}

	err := validateEvents(map[string]bool{"unknownEvent": false})
	if !errors.Is(err, repository.ErrInvalidWebhookEvent) {
		t.Fatalf("expected ErrInvalidWebhookEvent, got %v", err)
	}
}
