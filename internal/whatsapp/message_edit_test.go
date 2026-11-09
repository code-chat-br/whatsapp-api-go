package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/proto/waCommon"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	dbtypes "whatsapp-go-api/internal/database/types"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

const (
	testOriginalKeyID = "ACC2ABF14304C9EB5C661462EABFBFDC"
	testEditKeyID     = "AC49B050649BF1384C316DC52F48771C"
)

type fakeSecretDecrypter struct {
	calls   int
	message *wae2e.Message
	err     error
}

func (f *fakeSecretDecrypter) DecryptSecretEncryptedMessage(_ context.Context, evt *events.Message) (*wae2e.Message, error) {
	f.calls++
	if evt.Message.GetSecretEncryptedMessage() == nil {
		return nil, errors.New("not a secret encrypted message")
	}
	return f.message, f.err
}

// secretEditEvent mimics the edit received from the phone: a SecretEncryptedMessage
// of type MESSAGE_EDIT in a LID addressed chat with edit="1".
func secretEditEvent() *events.Message {
	lid := watypes.NewJID("123456789012345", watypes.HiddenUserServer)
	pn := watypes.NewJID("5511988888888", watypes.DefaultUserServer)
	raw := &wae2e.Message{
		SecretEncryptedMessage: &wae2e.SecretEncryptedMessage{
			TargetMessageKey: &waCommon.MessageKey{
				RemoteJID: proto.String(lid.String()),
				FromMe:    proto.Bool(false),
				ID:        proto.String(testOriginalKeyID),
			},
			EncPayload:    []byte("ciphertext"),
			EncIV:         []byte("iv-123456789"),
			SecretEncType: wae2e.SecretEncryptedMessage_MESSAGE_EDIT.Enum(),
		},
	}
	event := &events.Message{
		Info: watypes.MessageInfo{
			ID:        testEditKeyID,
			PushName:  "Contato",
			Timestamp: time.Unix(1794240607, 0).UTC(),
			Edit:      watypes.EditAttributeMessageEdit,
			Type:      "text",
			MessageSource: watypes.MessageSource{
				Chat:      lid,
				Sender:    lid,
				SenderAlt: pn,
			},
		},
		RawMessage: raw,
	}
	return event.UnwrapRaw()
}

func decodeMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return out
}

func TestSecretEncryptedEditWithoutDecryptionWasUnknown(t *testing.T) {
	// Without edit handling the content converter has no case for SecretEncryptedMessage.
	got, err := NewMessageEventNormalizer().NormalizeMessageContent(secretEditEvent().Message)
	if err != nil {
		t.Fatalf("NormalizeMessageContent() error = %v", err)
	}
	if got.MessageType != "unknownMessage" {
		t.Fatalf("expected raw secret edit to be unknownMessage, got %s", got.MessageType)
	}
}

func TestNormalizeSecretEncryptedTextEdit(t *testing.T) {
	event := secretEditEvent()
	if event.IsEdit {
		t.Fatalf("whatsmeow does not flag secret edits as IsEdit")
	}
	decrypter := &fakeSecretDecrypter{message: &wae2e.Message{Conversation: proto.String("Oi, tudo bem?")}}
	edit := resolveMessageEdit(context.Background(), event, decrypter)
	if edit == nil || decrypter.calls != 1 {
		t.Fatalf("expected decrypted edit, got %+v calls=%d", edit, decrypter.calls)
	}

	got, err := NewMessageEventNormalizer().NormalizeMessageWithEdit(1, event, edit)
	if err != nil {
		t.Fatalf("NormalizeMessageWithEdit() error = %v", err)
	}
	if got.MessageType != "editedMessage" || got.KeyID != testEditKeyID {
		t.Fatalf("unexpected type/key: %s %s", got.MessageType, got.KeyID)
	}
	if content := decodeMap(t, got.Content); content["text"] != "Oi, tudo bem?" {
		t.Fatalf("unexpected content: %#v", content)
	}
	metadata := decodeMap(t, got.Metadata)
	if metadata["isEdit"] != true || metadata["edit"] != "1" ||
		metadata["editedMessageId"] != testOriginalKeyID ||
		metadata["editSource"] != messageEditSourceSecret ||
		metadata["editedMessageType"] != "extendedTextMessage" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if _, ok := metadata["editDecryptError"]; ok {
		t.Fatalf("unexpected decrypt error: %#v", metadata)
	}
	if got.KeyRemoteJid == nil || *got.KeyRemoteJid != "5511988888888@s.whatsapp.net" {
		t.Fatalf("unexpected remote jid %#v", got.KeyRemoteJid)
	}
}

func TestNormalizeSecretEncryptedCaptionEdit(t *testing.T) {
	decrypter := &fakeSecretDecrypter{message: &wae2e.Message{
		ProtocolMessage: &wae2e.ProtocolMessage{
			Type: wae2e.ProtocolMessage_MESSAGE_EDIT.Enum(),
			Key:  &waCommon.MessageKey{ID: proto.String(testOriginalKeyID)},
			EditedMessage: &wae2e.Message{
				ImageMessage: &wae2e.ImageMessage{Caption: proto.String("nova legenda")},
			},
		},
	}}
	event := secretEditEvent()
	got, err := NewMessageEventNormalizer().NormalizeMessageWithEdit(1, event, resolveMessageEdit(context.Background(), event, decrypter))
	if err != nil {
		t.Fatalf("NormalizeMessageWithEdit() error = %v", err)
	}
	if content := decodeMap(t, got.Content); content["caption"] != "nova legenda" {
		t.Fatalf("unexpected content: %#v", content)
	}
	if metadata := decodeMap(t, got.Metadata); metadata["editedMessageType"] != "imageMessage" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestNormalizeSecretEncryptedEditDecryptFailure(t *testing.T) {
	event := secretEditEvent()
	edit := resolveMessageEdit(context.Background(), event, &fakeSecretDecrypter{err: errors.New("original message secret not found")})
	got, err := NewMessageEventNormalizer().NormalizeMessageWithEdit(1, event, edit)
	if err != nil {
		t.Fatalf("NormalizeMessageWithEdit() error = %v", err)
	}
	if got.MessageType != "editedMessage" {
		t.Fatalf("expected editedMessage, got %s", got.MessageType)
	}
	if content := decodeMap(t, got.Content); content["unhandled"] != true {
		t.Fatalf("expected unhandled content, got %#v", content)
	}
	metadata := decodeMap(t, got.Metadata)
	if metadata["editDecryptError"] != "original message secret not found" || metadata["editedMessageId"] != testOriginalKeyID {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if _, ok := metadata["editedMessageType"]; ok {
		t.Fatalf("editedMessageType must be absent when the edit is unreadable: %#v", metadata)
	}
}

func TestNormalizeMessageWithoutDecrypterReportsSecretEdit(t *testing.T) {
	got, err := NewMessageEventNormalizer().NormalizeMessage(1, secretEditEvent())
	if err != nil {
		t.Fatalf("NormalizeMessage() error = %v", err)
	}
	metadata := decodeMap(t, got.Metadata)
	if got.MessageType != "editedMessage" || metadata["editDecryptError"] != errSecretDecrypterUnavailable.Error() {
		t.Fatalf("unexpected result: %s %#v", got.MessageType, metadata)
	}
}

func TestNormalizeWrappedProtocolEdit(t *testing.T) {
	jid := watypes.NewJID("5511988888888", watypes.DefaultUserServer)
	event := (&events.Message{
		Info: watypes.MessageInfo{
			ID:            testEditKeyID,
			Timestamp:     time.Unix(100, 0),
			Edit:          watypes.EditAttributeMessageEdit,
			MessageSource: watypes.MessageSource{Chat: jid, Sender: jid},
		},
		RawMessage: &wae2e.Message{EditedMessage: &wae2e.FutureProofMessage{Message: &wae2e.Message{
			ProtocolMessage: &wae2e.ProtocolMessage{
				Type: wae2e.ProtocolMessage_MESSAGE_EDIT.Enum(),
				Key:  &waCommon.MessageKey{ID: proto.String(testOriginalKeyID)},
				EditedMessage: &wae2e.Message{
					ExtendedTextMessage: &wae2e.ExtendedTextMessage{Text: proto.String("editado")},
				},
			},
		}}},
	}).UnwrapRaw()

	got, err := NewMessageEventNormalizer().NormalizeMessage(1, event)
	if err != nil {
		t.Fatalf("NormalizeMessage() error = %v", err)
	}
	if got.MessageType != "editedMessage" {
		t.Fatalf("expected editedMessage, got %s", got.MessageType)
	}
	if content := decodeMap(t, got.Content); content["text"] != "editado" {
		t.Fatalf("unexpected content: %#v", content)
	}
	metadata := decodeMap(t, got.Metadata)
	if metadata["editSource"] != messageEditSourceWrapper || metadata["editedMessageId"] != testOriginalKeyID || metadata["isEdit"] != true {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
}

func TestResolveMessageEditIgnoresNonEdits(t *testing.T) {
	jid := watypes.NewJID("5511988888888", watypes.DefaultUserServer)
	tests := []struct {
		name    string
		edit    watypes.EditAttribute
		message *wae2e.Message
	}{
		{name: "text", message: &wae2e.Message{Conversation: proto.String("Oi")}},
		{name: "revoke", edit: watypes.EditAttributeSenderRevoke, message: &wae2e.Message{ProtocolMessage: &wae2e.ProtocolMessage{
			Type: wae2e.ProtocolMessage_REVOKE.Enum(),
			Key:  &waCommon.MessageKey{ID: proto.String(testOriginalKeyID)},
		}}},
		{name: "pin", edit: watypes.EditAttributePinInChat, message: &wae2e.Message{PinInChatMessage: &wae2e.PinInChatMessage{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &events.Message{
				Info:    watypes.MessageInfo{ID: "msg", Edit: tt.edit, MessageSource: watypes.MessageSource{Chat: jid, Sender: jid}},
				Message: tt.message,
			}
			if edit := resolveMessageEdit(context.Background(), event, nil); edit != nil {
				t.Fatalf("expected no edit, got %+v", edit)
			}
		})
	}
}

func TestApplyEditToStoredContent(t *testing.T) {
	editedAt := time.Unix(1794240607, 0).UTC()
	updated, changed, err := applyEditToStoredContent(json.RawMessage(`{"text":"Oi"}`), json.RawMessage(`{"text":"Oi, tudo bem?"}`), testEditKeyID, editedAt)
	if err != nil || !changed {
		t.Fatalf("expected change, got changed=%v err=%v", changed, err)
	}
	content := decodeMap(t, updated)
	if content["text"] != "Oi, tudo bem?" || content["previousText"] != "Oi" || content["edited"] != true ||
		content["editMessageId"] != testEditKeyID || content["editedAt"] != "2026-11-09T16:10:07Z" {
		t.Fatalf("unexpected content: %#v", content)
	}

	updated, changed, err = applyEditToStoredContent(json.RawMessage(`{"caption":"antiga","mimetype":"image/jpeg"}`), json.RawMessage(`{"caption":"nova"}`), testEditKeyID, editedAt)
	if err != nil || !changed {
		t.Fatalf("expected caption change, got changed=%v err=%v", changed, err)
	}
	content = decodeMap(t, updated)
	if content["caption"] != "nova" || content["previousCaption"] != "antiga" || content["mimetype"] != "image/jpeg" {
		t.Fatalf("unexpected caption content: %#v", content)
	}

	if _, changed, _ := applyEditToStoredContent(json.RawMessage(`{"text":"Oi"}`), json.RawMessage(`{"unhandled":true}`), testEditKeyID, editedAt); changed {
		t.Fatalf("unreadable edit must not change stored content")
	}
}

func TestHandleMessageSecretEditUpdatesOriginalAndDispatches(t *testing.T) {
	original := persistedTextMessage(10, testOriginalKeyID)
	original.Content = json.RawMessage(`{"text":"Oi"}`)
	editRow := persistedTextMessage(11, testEditKeyID)
	editRow.MessageType = "editedMessage"
	editRow.Content = json.RawMessage(`{"text":"Oi, tudo bem?"}`)
	messages := &fakePersistenceMessages{findResults: []findMessageResult{{message: original}, {message: editRow}}}
	webhooks := &fakePersistenceWebhooks{}
	decrypter := &fakeSecretDecrypter{message: &wae2e.Message{Conversation: proto.String("Oi, tudo bem?")}}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.decrypterFor = func(*ManagedWhatsAppClient) secretMessageDecrypter { return decrypter }
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, secretEditEvent())

	if messages.created.KeyID != testEditKeyID || messages.created.MessageType != "editedMessage" {
		t.Fatalf("unexpected created message: %+v", messages.created)
	}
	if messages.updateContentCalls != 1 || messages.updatedID != 10 {
		t.Fatalf("expected original update, got calls=%d id=%d", messages.updateContentCalls, messages.updatedID)
	}
	if content := decodeMap(t, messages.updatedContent); content["text"] != "Oi, tudo bem?" || content["previousText"] != "Oi" {
		t.Fatalf("unexpected updated content: %#v", content)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventMessagesUpsert {
		t.Fatalf("expected one messages.upsert, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	if data, ok := webhooks.data.(webhooksvc.MessageUpsertWebhookData); !ok || data.MessageID != testEditKeyID {
		t.Fatalf("unexpected webhook data: %#v", webhooks.data)
	}
}

func TestHandleMessageSecretEditOriginalMissingStillPersistsEdit(t *testing.T) {
	messages := &fakePersistenceMessages{}
	decrypter := &fakeSecretDecrypter{message: &wae2e.Message{Conversation: proto.String("Oi, tudo bem?")}}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.decrypterFor = func(*ManagedWhatsAppClient) secretMessageDecrypter { return decrypter }

	service.HandleMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, secretEditEvent())

	if messages.createOrIgnoreCalls != 1 || messages.created.MessageType != "editedMessage" {
		t.Fatalf("expected edit to be persisted, got %+v", messages.created)
	}
	if messages.updateContentCalls != 0 {
		t.Fatalf("expected no update without original, got %d", messages.updateContentCalls)
	}
}

func TestManagedSecretDecrypterNilClient(t *testing.T) {
	if managedSecretDecrypter(nil) != nil || managedSecretDecrypter(&ManagedWhatsAppClient{}) != nil {
		t.Fatalf("expected nil decrypter without client")
	}
}
