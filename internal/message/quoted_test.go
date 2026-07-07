package message

import (
	"encoding/json"
	"testing"

	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestContextInfoFromMap(t *testing.T) {
	info, err := contextInfoFromMap(map[string]any{
		"keyId":        "A5FDD9082F21",
		"keyRemoteJid": "5531999999999@s.whatsapp.net",
		"messageType":  "extendedTextMessage",
		"content": map[string]any{
			"text": "Mensagem original",
		},
	})
	if err != nil {
		t.Fatalf("contextInfoFromMap() error = %v", err)
	}
	if info.GetStanzaID() != "A5FDD9082F21" || info.GetRemoteJID() != "5531999999999@s.whatsapp.net" {
		t.Fatalf("unexpected context info: %#v", info)
	}
	if info.GetParticipant() != "" {
		t.Fatalf("private quote must not set participant")
	}
	if info.GetQuotedMessage().GetExtendedTextMessage().GetText() != "Mensagem original" {
		t.Fatalf("quoted text mismatch")
	}
}

func TestContextInfoFromMapRejectsInvalid(t *testing.T) {
	_, err := contextInfoFromMap(map[string]any{
		"keyRemoteJid": "invalid",
		"messageType":  "extendedTextMessage",
		"content":      map[string]any{"text": "x"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestContextInfoFromPersisted(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"text": "persisted"})
	remote := "120363000000000000@g.us"
	participant := "5531999999999@s.whatsapp.net"
	info, err := contextInfoFromPersisted(dbtypes.Message{
		KeyID:          "MSGID",
		KeyRemoteJid:   &remote,
		KeyParticipant: &participant,
		MessageType:    "extendedTextMessage",
		Content:        raw,
	})
	if err != nil {
		t.Fatalf("contextInfoFromPersisted() error = %v", err)
	}
	if info.GetParticipant() != participant {
		t.Fatalf("participant mismatch: %q", info.GetParticipant())
	}
}
