package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/proto/waSyncAction"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestChatUpdatedWebhookDataSubtypes(t *testing.T) {
	jid := watypes.NewJID("120363000000000000", watypes.GroupServer)
	eventTime := time.Date(2026, 7, 4, 13, 15, 0, 0, time.FixedZone("BRT", -3*3600))
	processingTime := time.Date(2026, 7, 4, 13, 16, 0, 0, time.UTC)

	tests := []struct {
		name      string
		event     any
		wantType  string
		wantField string
	}{
		{"blocklist", &events.Blocklist{Action: events.BlocklistActionModify}, "blocklist", "action"},
		{"blocklist.change", &events.BlocklistChange{JID: jid, Action: events.BlocklistChangeActionBlock}, "blocklist.change", "chatJid"},
		{"archive", &events.Archive{JID: jid, Timestamp: eventTime, Action: &waSyncAction.ArchiveChatAction{Archived: proto.Bool(true)}}, "archive", "archived"},
		{"unarchive.setting", &events.UnarchiveChatsSetting{Timestamp: eventTime, Action: &waSyncAction.UnarchiveChatsSetting{UnarchiveChats: proto.Bool(true)}}, "unarchive.setting", "unarchiveChats"},
		{"clear", &events.ClearChat{JID: jid, Timestamp: eventTime, Action: &waSyncAction.ClearChatAction{}, DeleteMedia: true}, "clear", "deleteMedia"},
		{"pin", &events.Pin{JID: jid, Timestamp: eventTime, Action: &waSyncAction.PinAction{Pinned: proto.Bool(true)}}, "pin", "pinned"},
		{"mute", &events.Mute{JID: jid, Timestamp: eventTime, Action: &waSyncAction.MuteAction{Muted: proto.Bool(true), MuteEndTimestamp: proto.Int64(1783256400)}}, "mute", "muted"},
		{"mark.read", &events.MarkChatAsRead{JID: jid, Timestamp: eventTime, Action: &waSyncAction.MarkChatAsReadAction{Read: proto.Bool(true)}}, "mark.read", "read"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chatUpdatedWebhookData(tt.event, processingTime)
			if err != nil {
				t.Fatalf("chatUpdatedWebhookData() error = %v", err)
			}
			if got["type"] != tt.wantType {
				t.Fatalf("type = %#v", got["type"])
			}
			if _, ok := got[tt.wantField]; !ok {
				t.Fatalf("expected field %s in %#v", tt.wantField, got)
			}
			if _, ok := got["data"]; ok {
				t.Fatalf("payload must be flattened, got %#v", got)
			}
			if dt, ok := got["dateTime"].(time.Time); !ok || dt.Location() != time.UTC {
				t.Fatalf("dateTime must be UTC time, got %#v", got["dateTime"])
			}
		})
	}
}

func TestPresenceUpdatedWebhookDataUserPresence(t *testing.T) {
	jid := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	lastSeen := time.Date(2026, 7, 4, 11, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	processingTime := time.Date(2026, 7, 4, 13, 30, 0, 0, time.UTC)

	got, err := presenceUpdatedWebhookData(&events.Presence{
		From:        jid,
		Unavailable: true,
		LastSeen:    lastSeen,
	}, processingTime)
	if err != nil {
		t.Fatalf("presenceUpdatedWebhookData() error = %v", err)
	}
	if got["type"] != "presence" || got["jid"] != "5531988888888@s.whatsapp.net" || got["unavailable"] != true {
		t.Fatalf("unexpected presence payload: %#v", got)
	}
	if got["lastSeen"] != lastSeen.UTC() || got["dateTime"] != processingTime {
		t.Fatalf("unexpected presence timestamps: %#v", got)
	}
}

func TestProfileIdentityAboutAndMediaRetryWebhookData(t *testing.T) {
	jid := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	group := watypes.NewJID("120363000000000000", watypes.GroupServer)
	eventTime := time.Date(2026, 7, 4, 13, 15, 0, 0, time.FixedZone("BRT", -3*3600))

	picture := profilePictureUpdatedWebhookData(&events.Picture{JID: group, Author: jid, Timestamp: eventTime, PictureID: "pic-1"}, time.Now())
	if picture.JID != "120363000000000000@g.us" || picture.Author != "5531988888888@s.whatsapp.net" || !picture.IsGroup || picture.DateTime != eventTime.UTC() {
		t.Fatalf("unexpected picture data: %#v", picture)
	}
	about := userAboutUpdatedWebhookData(&events.UserAbout{JID: jid, Status: "available", Timestamp: eventTime}, time.Now())
	if about.JID != "5531988888888@s.whatsapp.net" || about.Status != "available" || about.DateTime != eventTime.UTC() {
		t.Fatalf("unexpected about data: %#v", about)
	}
	identity := identityUpdatedWebhookData(&events.IdentityChange{JID: jid, Timestamp: eventTime, Implicit: true}, time.Now())
	if identity.JID != "5531988888888@s.whatsapp.net" || !identity.Implicit || identity.DateTime != eventTime.UTC() {
		t.Fatalf("unexpected identity data: %#v", identity)
	}
	retry := mediaRetryWebhookData(&events.MediaRetry{
		MessageID:  "msg-1",
		ChatID:     jid,
		SenderID:   watypes.NewJID("5531977777777", watypes.DefaultUserServer),
		FromMe:     false,
		Ciphertext: []byte("secret"),
		IV:         []byte("secret-iv"),
		Error:      &events.MediaRetryError{Code: 404},
		Timestamp:  eventTime,
	}, time.Now())
	if retry.MessageID != "msg-1" || retry.ChatJID != "5531988888888@s.whatsapp.net" || retry.SenderJID == nil || *retry.SenderJID != "5531977777777@s.whatsapp.net" || !retry.HasCiphertext || retry.ErrorCode == nil || *retry.ErrorCode != 404 {
		t.Fatalf("unexpected retry data: %#v", retry)
	}
}

func TestMessageDeleteStarUndecryptableAndSettingsWebhookData(t *testing.T) {
	chat := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	sender := watypes.NewJID("5531977777777", watypes.DefaultUserServer)
	eventTime := time.Date(2026, 7, 4, 13, 15, 0, 0, time.FixedZone("BRT", -3*3600))
	messageTime := eventTime.Add(-time.Minute)

	deleted := messageDeletedWebhookData(&events.DeleteForMe{
		ChatJID:      chat,
		SenderJID:    sender,
		MessageID:    "msg-1",
		Timestamp:    eventTime,
		Action:       &waSyncAction.DeleteMessageForMeAction{DeleteMedia: proto.Bool(true), MessageTimestamp: proto.Int64(messageTime.Unix())},
		FromFullSync: true,
	}, time.Now())
	if deleted.ChatJID != "5531988888888@s.whatsapp.net" || deleted.SenderJID == nil || *deleted.SenderJID != "5531977777777@s.whatsapp.net" || !deleted.DeleteMedia || deleted.MessageTime == nil || !deleted.MessageTime.Equal(messageTime.UTC()) {
		t.Fatalf("unexpected delete data: %#v", deleted)
	}
	starred := messageStarredWebhookData(&events.Star{
		ChatJID:   chat,
		SenderJID: sender,
		MessageID: "msg-1",
		Timestamp: eventTime,
		Action:    &waSyncAction.StarAction{Starred: proto.Bool(true)},
	}, time.Now())
	if starred.ChatJID != "5531988888888@s.whatsapp.net" || !starred.Starred || starred.DateTime != eventTime.UTC() {
		t.Fatalf("unexpected star data: %#v", starred)
	}
	undecryptable := messageUndecryptableWebhookData(&events.UndecryptableMessage{
		Info: watypes.MessageInfo{
			ID:        "msg-2",
			Timestamp: eventTime,
			MessageSource: watypes.MessageSource{
				Chat:     chat,
				Sender:   sender,
				IsFromMe: false,
			},
		},
		IsUnavailable:   true,
		UnavailableType: events.UnavailableTypeViewOnce,
		DecryptFailMode: events.DecryptFailHide,
	}, time.Now())
	if undecryptable.KeyID != "msg-2" || undecryptable.UnavailableType != "view_once" || undecryptable.DecryptFailMode != "hide" {
		t.Fatalf("unexpected undecryptable data: %#v", undecryptable)
	}
	name := "Novo nome"
	setting, err := settingsUpdatedWebhookData(&events.PushNameSetting{
		Timestamp: eventTime,
		Action:    &waSyncAction.PushNameSetting{Name: &name},
	}, time.Now())
	if err != nil {
		t.Fatalf("settingsUpdatedWebhookData() error = %v", err)
	}
	if setting.Type != "push.name" || setting.Name == nil || *setting.Name != "Novo nome" || setting.DateTime != eventTime.UTC() {
		t.Fatalf("unexpected settings data: %#v", setting)
	}
}

func TestChatUpdatedWebhookDataFlattensActionAndRenamesChatJID(t *testing.T) {
	jid := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	eventTime := time.Date(2026, 7, 4, 13, 15, 0, 0, time.FixedZone("BRT", -3*3600))

	got, err := chatUpdatedWebhookData(&events.Archive{
		JID:       jid,
		Timestamp: eventTime,
		Action:    &waSyncAction.ArchiveChatAction{Archived: proto.Bool(true)},
	}, time.Now())
	if err != nil {
		t.Fatalf("chatUpdatedWebhookData() error = %v", err)
	}
	if got["chatJid"] != "5531988888888@s.whatsapp.net" || got["archived"] != true {
		t.Fatalf("unexpected flattened payload: %#v", got)
	}
	if _, ok := got["jid"]; ok {
		t.Fatalf("raw jid key must be renamed: %#v", got)
	}
	if _, ok := got["action"]; ok {
		t.Fatalf("action map must be flattened: %#v", got)
	}
	if got["dateTime"] != eventTime.UTC() {
		t.Fatalf("dateTime = %#v", got["dateTime"])
	}
}

func TestChatDeletedWebhookData(t *testing.T) {
	jid := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	eventTime := time.Date(2026, 7, 4, 13, 25, 0, 0, time.FixedZone("BRT", -3*3600))

	got, err := chatDeletedWebhookData(&events.DeleteChat{JID: jid, Timestamp: eventTime, Action: &waSyncAction.DeleteChatAction{}, DeleteMedia: true}, time.Now())
	if err != nil {
		t.Fatalf("chatDeletedWebhookData() error = %v", err)
	}
	if _, ok := got["type"]; ok {
		t.Fatalf("chats.delete payload should not add subtype: %#v", got)
	}
	if got["chatJid"] != "5531988888888@s.whatsapp.net" || got["deleteMedia"] != true || got["dateTime"] != eventTime.UTC() {
		t.Fatalf("unexpected delete payload: %#v", got)
	}
}

func TestPresenceUpdatedWebhookData(t *testing.T) {
	chat := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	sender := watypes.NewJID("5531977777777", watypes.DefaultUserServer)
	processingTime := time.Date(2026, 7, 4, 13, 30, 0, 0, time.UTC)

	for _, state := range []watypes.ChatPresence{watypes.ChatPresenceComposing, watypes.ChatPresencePaused} {
		got, err := presenceUpdatedWebhookData(&events.ChatPresence{
			MessageSource: watypes.MessageSource{Chat: chat, Sender: sender},
			State:         state,
			Media:         watypes.ChatPresenceMediaAudio,
		}, processingTime)
		if err != nil {
			t.Fatalf("presenceUpdatedWebhookData() error = %v", err)
		}
		if got["chatJid"] != "5531988888888@s.whatsapp.net" || got["senderJid"] != "5531977777777@s.whatsapp.net" {
			t.Fatalf("unexpected presence JIDs: %#v", got)
		}
		if got["state"] != string(state) || got["media"] != "audio" || got["dateTime"] != processingTime {
			t.Fatalf("unexpected presence payload: %#v", got)
		}
		if _, ok := got["data"]; ok {
			t.Fatalf("presence payload must be flattened: %#v", got)
		}
	}
}
