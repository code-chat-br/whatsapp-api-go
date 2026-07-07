package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/whatsapp"
)

func TestWhatsAppConnectionHandlerRequiresBearer(t *testing.T) {
	handler := NewWhatsAppConnectionHandler(&fakeWhatsAppConnectionService{}, zerolog.Nop())
	app := fiber.New()
	app.Get("/instance/connect/:instanceName", handler.ConnectQRCode)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/instance/connect/codechat", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestWhatsAppConnectionHandlerResponses(t *testing.T) {
	service := &fakeWhatsAppConnectionService{}
	handler := NewWhatsAppConnectionHandler(service, zerolog.Nop())
	app := fiber.New()
	app.Get("/instance/connect/:instanceName", handler.ConnectQRCode)
	app.Get("/instance/connect/:instanceName/code/:phoneNumber", handler.ConnectPhone)
	app.Post("/instance/connect/:instanceName/passkey/challenge", handler.RequestPasskeyChallenge)
	app.Post("/instance/connect/:instanceName/passkey/assertion", handler.SubmitPasskeyAssertion)
	app.Get("/instance/connectionState/:instanceName", handler.ConnectionState)
	app.Delete("/instance/logout/:instanceName", handler.Logout)
	app.Delete("/instance/delete/:instanceName", handler.Delete)

	qrReq := httptest.NewRequest(http.MethodGet, "/instance/connect/codechat", nil)
	qrReq.Header.Set("Authorization", "Bearer token")
	qrResp, err := app.Test(qrReq)
	if err != nil {
		t.Fatalf("QR app.Test() error = %v", err)
	}
	if qrResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected QR status 200, got %d", qrResp.StatusCode)
	}
	var qrBody map[string]any
	if err := json.NewDecoder(qrResp.Body).Decode(&qrBody); err != nil {
		t.Fatalf("decode QR response: %v", err)
	}
	if qrBody["count"].(float64) != 1 || qrBody["code"] != "raw-qr" || qrBody["base64"] != "data:image/png;base64,abc" {
		t.Fatalf("unexpected QR response %#v", qrBody)
	}

	codeReq := httptest.NewRequest(http.MethodGet, "/instance/connect/codechat/code/+5531999999999", nil)
	codeReq.Header.Set("Authorization", "Bearer token")
	codeResp, err := app.Test(codeReq)
	if err != nil {
		t.Fatalf("code app.Test() error = %v", err)
	}
	var codeBody map[string]string
	if err := json.NewDecoder(codeResp.Body).Decode(&codeBody); err != nil {
		t.Fatalf("decode code response: %v", err)
	}
	if codeBody["code"] != "ABCDEF" {
		t.Fatalf("unexpected pairing code response %#v", codeBody)
	}

	challengeReq := httptest.NewRequest(http.MethodPost, "/instance/connect/codechat/passkey/challenge", nil)
	challengeReq.Header.Set("Authorization", "Bearer token")
	challengeResp, err := app.Test(challengeReq)
	if err != nil {
		t.Fatalf("challenge app.Test() error = %v", err)
	}
	if challengeResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected challenge status 200, got %d", challengeResp.StatusCode)
	}
	var challengeBody map[string]any
	if err := json.NewDecoder(challengeResp.Body).Decode(&challengeBody); err != nil {
		t.Fatalf("decode challenge response: %v", err)
	}
	if challengeBody["requestId"] != "7bbaf109-e0cc-44de-a434-8d48dfd5cb7b" || challengeBody["state"] != "AWAITING_ASSERTION" {
		t.Fatalf("unexpected challenge response %#v", challengeBody)
	}

	assertionBody := []byte(`{"requestId":"7bbaf109-e0cc-44de-a434-8d48dfd5cb7b","assertion":{"id":"credential-id","rawId":"cmF3","type":"public-key","response":{"clientDataJSON":"Y2xpZW50","authenticatorData":"YXV0aA","signature":"c2ln","userHandle":null}}}`)
	assertionReq := httptest.NewRequest(http.MethodPost, "/instance/connect/codechat/passkey/assertion", bytes.NewReader(assertionBody))
	assertionReq.Header.Set("Authorization", "Bearer token")
	assertionReq.Header.Set("Content-Type", "application/json")
	assertionResp, err := app.Test(assertionReq)
	if err != nil {
		t.Fatalf("assertion app.Test() error = %v", err)
	}
	if assertionResp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("expected assertion status 202, got %d", assertionResp.StatusCode)
	}
	var assertionResponse map[string]any
	if err := json.NewDecoder(assertionResp.Body).Decode(&assertionResponse); err != nil {
		t.Fatalf("decode assertion response: %v", err)
	}
	if assertionResponse["state"] != "AWAITING_CONFIRMATION" {
		t.Fatalf("unexpected assertion response %#v", assertionResponse)
	}

	stateReq := httptest.NewRequest(http.MethodGet, "/instance/connectionState/codechat", nil)
	stateReq.Header.Set("Authorization", "Bearer token")
	stateResp, err := app.Test(stateReq)
	if err != nil {
		t.Fatalf("state app.Test() error = %v", err)
	}
	var stateBody map[string]any
	if err := json.NewDecoder(stateResp.Body).Decode(&stateBody); err != nil {
		t.Fatalf("decode state response: %v", err)
	}
	if stateBody["state"] != "close" || stateBody["statusReason"].(float64) != 503 {
		t.Fatalf("unexpected state response %#v", stateBody)
	}

	logoutReq := httptest.NewRequest(http.MethodDelete, "/instance/logout/codechat", nil)
	logoutReq.Header.Set("Authorization", "Bearer token")
	logoutResp, err := app.Test(logoutReq)
	if err != nil {
		t.Fatalf("logout app.Test() error = %v", err)
	}
	var logoutBody map[string]any
	if err := json.NewDecoder(logoutResp.Body).Decode(&logoutBody); err != nil {
		t.Fatalf("decode logout response: %v", err)
	}
	if logoutBody["state"] != "logged_out" {
		t.Fatalf("unexpected logout response %#v", logoutBody)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/instance/delete/codechat?force=true", nil)
	deleteReq.Header.Set("Authorization", "Bearer token")
	deleteResp, err := app.Test(deleteReq)
	if err != nil {
		t.Fatalf("delete app.Test() error = %v", err)
	}
	var deleteBody map[string]any
	if err := json.NewDecoder(deleteResp.Body).Decode(&deleteBody); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if deleteBody["deleted"] != true || service.deleteForce != true {
		t.Fatalf("unexpected delete response %#v force=%v", deleteBody, service.deleteForce)
	}
}

func TestWhatsAppConnectionHandlerReturnsIdempotentConnectedResponse(t *testing.T) {
	handler := NewWhatsAppConnectionHandler(&fakeWhatsAppConnectionService{}, zerolog.Nop())
	app := fiber.New()
	app.Get("/instance/connect/:instanceName", handler.ConnectQRCode)

	req := httptest.NewRequest(http.MethodGet, "/instance/connect/connected", nil)
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
	if body["instanceName"] != "connected" || body["connectionStatus"] != "online" || body["alreadyConnected"] != true {
		t.Fatalf("unexpected idempotent connect response %#v", body)
	}
	if _, ok := body["code"]; ok {
		t.Fatalf("idempotent connected response must not include QR code: %#v", body)
	}
}

func TestWhatsAppConnectionHandlerRejectsInvalidForce(t *testing.T) {
	handler := NewWhatsAppConnectionHandler(&fakeWhatsAppConnectionService{}, zerolog.Nop())
	app := fiber.New()
	app.Delete("/instance/delete/:instanceName", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/instance/delete/codechat?force=1", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", resp.StatusCode)
	}
}

type fakeWhatsAppConnectionService struct {
	deleteForce bool
}

func (s *fakeWhatsAppConnectionService) ConnectQRCode(_ context.Context, instanceName string, _ string) (whatsapp.QRCodeConnectionResult, error) {
	if instanceName == "connected" {
		owner := "553171714339@s.whatsapp.net"
		return whatsapp.QRCodeConnectionResult{
			InstanceName:     instanceName,
			ConnectionStatus: "online",
			AlreadyConnected: true,
			OwnerJid:         &owner,
		}, nil
	}
	return whatsapp.QRCodeConnectionResult{Count: 1, Code: "raw-qr", Base64: "data:image/png;base64,abc"}, nil
}

func (s *fakeWhatsAppConnectionService) ConnectPhone(context.Context, string, string, string) (whatsapp.PhonePairingResult, error) {
	return whatsapp.PhonePairingResult{Code: "ABCDEF"}, nil
}

func (s *fakeWhatsAppConnectionService) RequestPasskeyChallenge(context.Context, string, string) (whatsapp.PasskeyChallengeResult, error) {
	return whatsapp.PasskeyChallengeResult{
		RequestID: "7bbaf109-e0cc-44de-a434-8d48dfd5cb7b",
		State:     whatsapp.PasskeyStateAwaitingAssertion,
	}, nil
}

func (s *fakeWhatsAppConnectionService) SubmitPasskeyAssertion(context.Context, string, string, whatsapp.SubmitPasskeyAssertionRequest) (whatsapp.PasskeyAssertionResult, error) {
	return whatsapp.PasskeyAssertionResult{
		State:   whatsapp.PasskeyStateAwaitingConfirmation,
		Message: "A assertion foi enviada ao WhatsApp.",
	}, nil
}

func (s *fakeWhatsAppConnectionService) ConnectionState(context.Context, string, string) (whatsapp.ConnectionStateResult, error) {
	return whatsapp.ConnectionStateResult{State: "close", StatusReason: 503, InstanceName: "codechat", ConnectionStatus: "OFFLINE"}, nil
}

func (s *fakeWhatsAppConnectionService) Logout(context.Context, string, string) (whatsapp.LogoutResult, error) {
	return whatsapp.LogoutResult{InstanceName: "codechat", State: "logged_out", ConnectionStatus: "LOGGED_OUT", Message: "Instance logged out successfully"}, nil
}

func (s *fakeWhatsAppConnectionService) DeleteInstance(_ context.Context, _ string, _ string, force bool) (whatsapp.DeleteResult, error) {
	s.deleteForce = force
	return whatsapp.DeleteResult{InstanceName: "codechat", Deleted: true, Forced: force, Message: "Instance deleted successfully"}, nil
}

func (s *fakeWhatsAppConnectionService) ResolveConnectedClient(context.Context, string) (*whatsapp.ManagedWhatsAppClient, error) {
	return nil, whatsapp.ErrClientNotConnected
}

func (s *fakeWhatsAppConnectionService) Restore(context.Context) error {
	return nil
}

func (s *fakeWhatsAppConnectionService) Shutdown(context.Context) error {
	return nil
}
