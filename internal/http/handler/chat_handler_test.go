package handler

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/chat"
	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestChatHandlerMediaDataBinaryResponse(t *testing.T) {
	expected := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	service := &fakeChatService{
		mediaResult: chat.MediaDownloadResult{
			Data: expected,
			MediaMetadata: chat.MediaMetadata{
				MediaType: chat.MediaTypeImage,
				MIMEType:  "image/jpeg",
				FileName:  "ABC.jpeg",
				Size:      map[string]any{"fileLength": "4", "height": 1, "width": 1},
			},
		},
	}
	app := fiber.New()
	app.Post("/chat/mediaData/:instanceName", NewChatHandler(service, zerolog.Nop()).MediaData)

	req := httptest.NewRequest(http.MethodPost, "/chat/mediaData/test_001?binary=true", strings.NewReader(`{"messageType":"imageMessage","content":{"directPath":"/media/path"}}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != string(expected) {
		t.Fatalf("body = %v, want %v", body, expected)
	}
	if got := resp.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	disposition, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition"))
	if err != nil {
		t.Fatalf("parse Content-Disposition: %v", err)
	}
	if disposition != "inline" || params["filename"] != "ABC.jpeg" {
		t.Fatalf("Content-Disposition = %q params=%v", disposition, params)
	}
	if service.mediaInstanceName != "test_001" || service.mediaToken != "token" {
		t.Fatalf("service instance/token = %q/%q", service.mediaInstanceName, service.mediaToken)
	}
}

func TestChatHandlerMediaDataMultipartResponse(t *testing.T) {
	expected := []byte("file bytes")
	service := &fakeChatService{
		mediaResult: chat.MediaDownloadResult{
			Data: expected,
			MediaMetadata: chat.MediaMetadata{
				MediaType: chat.MediaTypeImage,
				MIMEType:  "image/jpeg",
				FileName:  "ABC.jpeg",
				Size:      map[string]any{"fileLength": "10", "height": 1, "width": 1},
			},
		},
	}
	app := fiber.New()
	app.Post("/chat/mediaData/:instanceName", NewChatHandler(service, zerolog.Nop()).MediaData)

	req := httptest.NewRequest(http.MethodPost, "/chat/mediaData/test_001", strings.NewReader(`{"messageType":"imageMessage","content":{"directPath":"/media/path"}}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse Content-Type: %v", err)
	}
	if mediaType != "multipart/form-data" || params["boundary"] == "" {
		t.Fatalf("Content-Type = %q params=%v", mediaType, params)
	}
	reader := multipart.NewReader(resp.Body, params["boundary"])
	parts := map[string]struct {
		content string
		header  textHeader
	}{}
	order := []string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart() error = %v", err)
		}
		data, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("ReadAll(part) error = %v", err)
		}
		name := part.FormName()
		order = append(order, name)
		parts[name] = struct {
			content string
			header  textHeader
		}{content: string(data), header: textHeader{contentType: part.Header.Get("Content-Type"), fileName: part.FileName()}}
	}
	if strings.Join(order, ",") != "mediaType,fileName,size,mimetype,file" {
		t.Fatalf("part order = %v", order)
	}
	if parts["mediaType"].content != chat.MediaTypeImage || parts["fileName"].content != "ABC.jpeg" || parts["mimetype"].content != "image/jpeg" {
		t.Fatalf("unexpected multipart fields: %#v", parts)
	}
	var size map[string]any
	if err := json.Unmarshal([]byte(parts["size"].content), &size); err != nil {
		t.Fatalf("size is not valid json: %v", err)
	}
	if parts["file"].content != string(expected) || parts["file"].header.contentType != "image/jpeg" || parts["file"].header.fileName != "ABC.jpeg" {
		t.Fatalf("unexpected file part: %#v", parts["file"])
	}
}

func TestChatHandlerMediaDataInvalidBinary(t *testing.T) {
	app := fiber.New()
	app.Post("/chat/mediaData/:instanceName", NewChatHandler(&fakeChatService{}, zerolog.Nop()).MediaData)
	req := httptest.NewRequest(http.MethodPost, "/chat/mediaData/test_001?binary=abc", strings.NewReader(`{"id":1}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestChatHandlerFindMessages(t *testing.T) {
	remoteJID := "123@s.whatsapp.net"
	service := &fakeChatService{
		findResult: dbtypes.MessageListResult{
			Messages: dbtypes.MessagePage{
				Total:       1,
				Pages:       1,
				CurrentPage: 1,
				Records: []dbtypes.MessageWithUpdates{
					{
						Message: dbtypes.Message{
							ID:               2,
							KeyID:            "key-1",
							KeyRemoteJid:     &remoteJID,
							KeyFromMe:        true,
							MessageType:      "conversation",
							Content:          json.RawMessage(`{"text":"hello"}`),
							MessageTimestamp: 100,
							Device:           dbtypes.DeviceMessageIOS,
							InstanceID:       1,
						},
						MessageUpdate: []dbtypes.MessageUpdateSummary{
							{Status: "DELIVERY_ACK", DateTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
						},
					},
				},
			},
		},
	}
	app := fiber.New()
	app.Post("/chat/findMessages/:instanceName", NewChatHandler(service, zerolog.Nop()).FindMessages)

	req := httptest.NewRequest(http.MethodPost, "/chat/findMessages/test_001", strings.NewReader(`{"where":{"keyFromMe":"true"},"offset":20,"page":1}`))
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if service.findInstanceName != "test_001" || service.findToken != "token" {
		t.Fatalf("service instance/token = %q/%q", service.findInstanceName, service.findToken)
	}
	if service.findInput.Offset != 20 || service.findInput.Page != 1 || service.findInput.Where.KeyFromMe == nil {
		t.Fatalf("unexpected find input: %#v", service.findInput)
	}
}

func TestChatHandlerFindMessagesAcceptsEmptyBody(t *testing.T) {
	service := &fakeChatService{}
	app := fiber.New()
	app.Post("/chat/findMessages/:instanceName", NewChatHandler(service, zerolog.Nop()).FindMessages)

	req := httptest.NewRequest(http.MethodPost, "/chat/findMessages/test_001", nil)
	req.Header.Set("Authorization", "Bearer token")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if service.findInstanceName != "test_001" || service.findToken != "token" {
		t.Fatalf("service instance/token = %q/%q", service.findInstanceName, service.findToken)
	}
	if service.findInput.Offset != 0 || service.findInput.Page != 0 || service.findInput.Where.KeyFromMe != nil {
		t.Fatalf("expected zero-value find input, got %#v", service.findInput)
	}
}

type textHeader struct {
	contentType string
	fileName    string
}

type fakeChatService struct {
	findResult        dbtypes.MessageListResult
	findErr           error
	findInstanceName  string
	findToken         string
	findInput         chat.FindMessagesRequest
	mediaResult       chat.MediaDownloadResult
	mediaErr          error
	mediaInstanceName string
	mediaToken        string
}

func (s *fakeChatService) CheckWhatsAppNumbers(context.Context, string, string, chat.WhatsAppNumbersRequest) ([]chat.WhatsAppNumberResponse, error) {
	return nil, nil
}

func (s *fakeChatService) ReadMessages(context.Context, string, string, chat.ReadMessagesRequest) error {
	return nil
}

func (s *fakeChatService) FindMessages(_ context.Context, instanceName string, token string, input chat.FindMessagesRequest) (dbtypes.MessageListResult, error) {
	s.findInstanceName = instanceName
	s.findToken = token
	s.findInput = input
	return s.findResult, s.findErr
}

func (s *fakeChatService) ArchiveChat(context.Context, string, string, chat.ArchiveChatRequest) error {
	return nil
}

func (s *fakeChatService) DeleteMessageForEveryone(context.Context, string, string, int64) error {
	return nil
}

func (s *fakeChatService) FetchProfilePicture(context.Context, string, string, chat.FetchProfilePictureRequest) (*string, error) {
	return nil, nil
}

func (s *fakeChatService) RejectCall(context.Context, string, string, chat.RejectCallRequest) error {
	return nil
}

func (s *fakeChatService) EditMessage(context.Context, string, string, chat.EditMessageRequest) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (s *fakeChatService) MediaData(_ context.Context, instanceName string, token string, input chat.MediaDataRequest) (chat.MediaDownloadResult, error) {
	s.mediaInstanceName = instanceName
	s.mediaToken = token
	if _, err := input.Validate(); err != nil {
		return chat.MediaDownloadResult{}, err
	}
	return s.mediaResult, s.mediaErr
}
