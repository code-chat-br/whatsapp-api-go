package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/http/validation"
	"whatsapp-go-api/internal/instance"
)

func TestInstanceHandlerCreateAndRefreshResponses(t *testing.T) {
	service := &fakeInstanceService{}
	handler := NewInstanceHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New()
	app.Post("/instance/create", handler.Create)
	app.Put("/instance/refreshToken/:instanceName", handler.RefreshToken)

	createReq := httptest.NewRequest(http.MethodPost, "/instance/create", strings.NewReader(`{"instanceName":"codechat","externalAttributes":{}}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create app.Test() error = %v", err)
	}
	if createResp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected create status 201, got %d", createResp.StatusCode)
	}

	var createBody map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&createBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if _, ok := createBody["externalAttributes"]; ok {
		t.Fatal("create response must not include externalAttributes")
	}
	if _, ok := createBody["connectionStatus"]; ok {
		t.Fatal("create response must not include connectionStatus")
	}
	if _, ok := createBody["Auth"].(map[string]any)["token"]; !ok {
		t.Fatal("create response must include Auth.token")
	}

	refreshReq := httptest.NewRequest(http.MethodPut, "/instance/refreshToken/codechat", strings.NewReader(`{"oldToken":"old-token"}`))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshReq.Header.Set("Authorization", "Bearer old-token")
	refreshResp, err := app.Test(refreshReq)
	if err != nil {
		t.Fatalf("refresh app.Test() error = %v", err)
	}
	if refreshResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected refresh status 200, got %d", refreshResp.StatusCode)
	}
	var refreshBody map[string]any
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshBody); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if _, ok := refreshBody["Webhook"]; ok {
		t.Fatal("refresh response must not include Webhook")
	}
	if _, ok := refreshBody["instance"]; ok {
		t.Fatal("refresh response must not include full instance")
	}
}

func TestInstanceHandlerFetchOmitsAuthAndWhatsapp(t *testing.T) {
	service := &fakeInstanceService{}
	handler := NewInstanceHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New()
	app.Get("/instance/fetchInstance/:instanceName", handler.Fetch)

	req := httptest.NewRequest(http.MethodGet, "/instance/fetchInstance/codechat", nil)
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
	if _, ok := body["Auth"]; ok {
		t.Fatal("fetch response must not include Auth")
	}
	if _, ok := body["token"]; ok {
		t.Fatal("fetch response must not include token")
	}
	if _, ok := body["Whatsapp"]; ok {
		t.Fatal("fetch response must not include Whatsapp")
	}
	if _, ok := body["Webhook"]; !ok {
		t.Fatal("fetch response must include Webhook key")
	}
}

func TestInstanceHandlerValidationErrors(t *testing.T) {
	service := &fakeInstanceService{}
	handler := NewInstanceHandler(service, validation.New(), zerolog.Nop())
	app := fiber.New(fiber.Config{ErrorHandler: testErrorHandler})
	app.Post("/instance/create", handler.Create)
	app.Put("/instance/refreshToken/:instanceName", handler.RefreshToken)

	createReq := httptest.NewRequest(http.MethodPost, "/instance/create", strings.NewReader(`{`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp, err := app.Test(createReq)
	if err != nil {
		t.Fatalf("create app.Test() error = %v", err)
	}
	if createResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected invalid JSON status 400, got %d", createResp.StatusCode)
	}

	refreshReq := httptest.NewRequest(http.MethodPut, "/instance/refreshToken/codechat", strings.NewReader(`{"oldToken":"   "}`))
	refreshReq.Header.Set("Content-Type", "application/json")
	refreshResp, err := app.Test(refreshReq)
	if err != nil {
		t.Fatalf("refresh app.Test() error = %v", err)
	}
	if refreshResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected refresh validation status 400, got %d", refreshResp.StatusCode)
	}
}

type fakeInstanceService struct{}

func (s *fakeInstanceService) Create(context.Context, instance.CreateInstanceInput) (instance.CreateInstanceResult, error) {
	now := time.Date(2026, 7, 2, 20, 30, 0, 0, time.UTC)
	description := "Instance: Test V1"
	return instance.CreateInstanceResult{
		Instance: types.Instance{
			ID:          1,
			Name:        "codechat",
			Description: &description,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Auth: types.Auth{ID: 1, Token: "new-token", CreatedAt: now, UpdatedAt: now, InstanceID: 1},
	}, nil
}

func (s *fakeInstanceService) List(context.Context, *string) ([]types.InstanceDetails, error) {
	now := time.Date(2026, 7, 2, 20, 30, 0, 0, time.UTC)
	return []types.InstanceDetails{{
		Instance: types.Instance{ID: 1, Name: "codechat", Status: types.InstanceStatusOnline, ConnectionStatus: types.InstanceConnectionStatusOffline, CreatedAt: now, UpdatedAt: now},
		Auth:     &types.Auth{ID: 1, Token: "token", CreatedAt: now, UpdatedAt: now, InstanceID: 1},
	}}, nil
}

func (s *fakeInstanceService) FetchByName(context.Context, string) (types.InstanceDetails, error) {
	now := time.Date(2026, 7, 2, 20, 30, 0, 0, time.UTC)
	return types.InstanceDetails{
		Instance: types.Instance{ID: 1, Name: "codechat", Status: types.InstanceStatusOnline, ConnectionStatus: types.InstanceConnectionStatusOnline, CreatedAt: now, UpdatedAt: now},
	}, nil
}

func (s *fakeInstanceService) RefreshToken(context.Context, string, string, string) (types.Auth, error) {
	now := time.Date(2026, 7, 2, 20, 30, 0, 0, time.UTC)
	return types.Auth{ID: 1, Token: "new-token", CreatedAt: now, UpdatedAt: now, InstanceID: 1}, nil
}

func testErrorHandler(c fiber.Ctx, err error) error {
	var validationErr validation.ValidationError
	if errors.As(err, &validationErr) {
		return c.SendStatus(fiber.StatusBadRequest)
	}
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return c.SendStatus(fiberErr.Code)
	}
	return c.SendStatus(fiber.StatusInternalServerError)
}
