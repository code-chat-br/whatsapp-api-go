package whatsapp

import (
	"testing"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	webhooksvc "whatsapp-go-api/internal/webhook"
)

func TestCallEventNormalizerStatuses(t *testing.T) {
	meta := watypes.BasicCallMeta{
		From:        watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		CallCreator: watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		CallID:      "3EB0C4D0A1",
		Timestamp:   time.Date(2026, 7, 4, 15, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
	}
	tests := []struct {
		name   string
		event  any
		status webhooksvc.WebhookCallStatus
	}{
		{"offer", &events.CallOffer{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusOffer},
		{"accept", &events.CallAccept{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusAccept},
		{"ringing", &events.CallOfferNotice{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusRinging},
		{"preaccept", &events.CallPreAccept{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusPreAccept},
		{"transport", &events.CallTransport{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusTransport},
		{"terminate", &events.CallTerminate{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusTerminate},
		{"reject", &events.CallReject{BasicCallMeta: meta}, webhooksvc.WebhookCallStatusReject},
		{"relaylatency", &events.CallRelayLatency{BasicCallMeta: meta, Data: &waBinary.Node{Attrs: waBinary.Attrs{"latencyMs": int64(37)}}}, webhooksvc.WebhookCallStatusRelayLatency},
		{"unknown", &events.UnknownCallEvent{Node: &waBinary.Node{Attrs: waBinary.Attrs{"from": "5531988888888@s.whatsapp.net", "id": "unknown-1"}}}, webhooksvc.WebhookCallStatusUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCallEventNormalizer().Normalize(tt.event)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}
			if got.Status != tt.status {
				t.Fatalf("status = %s", got.Status)
			}
			if got.Status == webhooksvc.WebhookCallStatusRelayLatency && (got.Latency == nil || *got.Latency != 37) {
				t.Fatalf("latency = %#v", got.Latency)
			}
			if got.Status != webhooksvc.WebhookCallStatusUnknown && (got.ChatID == "" || got.From == "" || got.ID != "3EB0C4D0A1" || !got.Date.Equal(meta.Timestamp.UTC())) {
				t.Fatalf("unexpected call data: %#v", got)
			}
		})
	}
}

func TestCallEventNormalizerGroupVideoOffline(t *testing.T) {
	meta := watypes.BasicCallMeta{
		From:        watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		GroupJID:    watypes.NewJID("120363000000000000", watypes.GroupServer),
		CallCreator: watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		CallID:      "call-1",
	}
	got, err := NewCallEventNormalizer().Normalize(&events.CallOfferNotice{
		BasicCallMeta: meta,
		Media:         "video",
		Type:          "group",
		Data:          &waBinary.Node{Attrs: waBinary.Attrs{"offline": "true"}},
	})
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if got.IsGroup == nil || !*got.IsGroup || got.GroupJID == nil || *got.GroupJID != "120363000000000000@g.us" || got.IsVideo == nil || !*got.IsVideo || !got.Offline {
		t.Fatalf("unexpected group call data: %#v", got)
	}
}

func TestGroupEventNormalizerUpdateAndParticipants(t *testing.T) {
	author := watypes.NewJID("5531999999999", watypes.DefaultUserServer)
	authorPN := author
	event := &events.GroupInfo{
		JID:      watypes.NewJID("120363000000000000", watypes.GroupServer),
		Sender:   &author,
		SenderPN: &authorPN,
		Name:     &watypes.GroupName{Name: "Novo assunto"},
		Announce: &watypes.GroupAnnounce{IsAnnounce: true},
		Join:     []watypes.JID{watypes.NewJID("5531988888888", watypes.DefaultUserServer)},
		Promote:  []watypes.JID{watypes.NewJID("279847268053216", watypes.HiddenUserServer)},
	}
	normalizer := NewGroupEventNormalizer()
	updates, err := normalizer.NormalizeUpdate(event)
	if err != nil {
		t.Fatalf("NormalizeUpdate() error = %v", err)
	}
	if len(updates) != 1 || updates[0].Partial.Subject == nil || *updates[0].Partial.Subject != "Novo assunto" || updates[0].Partial.Announce == nil || !*updates[0].Partial.Announce {
		t.Fatalf("unexpected group update: %#v", updates)
	}
	participants, err := normalizer.NormalizeParticipantUpdates(event)
	if err != nil {
		t.Fatalf("NormalizeParticipantUpdates() error = %v", err)
	}
	if len(participants) != 2 || participants[0].Action != webhooksvc.GroupParticipantActionAdd || participants[1].Action != webhooksvc.GroupParticipantActionPromote {
		t.Fatalf("unexpected participant updates: %#v", participants)
	}
	if len(updates[0].Partial.ID) == 0 {
		t.Fatal("expected group id")
	}
}

func TestGroupEventNormalizerUpsertParticipantsArray(t *testing.T) {
	joined := &events.JoinedGroup{
		Notify: "invite",
		GroupInfo: watypes.GroupInfo{
			JID:              watypes.NewJID("120363000000000000", watypes.GroupServer),
			GroupName:        watypes.GroupName{Name: "Grupo"},
			GroupParent:      watypes.GroupParent{IsParent: true},
			ParticipantCount: 1,
			Participants: []watypes.GroupParticipant{{
				JID:     watypes.NewJID("5531988888888", watypes.DefaultUserServer),
				LID:     watypes.NewJID("279847268053216", watypes.HiddenUserServer),
				IsAdmin: true,
			}},
		},
	}
	got, err := NewGroupEventNormalizer().NormalizeUpsert(joined)
	if err != nil {
		t.Fatalf("NormalizeUpsert() error = %v", err)
	}
	if len(got) != 1 || got[0].Subject != "Grupo" || got[0].Participants == nil || len(got[0].Participants) != 1 {
		t.Fatalf("unexpected group upsert: %#v", got)
	}
	if got[0].IsCommunity == nil || !*got[0].IsCommunity || got[0].Participants[0].Admin == nil || *got[0].Participants[0].Admin != "admin" {
		t.Fatalf("unexpected group upsert flags: %#v", got[0])
	}
}

func TestNewsletterEventNormalizerSubtypes(t *testing.T) {
	tests := []struct {
		event any
		typ   string
	}{
		{&events.NewsletterJoin{}, "join"},
		{&events.NewsletterLeave{}, "leave"},
		{&events.NewsletterLiveUpdate{}, "live.update"},
		{&events.NewsletterMessageMeta{}, "message.meta"},
		{&events.NewsletterMuteChange{}, "mute.change"},
	}
	for _, tt := range tests {
		got, err := NewNewsletterEventNormalizer().Normalize(tt.event)
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if got["type"] != tt.typ {
			t.Fatalf("type = %#v", got["type"])
		}
	}
}

func TestLabelEventNormalizerAssociationAndEdit(t *testing.T) {
	labeled := true
	association, err := NewLabelEventNormalizer().NormalizeAssociation(&events.LabelAssociationChat{
		JID:     watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		LabelID: "12",
		Action:  &waSyncAction.LabelAssociationAction{Labeled: &labeled},
	})
	if err != nil {
		t.Fatalf("NormalizeAssociation() error = %v", err)
	}
	if association["type"] != "chat" || association["chatJid"] != "5531988888888@s.whatsapp.net" || association["action"] != "add" {
		t.Fatalf("unexpected association payload: %#v", association)
	}
	name := "Cliente"
	color := int32(3)
	deleted := false
	edit, err := NewLabelEventNormalizer().NormalizeEdit(&events.LabelEdit{
		LabelID: "12",
		Action:  &waSyncAction.LabelEditAction{Name: &name, Color: &color, Deleted: &deleted},
	})
	if err != nil {
		t.Fatalf("NormalizeEdit() error = %v", err)
	}
	if edit["id"] != "12" || edit["name"] != "Cliente" || edit["color"] != int64(3) || edit["deleted"] != false {
		t.Fatalf("unexpected edit payload: %#v", edit)
	}
	if _, ok := edit["data"]; ok {
		t.Fatalf("label edit payload must be flattened: %#v", edit)
	}
}
