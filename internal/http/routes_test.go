package http

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/chat"
	"whatsapp-go-api/internal/config"
	dbtypes "whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/group"
	"whatsapp-go-api/internal/http/handler"
	msgpkg "whatsapp-go-api/internal/message"
	webhooksvc "whatsapp-go-api/internal/webhook"
	"whatsapp-go-api/internal/whatsapp"
)

func TestRegisterRoutesHealthAndReadiness(t *testing.T) {
	app := fiber.New()
	RegisterRoutes(app, testConfig(), nil, nil, nil, nil, nil, nil, acceptingValidator{}, readyState{ready: true})

	healthResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	if err != nil {
		t.Fatalf("health app.Test() error = %v", err)
	}
	if healthResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected health status 200, got %d", healthResp.StatusCode)
	}

	readyResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ready", nil))
	if err != nil {
		t.Fatalf("ready app.Test() error = %v", err)
	}
	if readyResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected ready status 200, got %d", readyResp.StatusCode)
	}
}

func TestRegisterRoutesReadinessUnavailable(t *testing.T) {
	app := fiber.New()
	RegisterRoutes(app, testConfig(), nil, nil, nil, nil, nil, nil, acceptingValidator{}, readyState{})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/ready", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("expected ready status 503, got %d", resp.StatusCode)
	}
}

func TestRegisterRoutesConnectionRoutesRequireBearer(t *testing.T) {
	app := fiber.New()
	whatsAppHandler := handler.NewWhatsAppConnectionHandler(&routeWhatsAppService{}, testLogger())
	RegisterRoutes(app, testConfig(), nil, nil, whatsAppHandler, nil, nil, nil, acceptingValidator{}, readyState{ready: true})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/instance/connect/codechat", nil))
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}

	postResp, err := app.Test(httptest.NewRequest(http.MethodPost, "/instance/connect/codechat", nil))
	if err != nil {
		t.Fatalf("POST app.Test() error = %v", err)
	}
	if postResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected POST status 401, got %d", postResp.StatusCode)
	}

	for _, path := range []string{
		"/instance/connect/codechat/passkey/challenge",
		"/instance/connect/codechat/passkey/assertion",
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, path, nil))
		if err != nil {
			t.Fatalf("%s app.Test() error = %v", path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s expected status 401, got %d", path, resp.StatusCode)
		}
	}
}

func TestRegisterRoutesInstanceRefreshRequiresBearer(t *testing.T) {
	app := fiber.New()
	RegisterRoutes(app, testConfig(), nil, nil, nil, nil, nil, nil, acceptingValidator{}, readyState{ready: true})

	req := httptest.NewRequest(http.MethodPut, "/instance/refreshToken/codechat", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestRegisterRoutesWebhookRoutesRequireBearer(t *testing.T) {
	app := fiber.New()
	webhookHandler := handler.NewWebhookHandler(&routeWebhookService{}, nil, testLogger())
	RegisterRoutes(app, testConfig(), nil, webhookHandler, nil, nil, nil, nil, acceptingValidator{}, readyState{ready: true})

	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/webhook/set/codechat"},
		{http.MethodGet, "/webhook/find/codechat"},
	} {
		resp, err := app.Test(httptest.NewRequest(route.method, route.path, nil))
		if err != nil {
			t.Fatalf("%s %s app.Test() error = %v", route.method, route.path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s %s expected status 401, got %d", route.method, route.path, resp.StatusCode)
		}
	}
}

func TestRegisterRoutesConnectionRoutePriority(t *testing.T) {
	app := fiber.New()
	service := &routeWhatsAppService{}
	whatsAppHandler := handler.NewWhatsAppConnectionHandler(service, testLogger())
	RegisterRoutes(app, testConfig(), nil, nil, whatsAppHandler, nil, nil, nil, acceptingValidator{}, readyState{ready: true})

	req := httptest.NewRequest(http.MethodGet, "/instance/connect/codechat/code/5531999999999", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if service.phoneCalls != 1 || service.qrCalls != 0 {
		t.Fatalf("expected phone route only, phone=%d qr=%d", service.phoneCalls, service.qrCalls)
	}
}

func TestRegisterRoutesMessageRoutesRequireBearer(t *testing.T) {
	app := fiber.New()
	messageHandler := handler.NewMessageHandler(&routeMessageService{}, testLogger())
	RegisterRoutes(app, testConfig(), nil, nil, nil, messageHandler, nil, nil, acceptingValidator{}, readyState{ready: true})

	for _, route := range []string{
		"/message/sendText/codechat",
		"/message/sendLink/codechat",
		"/message/sendMedia/codechat",
		"/message/sendMediaFile/codechat",
		"/message/sendWhatsAppAudio/codechat",
		"/message/sendWhatsAppAudioFile/codechat",
		"/message/sendContact/codechat",
		"/message/sendLocation/codechat",
		"/message/sendReaction/codechat",
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, route, nil))
		if err != nil {
			t.Fatalf("%s app.Test() error = %v", route, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s expected status 401, got %d", route, resp.StatusCode)
		}
	}
}

func TestRegisterRoutesChatRoutesRequireBearer(t *testing.T) {
	app := fiber.New()
	chatHandler := handler.NewChatHandler(&routeChatService{}, testLogger())
	RegisterRoutes(app, testConfig(), nil, nil, nil, nil, chatHandler, nil, acceptingValidator{}, readyState{ready: true})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/chat/whatsappNumbers/codechat"},
		{http.MethodPatch, "/chat/readMessages/codechat"},
		{http.MethodPut, "/chat/archiveChat/codechat"},
		{http.MethodDelete, "/chat/deleteMessage/codechat?id=1"},
		{http.MethodPost, "/chat/fetchProfilePictureUrl/codechat"},
		{http.MethodPost, "/chat/rejectCall/codechat"},
		{http.MethodPost, "/chat/editMessage/codechat"},
		{http.MethodPost, "/chat/mediaData/codechat"},
	}

	for _, route := range routes {
		resp, err := app.Test(httptest.NewRequest(route.method, route.path, nil))
		if err != nil {
			t.Fatalf("%s %s app.Test() error = %v", route.method, route.path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s %s expected status 401, got %d", route.method, route.path, resp.StatusCode)
		}
	}
}

func TestRegisterRoutesGroupRoutesRequireBearer(t *testing.T) {
	app := fiber.New()
	groupHandler := handler.NewGroupHandler(&routeGroupService{}, testLogger())
	RegisterRoutes(app, testConfig(), nil, nil, nil, nil, nil, groupHandler, acceptingValidator{}, readyState{ready: true})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/group/create/codechat"},
		{http.MethodPut, "/group/updateGroupPicture/codechat"},
		{http.MethodGet, "/group/inviteCode/codechat?groupJid=123@g.us"},
		{http.MethodPut, "/group/revokeInviteCode/codechat?groupJid=123@g.us"},
		{http.MethodPut, "/group/updateParticipant/codechat?groupJid=123@g.us"},
		{http.MethodDelete, "/group/leaveGroup/codechat?groupJid=123@g.us"},
	}

	for _, route := range routes {
		resp, err := app.Test(httptest.NewRequest(route.method, route.path, nil))
		if err != nil {
			t.Fatalf("%s %s app.Test() error = %v", route.method, route.path, err)
		}
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("%s %s expected status 401, got %d", route.method, route.path, resp.StatusCode)
		}
	}
}

func testConfig() config.Config {
	return config.Config{
		Authentication: config.AuthenticationConfig{
			GlobalAuthToken: "global-token",
		},
	}
}

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

type readyState struct {
	ready bool
}

func (s readyState) Ready() bool {
	return s.ready
}

type acceptingValidator struct{}

func (acceptingValidator) Validate(string) (authjwt.InstanceClaims, error) {
	return authjwt.InstanceClaims{InstanceName: "codechat"}, nil
}

type routeWhatsAppService struct {
	qrCalls    int
	phoneCalls int
}

type routeMessageService struct{}

type routeChatService struct{}

type routeGroupService struct{}

type routeWebhookService struct{}

func (routeMessageService) SendText(context.Context, string, string, msgpkg.SendTextRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendLink(context.Context, string, string, msgpkg.SendLinkRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendMedia(context.Context, string, string, msgpkg.SendMediaRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendMediaFile(context.Context, string, string, string, multipart.File, *multipart.FileHeader, string, *string, *msgpkg.MessageOptions) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendWhatsAppAudio(context.Context, string, string, msgpkg.SendWhatsAppAudioRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendWhatsAppAudioFile(context.Context, string, string, string, multipart.File, *multipart.FileHeader, *msgpkg.MessageOptions) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendContact(context.Context, string, string, msgpkg.SendContactRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendLocation(context.Context, string, string, msgpkg.SendLocationRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeMessageService) SendReaction(context.Context, string, string, msgpkg.SendReactionRequest) (msgpkg.SendResult, error) {
	return msgpkg.SendResult{}, nil
}

func (routeChatService) CheckWhatsAppNumbers(context.Context, string, string, chat.WhatsAppNumbersRequest) ([]chat.WhatsAppNumberResponse, error) {
	return nil, nil
}

func (routeChatService) ReadMessages(context.Context, string, string, chat.ReadMessagesRequest) error {
	return nil
}

func (routeChatService) ArchiveChat(context.Context, string, string, chat.ArchiveChatRequest) error {
	return nil
}

func (routeChatService) DeleteMessageForEveryone(context.Context, string, string, int64) error {
	return nil
}

func (routeChatService) FetchProfilePicture(context.Context, string, string, chat.FetchProfilePictureRequest) (*string, error) {
	return nil, nil
}

func (routeChatService) RejectCall(context.Context, string, string, chat.RejectCallRequest) error {
	return nil
}

func (routeChatService) EditMessage(context.Context, string, string, chat.EditMessageRequest) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (routeChatService) MediaData(context.Context, string, string, chat.MediaDataRequest) (chat.MediaDownloadResult, error) {
	return chat.MediaDownloadResult{}, nil
}

func (routeGroupService) Create(context.Context, string, string, group.CreateRequest) (group.InfoResponse, error) {
	return group.InfoResponse{}, nil
}

func (routeGroupService) UpdatePicture(context.Context, string, string, group.UpdatePictureRequest) (group.InfoResponse, error) {
	return group.InfoResponse{}, nil
}

func (routeGroupService) InviteCode(context.Context, string, string, string) (group.InviteCodeResponse, error) {
	return group.InviteCodeResponse{}, nil
}

func (routeGroupService) RevokeInviteCode(context.Context, string, string, string) error {
	return nil
}

func (routeGroupService) UpdateParticipant(context.Context, string, string, string, group.UpdateParticipantRequest) error {
	return nil
}

func (routeGroupService) Leave(context.Context, string, string, string) error {
	return nil
}

func (routeWebhookService) Set(context.Context, string, string, webhooksvc.SetInput) (dbtypes.Webhook, error) {
	return dbtypes.Webhook{}, nil
}

func (routeWebhookService) Find(context.Context, string, string) (dbtypes.Webhook, error) {
	return dbtypes.Webhook{}, nil
}

func (s *routeWhatsAppService) ConnectQRCode(context.Context, string, string) (whatsapp.QRCodeConnectionResult, error) {
	s.qrCalls++
	return whatsapp.QRCodeConnectionResult{Count: 1, Code: "qr", Base64: "png"}, nil
}

func (s *routeWhatsAppService) ConnectPhone(context.Context, string, string, string) (whatsapp.PhonePairingResult, error) {
	s.phoneCalls++
	return whatsapp.PhonePairingResult{Code: "ABCDEF"}, nil
}

func (s *routeWhatsAppService) RequestPasskeyChallenge(context.Context, string, string) (whatsapp.PasskeyChallengeResult, error) {
	return whatsapp.PasskeyChallengeResult{RequestID: "7bbaf109-e0cc-44de-a434-8d48dfd5cb7b", State: whatsapp.PasskeyStateAwaitingAssertion}, nil
}

func (s *routeWhatsAppService) SubmitPasskeyAssertion(context.Context, string, string, whatsapp.SubmitPasskeyAssertionRequest) (whatsapp.PasskeyAssertionResult, error) {
	return whatsapp.PasskeyAssertionResult{State: whatsapp.PasskeyStateAwaitingConfirmation}, nil
}

func (s *routeWhatsAppService) ConnectionState(context.Context, string, string) (whatsapp.ConnectionStateResult, error) {
	return whatsapp.ConnectionStateResult{State: "close", StatusReason: 503}, nil
}

func (s *routeWhatsAppService) Logout(context.Context, string, string) (whatsapp.LogoutResult, error) {
	return whatsapp.LogoutResult{State: "logged_out"}, nil
}

func (s *routeWhatsAppService) DeleteInstance(context.Context, string, string, bool) (whatsapp.DeleteResult, error) {
	return whatsapp.DeleteResult{Deleted: true}, nil
}

func (s *routeWhatsAppService) ResolveConnectedClient(context.Context, string) (*whatsapp.ManagedWhatsAppClient, error) {
	return nil, whatsapp.ErrClientNotConnected
}

func (s *routeWhatsAppService) Restore(context.Context) error {
	return nil
}

func (s *routeWhatsAppService) Shutdown(context.Context) error {
	return nil
}
