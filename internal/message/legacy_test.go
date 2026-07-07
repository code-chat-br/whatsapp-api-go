package message

import (
	"mime/multipart"
	"net/textproto"
	"strings"
	"testing"

	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestBuildContactProtoSingleAndMentionAll(t *testing.T) {
	contacts, err := validateContacts([]ContactMessage{{
		FullName:    "Code Chat",
		WUID:        "5531999999999@s.whatsapp.net",
		PhoneNumber: "+55 31 99999-9999",
	}})
	if err != nil {
		t.Fatalf("validateContacts() error = %v", err)
	}

	msg, messageType, content, err := buildContactProto(contacts, nil)
	if err != nil {
		t.Fatalf("buildContactProto() error = %v", err)
	}
	if messageType != "contactMessage" {
		t.Fatalf("messageType = %q", messageType)
	}
	if msg.ContactMessage == nil {
		t.Fatal("expected contactMessage")
	}
	if !strings.Contains(msg.ContactMessage.GetVcard(), "BEGIN:VCARD") {
		t.Fatalf("expected generated vcard, got %q", msg.ContactMessage.GetVcard())
	}
	if content["fullName"] != "Code Chat" {
		t.Fatalf("unexpected content fullName = %v", content["fullName"])
	}

	applyMentionedJIDs(msg, []string{"5531888888888@s.whatsapp.net"})
	info := contextInfoFromMessage(msg)
	if info == nil || len(info.MentionedJID) != 1 {
		t.Fatalf("expected mentionAll context info, got %#v", info)
	}
}

func TestBuildContactProtoMultiple(t *testing.T) {
	contacts, err := validateContacts([]ContactMessage{
		{FullName: "One", PhoneNumber: "1"},
		{FullName: "Two", PhoneNumber: "2"},
	})
	if err != nil {
		t.Fatalf("validateContacts() error = %v", err)
	}

	msg, messageType, content, err := buildContactProto(contacts, nil)
	if err != nil {
		t.Fatalf("buildContactProto() error = %v", err)
	}
	if messageType != "contactsArrayMessage" {
		t.Fatalf("messageType = %q", messageType)
	}
	if msg.ContactsArrayMessage == nil || len(msg.ContactsArrayMessage.Contacts) != 2 {
		t.Fatalf("expected contacts array, got %#v", msg.ContactsArrayMessage)
	}
	if _, ok := content["contacts"]; !ok {
		t.Fatalf("expected contacts content, got %#v", content)
	}
}

func TestBuildLocationProtoAndMentionAll(t *testing.T) {
	lat := -19.9212
	lng := -43.9378
	name := "Belo Horizonte"
	location, err := validateLocation(&LocationMessage{
		Name:      &name,
		Latitude:  &lat,
		Longitude: &lng,
	})
	if err != nil {
		t.Fatalf("validateLocation() error = %v", err)
	}

	msg, messageType, content, err := buildLocationProto(location, &wae2e.ContextInfo{StanzaID: proto.String("quoted")})
	if err != nil {
		t.Fatalf("buildLocationProto() error = %v", err)
	}
	if messageType != "locationMessage" {
		t.Fatalf("messageType = %q", messageType)
	}
	if msg.LocationMessage == nil {
		t.Fatal("expected locationMessage")
	}
	if content["latitude"] != lat || content["longitude"] != lng {
		t.Fatalf("unexpected location content = %#v", content)
	}

	applyMentionedJIDs(msg, []string{"5531888888888@s.whatsapp.net"})
	info := contextInfoFromMessage(msg)
	if info == nil || info.GetStanzaID() != "quoted" || len(info.MentionedJID) != 1 {
		t.Fatalf("expected quoted context plus mentionAll, got %#v", info)
	}
}

func TestValidateReaction(t *testing.T) {
	fromMe := true
	reaction, err := validateReaction(&ReactionMessage{
		Key: ReactionKey{
			RemoteJID: "5531999999999@s.whatsapp.net",
			FromMe:    &fromMe,
			ID:        "ABC123",
		},
		Reaction: "ok",
	})
	if err != nil {
		t.Fatalf("validateReaction() error = %v", err)
	}
	if reaction.RemoteJID.String() != "5531999999999@s.whatsapp.net" {
		t.Fatalf("remote jid = %q", reaction.RemoteJID.String())
	}
	if reaction.Key.GetID() != "ABC123" || !reaction.Key.GetFromMe() {
		t.Fatalf("unexpected key = %#v", reaction.Key)
	}
}

func TestParseMultipartMessageOptions(t *testing.T) {
	options, err := ParseMultipartMessageOptions("1200", "composing", "42", `{"keyId":"abc"}`, "true")
	if err != nil {
		t.Fatalf("ParseMultipartMessageOptions() error = %v", err)
	}
	if options == nil || options.Delay == nil || *options.Delay != 1200 {
		t.Fatalf("unexpected delay = %#v", options)
	}
	if options.Presence == nil || *options.Presence != "composing" {
		t.Fatalf("unexpected presence = %#v", options.Presence)
	}
	if options.QuotedMessageID == nil || *options.QuotedMessageID != 42 {
		t.Fatalf("unexpected quotedMessageId = %#v", options.QuotedMessageID)
	}
	if options.MentionAll == nil || !*options.MentionAll {
		t.Fatalf("unexpected mentionAll = %#v", options.MentionAll)
	}
}

func TestReadMediaFile(t *testing.T) {
	header := &multipart.FileHeader{
		Filename: "photo.jpg",
		Size:     int64(len(fakeJPEG)),
		Header:   textproto.MIMEHeader{"Content-Type": []string{"image/jpeg"}},
	}
	data, mimeType, filename, err := readMediaFile(testMultipartFile{Reader: strings.NewReader(string(fakeJPEG))}, header, KindImage)
	if err != nil {
		t.Fatalf("readMediaFile() error = %v", err)
	}
	if string(data) != string(fakeJPEG) {
		t.Fatal("unexpected file data")
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("mimeType = %q", mimeType)
	}
	if filename != "photo.jpg" {
		t.Fatalf("filename = %q", filename)
	}
}

var fakeJPEG = []byte{0xff, 0xd8, 0xff, 0xdb, 0x00, 0x43, 0x00, 0x00}

type testMultipartFile struct {
	*strings.Reader
}

func (testMultipartFile) Close() error {
	return nil
}
