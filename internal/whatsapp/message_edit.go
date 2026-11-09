package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	waevents "go.mau.fi/whatsmeow/types/events"
)

const (
	messageEditSourceProtocol = "protocolMessage"
	messageEditSourceWrapper  = "editedMessage"
	messageEditSourceSecret   = "secretEncryptedMessage"
	messageEditSourceInfo     = "editAttribute"
)

var errSecretDecrypterUnavailable = errors.New("secret message decrypter unavailable")

// secretMessageDecrypter is satisfied by *whatsmeow.Client. Edits sent by current
// WhatsApp clients arrive as SecretEncryptedMessage (type MESSAGE_EDIT) and can only
// be read after decrypting them with the secret of the original message.
type secretMessageDecrypter interface {
	DecryptSecretEncryptedMessage(ctx context.Context, evt *waevents.Message) (*wae2e.Message, error)
}

// messageEdit describes an incoming edit of a previously sent message.
type messageEdit struct {
	// OriginalKeyID is the key id of the message that was edited.
	OriginalKeyID string
	// Source tells how the edit was delivered.
	Source string
	// Content is the new message content. It is nil when the edit could not be read.
	Content *wae2e.Message
	// DecryptError is set when a secret encrypted edit could not be decrypted.
	DecryptError string
}

// resolveMessageEdit detects whether the event is an edit and extracts the new
// content and the id of the edited message. It returns nil for regular messages.
func resolveMessageEdit(ctx context.Context, event *waevents.Message, decrypter secretMessageDecrypter) *messageEdit {
	if event == nil || event.Message == nil {
		return nil
	}
	message := event.Message

	if secret := message.GetSecretEncryptedMessage(); secret != nil && secret.GetSecretEncType() == wae2e.SecretEncryptedMessage_MESSAGE_EDIT {
		edit := &messageEdit{
			OriginalKeyID: secret.GetTargetMessageKey().GetID(),
			Source:        messageEditSourceSecret,
		}
		if decrypter == nil {
			edit.DecryptError = errSecretDecrypterUnavailable.Error()
			return edit
		}
		if ctx == nil {
			ctx = context.Background()
		}
		decrypted, err := decrypter.DecryptSecretEncryptedMessage(ctx, event)
		if err != nil {
			edit.DecryptError = err.Error()
			return edit
		}
		if edit.OriginalKeyID == "" {
			edit.OriginalKeyID = decrypted.GetProtocolMessage().GetKey().GetID()
		}
		edit.Content = unwrapEditedMessage(decrypted)
		return edit
	}

	if protocol := message.GetProtocolMessage(); protocol != nil && protocol.GetType() == wae2e.ProtocolMessage_MESSAGE_EDIT {
		source := messageEditSourceProtocol
		if event.IsEdit {
			source = messageEditSourceWrapper
		}
		return &messageEdit{
			OriginalKeyID: protocol.GetKey().GetID(),
			Source:        source,
			Content:       unwrapEditedMessage(message),
		}
	}

	if event.IsEdit {
		return &messageEdit{Source: messageEditSourceWrapper, Content: unwrapEditedMessage(message)}
	}

	switch event.Info.Edit {
	case watypes.EditAttributeMessageEdit, watypes.EditAttributeAdminEdit:
		return &messageEdit{Source: messageEditSourceInfo, Content: unwrapEditedMessage(message)}
	}
	return nil
}

// applyEditToStoredContent writes the edited text or caption into the content of
// the original stored message. It reports false when the edit carries neither.
func applyEditToStoredContent(raw json.RawMessage, edited json.RawMessage, editKeyID string, editedAt time.Time) (json.RawMessage, bool, error) {
	newContent := map[string]any{}
	if err := json.Unmarshal(edited, &newContent); err != nil {
		return nil, false, err
	}
	content := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &content)
	}
	changed := false
	for _, field := range []struct{ current, previous string }{
		{current: "text", previous: "previousText"},
		{current: "caption", previous: "previousCaption"},
	} {
		value, ok := newContent[field.current].(string)
		if !ok {
			continue
		}
		if previous, ok := content[field.current].(string); ok && previous != "" && previous != value {
			content[field.previous] = previous
		}
		content[field.current] = value
		changed = true
	}
	if !changed {
		return nil, false, nil
	}
	content["edited"] = true
	content["editedAt"] = editedAt.UTC().Format(time.RFC3339Nano)
	if editKeyID != "" {
		content["editMessageId"] = editKeyID
	}
	updated, err := json.Marshal(content)
	return updated, err == nil, err
}
