package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
)

type receivedWebhook struct {
	Headers http.Header
	Payload WebhookPayload
}

type webhookRecorder struct {
	mu       sync.Mutex
	received []receivedWebhook
}

func (r *webhookRecorder) handler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var payload WebhookPayload
		_ = json.NewDecoder(req.Body).Decode(&payload)
		r.mu.Lock()
		r.received = append(r.received, receivedWebhook{Headers: req.Header.Clone(), Payload: payload})
		r.mu.Unlock()
		w.WriteHeader(status)
	}
}

func (r *webhookRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.received)
}

func (r *webhookRecorder) first(t *testing.T) receivedWebhook {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.received) == 0 {
		t.Fatal("expected at least one webhook")
	}
	return r.received[0]
}

func TestManagerDispatchSpecificWebhook(t *testing.T) {
	recorder := &webhookRecorder{}
	server := httptest.NewServer(recorder.handler(http.StatusAccepted))
	defer server.Close()

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     server.URL,
		Enabled: true,
		Events:  types.WebhookEvents{ConnectionUpdated: true},
	})
	manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	ctx := ContextWithRequestID(context.Background(), "req-123")
	if err := manager.Dispatch(ctx, testInstance(), types.WebhookEventConnectionUpdated, ConnectionWebhookData{Type: "connected", Connection: "open"}); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	shutdownManager(t, manager)

	item := recorder.first(t)
	if item.Headers.Get("x-request-id") != "req-123" {
		t.Fatalf("request id header = %q", item.Headers.Get("x-request-id"))
	}
	if item.Headers.Get("x-webhook-event") != string(types.WebhookEventConnectionUpdated) {
		t.Fatalf("event header = %q", item.Headers.Get("x-webhook-event"))
	}
	if item.Payload.Event != types.WebhookEventConnectionUpdated || item.Payload.Instance.Name != "codechat" {
		t.Fatalf("unexpected payload %#v", item.Payload)
	}
}

func TestManagerDispatchHistorySyncWebhook(t *testing.T) {
	recorder := &webhookRecorder{}
	server := httptest.NewServer(recorder.handler(http.StatusAccepted))
	defer server.Close()

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     server.URL,
		Enabled: true,
		Events:  types.WebhookEvents{HistorySync: true},
	})
	manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if err := manager.Dispatch(context.Background(), testInstance(), types.WebhookEventHistorySync, map[string]string{"syncType": "RECENT"}); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	shutdownManager(t, manager)

	item := recorder.first(t)
	if item.Headers.Get("x-webhook-event") != string(types.WebhookEventHistorySync) {
		t.Fatalf("event header = %q", item.Headers.Get("x-webhook-event"))
	}
	if item.Payload.Event != types.WebhookEventHistorySync {
		t.Fatalf("unexpected payload event %#v", item.Payload)
	}
}

func TestManagerHeadersForOfficialEvents(t *testing.T) {
	for _, event := range types.SupportedWebhookEvents() {
		t.Run(string(event), func(t *testing.T) {
			recorder := &webhookRecorder{}
			server := httptest.NewServer(recorder.handler(http.StatusOK))
			defer server.Close()

			cache := NewMemoryWebhookCache()
			cache.Set(1, "codechat", CachedWebhook{
				ID:      10,
				URL:     server.URL,
				Enabled: true,
				Events:  webhookEventsFor(event),
			})
			manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 10}, testLogger())
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}

			ctx := ContextWithRequestID(context.Background(), "req-headers")
			if err := manager.Dispatch(ctx, testInstance(), event, map[string]any{}); err != nil {
				t.Fatalf("Dispatch() error = %v", err)
			}
			shutdownManager(t, manager)

			item := recorder.first(t)
			if item.Headers.Get("Content-Type") != "application/json" ||
				item.Headers.Get("User-Agent") != webhookUserAgent ||
				item.Headers.Get("x-request-id") != "req-headers" ||
				item.Headers.Get("x-owner-jid") != "5531999999999@s.whatsapp.net" ||
				item.Headers.Get("x-instance-name") != "codechat" ||
				item.Headers.Get("x-instance-id") != "1" ||
				item.Headers.Get("x-webhook-event") != string(event) {
				t.Fatalf("unexpected headers for %s: %#v", event, item.Headers)
			}
		})
	}
}

func TestManagerDispatchGlobalWebhookWithoutInstanceConfig(t *testing.T) {
	recorder := &webhookRecorder{}
	server := httptest.NewServer(recorder.handler(http.StatusOK))
	defer server.Close()

	manager, err := NewManager(NewMemoryWebhookCache(), ManagerConfig{GlobalEnabled: true, GlobalURL: server.URL, Workers: 1, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, map[string]string{"type": "connected"}); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	shutdownManager(t, manager)

	if recorder.count() != 1 {
		t.Fatalf("expected global webhook once, got %d", recorder.count())
	}
	if recorder.first(t).Headers.Get("x-request-id") == "" {
		t.Fatal("expected generated request id")
	}
}

func TestManagerDispatchBothTargetsAndIsolatesFailures(t *testing.T) {
	instanceRecorder := &webhookRecorder{}
	instanceServer := httptest.NewServer(instanceRecorder.handler(http.StatusInternalServerError))
	defer instanceServer.Close()
	globalRecorder := &webhookRecorder{}
	globalServer := httptest.NewServer(globalRecorder.handler(http.StatusOK))
	defer globalServer.Close()

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     instanceServer.URL,
		Enabled: true,
		Events:  types.WebhookEvents{ConnectionUpdated: true},
	})
	manager, err := NewManager(cache, ManagerConfig{GlobalEnabled: true, GlobalURL: globalServer.URL, Workers: 2, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, nil); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	shutdownManager(t, manager)

	if instanceRecorder.count() != 1 {
		t.Fatalf("expected instance webhook once, got %d", instanceRecorder.count())
	}
	if globalRecorder.count() != 1 {
		t.Fatalf("expected global webhook once despite instance failure, got %d", globalRecorder.count())
	}
}

func TestManagerSkipsDisabledInstanceEvent(t *testing.T) {
	recorder := &webhookRecorder{}
	server := httptest.NewServer(recorder.handler(http.StatusOK))
	defer server.Close()

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     server.URL,
		Enabled: true,
		Events:  types.WebhookEvents{StatusInstance: true},
	})
	manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, nil); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	shutdownManager(t, manager)

	if recorder.count() != 0 {
		t.Fatalf("expected disabled event to be skipped, got %d", recorder.count())
	}
}

func TestManagerQueueFull(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		<-release
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     "https://example.com/webhook",
		Enabled: true,
		Events:  types.WebhookEvents{ConnectionUpdated: true},
	})
	manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 1, HTTPClient: &http.Client{Transport: transport}}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	_ = manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, nil)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start blocked delivery")
	}
	_ = manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, nil)
	err = manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, nil)
	close(release)
	shutdownManager(t, manager)

	if !errors.Is(err, ErrWebhookQueueFull) {
		t.Fatalf("expected ErrWebhookQueueFull, got %v", err)
	}
}

func TestNewManagerRejectsInvalidGlobalURL(t *testing.T) {
	if _, err := NewManager(NewMemoryWebhookCache(), ManagerConfig{GlobalEnabled: true}, testLogger()); !errors.Is(err, ErrInvalidWebhookURL) {
		t.Fatalf("expected ErrInvalidWebhookURL for missing enabled global URL, got %v", err)
	}
	if _, err := NewManager(NewMemoryWebhookCache(), ManagerConfig{GlobalURL: "ftp://example.com/hook"}, testLogger()); !errors.Is(err, ErrInvalidWebhookURL) {
		t.Fatalf("expected ErrInvalidWebhookURL for invalid global URL, got %v", err)
	}
}

func TestManagerRejectsUnsupportedEvent(t *testing.T) {
	manager, err := NewManager(NewMemoryWebhookCache(), ManagerConfig{Workers: 1, QueueSize: 1}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer shutdownManager(t, manager)

	for _, event := range []types.WebhookEvent{"instance.status", "hystory.sync", "chats.deleted"} {
		if err := manager.Dispatch(context.Background(), testInstance(), event, nil); !errors.Is(err, ErrUnsupportedEvent) {
			t.Fatalf("expected ErrUnsupportedEvent for %s, got %v", event, err)
		}
	}
}

func TestManagerRejectsUnserializablePayloadBeforeQueue(t *testing.T) {
	recorder := &webhookRecorder{}
	server := httptest.NewServer(recorder.handler(http.StatusOK))
	defer server.Close()

	cache := NewMemoryWebhookCache()
	cache.Set(1, "codechat", CachedWebhook{
		ID:      10,
		URL:     server.URL,
		Enabled: true,
		Events:  types.WebhookEvents{ConnectionUpdated: true},
	})
	manager, err := NewManager(cache, ManagerConfig{Workers: 1, QueueSize: 10}, testLogger())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer shutdownManager(t, manager)

	if err := manager.Dispatch(context.Background(), testInstance(), types.WebhookEventConnectionUpdated, map[string]any{"bad": make(chan struct{})}); err == nil {
		t.Fatal("expected serialization error")
	}
	if recorder.count() != 0 {
		t.Fatalf("expected no HTTP request for unserializable payload, got %d", recorder.count())
	}
}

func testInstance() WebhookInstance {
	owner := "5531999999999@s.whatsapp.net"
	return WebhookInstance{
		ID:                 1,
		Name:               "codechat",
		ConnectionStatus:   "online",
		OwnerJID:           &owner,
		ExternalAttributes: map[string]any{},
	}
}

func webhookEventsFor(event types.WebhookEvent) types.WebhookEvents {
	switch event {
	case types.WebhookEventQRCodeUpdated:
		return types.WebhookEvents{QRCodeUpdated: true}
	case types.WebhookEventHistorySync:
		return types.WebhookEvents{HistorySync: true}
	case types.WebhookEventMessagesUpsert:
		return types.WebhookEvents{MessagesUpsert: true}
	case types.WebhookEventMessagesUpdated:
		return types.WebhookEvents{MessagesUpdated: true}
	case types.WebhookEventMessagesDeleted:
		return types.WebhookEvents{MessagesDeleted: true}
	case types.WebhookEventMessagesStarred:
		return types.WebhookEvents{MessagesStarred: true}
	case types.WebhookEventMessagesUndecryptable:
		return types.WebhookEvents{MessagesUndecryptable: true}
	case types.WebhookEventSendMessage:
		return types.WebhookEvents{SendMessage: true}
	case types.WebhookEventContactsUpsert:
		return types.WebhookEvents{ContactsUpsert: true}
	case types.WebhookEventChatsUpdated:
		return types.WebhookEvents{ChatsUpdated: true}
	case types.WebhookEventChatsDeleted:
		return types.WebhookEvents{ChatsDeleted: true}
	case types.WebhookEventPresenceUpdated:
		return types.WebhookEvents{PresenceUpdated: true}
	case types.WebhookEventGroupsUpsert:
		return types.WebhookEvents{GroupsUpsert: true}
	case types.WebhookEventGroupsParticipantsUpdated:
		return types.WebhookEvents{GroupsParticipantsUpdated: true}
	case types.WebhookEventGroupsUpdated:
		return types.WebhookEvents{GroupsUpdated: true}
	case types.WebhookEventConnectionUpdated:
		return types.WebhookEvents{ConnectionUpdated: true}
	case types.WebhookEventStatusInstance:
		return types.WebhookEvents{StatusInstance: true}
	case types.WebhookEventNewsletter:
		return types.WebhookEvents{Newsletter: true}
	case types.WebhookEventContactsUpdated:
		return types.WebhookEvents{ContactsUpdated: true}
	case types.WebhookEventCallUpsert:
		return types.WebhookEvents{CallUpsert: true}
	case types.WebhookEventLabelsAssociation:
		return types.WebhookEvents{LabelsAssociation: true}
	case types.WebhookEventLabelsEdit:
		return types.WebhookEvents{LabelsEdit: true}
	case types.WebhookEventProfilePictureUpdated:
		return types.WebhookEvents{ProfilePictureUpdated: true}
	case types.WebhookEventUserAboutUpdated:
		return types.WebhookEvents{UserAboutUpdated: true}
	case types.WebhookEventIdentityUpdated:
		return types.WebhookEvents{IdentityUpdated: true}
	case types.WebhookEventMediaRetry:
		return types.WebhookEvents{MediaRetry: true}
	case types.WebhookEventSettingsUpdated:
		return types.WebhookEvents{SettingsUpdated: true}
	default:
		return types.WebhookEvents{}
	}
}

func shutdownManager(t *testing.T, manager *Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := manager.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
