package webhook

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"whatsapp-go-api/internal/database/types"
)

func TestMemoryWebhookCacheLoadGetSetDeleteAndRename(t *testing.T) {
	cache := NewMemoryWebhookCache()
	cache.Load(context.Background(), []CachedWebhook{
		{
			ID:           1,
			InstanceID:   10,
			InstanceName: "CodeChat",
			URL:          "https://example.com/one",
			Enabled:      true,
			Events:       types.WebhookEvents{ConnectionUpdated: true},
			UpdatedAt:    time.Now(),
		},
		{
			ID:           2,
			InstanceID:   11,
			InstanceName: "Disabled",
			URL:          "https://example.com/two",
			Enabled:      false,
		},
	})

	if _, ok := cache.Get(10, "codechat"); !ok {
		t.Fatal("expected loaded webhook to be found case-insensitively")
	}
	if _, ok := cache.Get(11, "disabled"); ok {
		t.Fatal("disabled webhook must not remain in cache")
	}

	cache.Set(10, "renamed", CachedWebhook{
		ID:      1,
		URL:     "https://example.com/renamed",
		Enabled: true,
		Events:  types.WebhookEvents{StatusInstance: true},
	})
	cache.Delete(10, "codechat")

	if _, ok := cache.Get(10, "codechat"); ok {
		t.Fatal("old instance name key must be removed")
	}
	if cached, ok := cache.Get(10, "renamed"); !ok || cached.URL != "https://example.com/renamed" {
		t.Fatalf("expected renamed webhook in cache, got %#v ok=%v", cached, ok)
	}

	cache.Set(10, "renamed", CachedWebhook{Enabled: false})
	if _, ok := cache.Get(10, "renamed"); ok {
		t.Fatal("disabled Set must remove cache entry")
	}
}

func TestMemoryWebhookCacheConcurrentAccess(t *testing.T) {
	cache := NewMemoryWebhookCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			cache.Set(int64(index), "codechat", CachedWebhook{
				ID:      int64(index),
				URL:     "https://example.com/webhook",
				Enabled: true,
				Events:  types.WebhookEvents{ConnectionUpdated: true},
			})
			if _, ok := cache.Get(int64(index), "CODECHAT"); !ok {
				t.Errorf("missing cached webhook %d", index)
			}
			cache.Delete(int64(index), "codechat")
		}(i)
	}
	wg.Wait()
}

func TestCachedWebhookFromModelRecognizesNewEventFlagsAndLegacyMissingFields(t *testing.T) {
	model := types.Webhook{
		ID:         1,
		URL:        "https://example.com/webhook",
		Enabled:    true,
		InstanceID: 10,
		Events: json.RawMessage(`{
			"contactsUpdated": true,
			"groupsUpdated": true,
			"callUpsert": true,
			"labelsAssociation": true,
			"labelsEdit": true,
			"messagesDeleted": true,
			"profilePictureUpdated": true,
			"settingsUpdated": true
		}`),
	}
	cached, err := CachedWebhookFromModel(model, "codechat")
	if err != nil {
		t.Fatalf("CachedWebhookFromModel() error = %v", err)
	}
	if !cached.Events.IsEnabled(types.WebhookEventContactsUpdated) ||
		!cached.Events.IsEnabled(types.WebhookEventGroupsUpdated) ||
		!cached.Events.IsEnabled(types.WebhookEventCallUpsert) ||
		!cached.Events.IsEnabled(types.WebhookEventLabelsAssociation) ||
		!cached.Events.IsEnabled(types.WebhookEventLabelsEdit) ||
		!cached.Events.IsEnabled(types.WebhookEventMessagesDeleted) ||
		!cached.Events.IsEnabled(types.WebhookEventProfilePictureUpdated) ||
		!cached.Events.IsEnabled(types.WebhookEventSettingsUpdated) {
		t.Fatalf("new flags not loaded into cache: %#v", cached.Events)
	}

	legacy, err := CachedWebhookFromModel(types.Webhook{
		URL:    "https://example.com/webhook",
		Events: json.RawMessage(`{"messagesUpsert": true}`),
	}, "legacy")
	if err != nil {
		t.Fatalf("legacy CachedWebhookFromModel() error = %v", err)
	}
	if legacy.Events.IsEnabled(types.WebhookEventContactsUpdated) ||
		legacy.Events.IsEnabled(types.WebhookEventGroupsUpdated) ||
		legacy.Events.IsEnabled(types.WebhookEventCallUpsert) ||
		legacy.Events.IsEnabled(types.WebhookEventLabelsAssociation) ||
		legacy.Events.IsEnabled(types.WebhookEventLabelsEdit) ||
		legacy.Events.IsEnabled(types.WebhookEventMessagesDeleted) ||
		legacy.Events.IsEnabled(types.WebhookEventProfilePictureUpdated) ||
		legacy.Events.IsEnabled(types.WebhookEventSettingsUpdated) {
		t.Fatalf("missing new fields must default false: %#v", legacy.Events)
	}
}
