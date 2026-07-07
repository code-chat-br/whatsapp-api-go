package whatsapp

import (
	"encoding/json"
	"testing"
	"time"

	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestNormalizeMessageConversation(t *testing.T) {
	normalizer := NewMessageEventNormalizer()
	event := &events.Message{
		Info: watypes.MessageInfo{
			ID:        "msg-1",
			PushName:  "Alice",
			Timestamp: time.Unix(100, 0).UTC(),
			MessageSource: watypes.MessageSource{
				Chat:     watypes.NewJID("5511999999999", watypes.DefaultUserServer),
				Sender:   watypes.NewJID("5511999999999", watypes.DefaultUserServer),
				IsFromMe: false,
			},
		},
		Message: &wae2e.Message{Conversation: proto.String("Ok")},
	}

	got, err := normalizer.NormalizeMessage(42, event)
	if err != nil {
		t.Fatalf("NormalizeMessage() error = %v", err)
	}
	if got.MessageType != "extendedTextMessage" {
		t.Fatalf("expected extendedTextMessage, got %q", got.MessageType)
	}
	var content map[string]string
	if err := json.Unmarshal(got.Content, &content); err != nil {
		t.Fatalf("unmarshal content: %v", err)
	}
	if content["text"] != "Ok" {
		t.Fatalf("expected text Ok, got %q", content["text"])
	}
	if got.MessageTimestamp != 100 {
		t.Fatalf("expected timestamp 100, got %d", got.MessageTimestamp)
	}
	if got.KeyRemoteJid == nil || *got.KeyRemoteJid != "5511999999999@s.whatsapp.net" {
		t.Fatalf("unexpected remote jid: %#v", got.KeyRemoteJid)
	}
}

func TestNormalizeMessageSenderTimestampAndLID(t *testing.T) {
	normalizer := NewMessageEventNormalizer()
	traditional := watypes.NewJID("5511888888888", watypes.DefaultUserServer)
	lid := watypes.NewJID("123456", watypes.HiddenUserServer)
	event := &events.Message{
		Info: watypes.MessageInfo{
			ID:        "msg-2",
			Timestamp: time.Unix(100, 0).UTC(),
			MessageSource: watypes.MessageSource{
				Chat:      lid,
				Sender:    lid,
				SenderAlt: traditional,
				IsFromMe:  false,
			},
		},
		Message: &wae2e.Message{
			ExtendedTextMessage: &wae2e.ExtendedTextMessage{Text: proto.String("with timestamp")},
			MessageContextInfo: &wae2e.MessageContextInfo{
				DeviceListMetadata: &wae2e.DeviceListMetadata{SenderTimestamp: proto.Uint64(200)},
			},
		},
	}

	got, err := normalizer.NormalizeMessage(42, event)
	if err != nil {
		t.Fatalf("NormalizeMessage() error = %v", err)
	}
	if got.MessageTimestamp != 200 {
		t.Fatalf("expected sender timestamp 200, got %d", got.MessageTimestamp)
	}
	if got.KeyRemoteJid == nil || *got.KeyRemoteJid != "5511888888888@s.whatsapp.net" {
		t.Fatalf("expected traditional remote jid, got %#v", got.KeyRemoteJid)
	}
	if got.KeyLid == nil || *got.KeyLid != "123456@lid" {
		t.Fatalf("expected lid jid, got %#v", got.KeyLid)
	}
}

func TestNormalizeMessageMediaTypes(t *testing.T) {
	normalizer := NewMessageEventNormalizer()
	tests := []struct {
		name    string
		message *wae2e.Message
		want    string
	}{
		{name: "image", message: &wae2e.Message{ImageMessage: &wae2e.ImageMessage{Mimetype: proto.String("image/jpeg")}}, want: "imageMessage"},
		{name: "video", message: &wae2e.Message{VideoMessage: &wae2e.VideoMessage{Mimetype: proto.String("video/mp4")}}, want: "videoMessage"},
		{name: "audio", message: &wae2e.Message{AudioMessage: &wae2e.AudioMessage{Mimetype: proto.String("audio/ogg")}}, want: "audioMessage"},
		{name: "document", message: &wae2e.Message{DocumentMessage: &wae2e.DocumentMessage{Mimetype: proto.String("application/pdf")}}, want: "documentMessage"},
		{name: "sticker", message: &wae2e.Message{StickerMessage: &wae2e.StickerMessage{Mimetype: proto.String("image/webp")}}, want: "stickerMessage"},
		{name: "reaction", message: &wae2e.Message{ReactionMessage: &wae2e.ReactionMessage{Text: proto.String("+1")}}, want: "reactionMessage"},
		{name: "unknown", message: &wae2e.Message{}, want: "unknownMessage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizer.NormalizeMessageContent(tt.message)
			if err != nil {
				t.Fatalf("NormalizeMessageContent() error = %v", err)
			}
			if got.MessageType != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got.MessageType)
			}
			if !json.Valid(got.Content) {
				t.Fatalf("content is not valid JSON: %s", got.Content)
			}
		})
	}
}
