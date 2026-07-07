package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/http/validation"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

func TestWebhookHandlerSetPassesExplicitFalseAndEmptyEvents(t *testing.T) {
	service := &fakeWebhookService{}
	handler := NewWebhookHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Put("/webhook/set/:instanceName", handler.Set)

	req := httptest.NewRequest(http.MethodPut, "/webhook/set/codechat", strings.NewReader(`{"enabled":false,"url":"https://example.com/webhook","events":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if service.setInput.Enabled == nil || *service.setInput.Enabled {
		t.Fatalf("expected explicit enabled=false, got %#v", service.setInput.Enabled)
	}
	if !service.setInput.EventsSet || len(service.setInput.Events) != 0 {
		t.Fatalf("expected explicit empty events, got set=%v events=%#v", service.setInput.EventsSet, service.setInput.Events)
	}
}

func TestWebhookHandlerSetLeavesEventsAbsent(t *testing.T) {
	service := &fakeWebhookService{}
	handler := NewWebhookHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Put("/webhook/set/:instanceName", handler.Set)

	req := httptest.NewRequest(http.MethodPut, "/webhook/set/codechat", strings.NewReader(`{"url":"https://example.com/webhook"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if service.setInput.EventsSet {
		t.Fatal("events must stay absent when the field is not sent")
	}
}

func TestWebhookHandlerFindResponseIncludesInstanceIDAndDefaultEvents(t *testing.T) {
	service := &fakeWebhookService{}
	handler := NewWebhookHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Get("/webhook/find/:instanceName", handler.Find)

	req := httptest.NewRequest(http.MethodGet, "/webhook/find/codechat", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["instanceId"].(float64) != 1 {
		t.Fatalf("expected instanceId in response, got %#v", body)
	}
	if _, ok := body["events"].(map[string]any); !ok {
		t.Fatalf("expected events object, got %#v", body["events"])
	}
}

func TestWebhookHandlerRejectsInvalidURL(t *testing.T) {
	service := &fakeWebhookService{}
	handler := NewWebhookHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Put("/webhook/set/:instanceName", handler.Set)

	req := httptest.NewRequest(http.MethodPut, "/webhook/set/codechat", strings.NewReader(`{"url":"not-a-url"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

type fakeWebhookService struct {
	setInput webhooksvc.SetInput
}

func (s *fakeWebhookService) Set(_ context.Context, _ string, _ string, input webhooksvc.SetInput) (types.Webhook, error) {
	s.setInput = input
	now := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	return types.Webhook{ID: 1, URL: input.URL, Enabled: true, CreatedAt: now, UpdatedAt: now, InstanceID: 1}, nil
}

func (s *fakeWebhookService) Find(context.Context, string, string) (types.Webhook, error) {
	now := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)
	return types.Webhook{ID: 1, URL: "https://example.com/webhook", Enabled: true, CreatedAt: now, UpdatedAt: now, InstanceID: 1}, nil
}
