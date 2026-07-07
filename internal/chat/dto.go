package chat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	DefaultWhatsAppNumbersLimit = 100
	MaxEditTextLength           = 65536
	MaxMediaKeyIDLength         = 256
)

type WhatsAppNumbersRequest struct {
	Numbers []string `json:"numbers" validate:"required,min=1,max=100,dive,required"`
}

type WhatsAppNumberResponse struct {
	JID    string  `json:"jid"`
	LID    *string `json:"lid"`
	Exists bool    `json:"exists"`
}

type ReadMessagesRequest struct {
	IDs        []int64  `json:"ids,omitempty" validate:"omitempty,min=1,dive,gt=0"`
	Sender     *string  `json:"sender,omitempty"`
	Chat       *string  `json:"chat,omitempty"`
	MessageIDs []string `json:"messageIds,omitempty" validate:"omitempty,min=1,dive,message_id"`
}

type ArchiveChatRequest struct {
	LastMessage MessageReferenceDTO `json:"lastMessage" validate:"required"`
	Archive     *bool               `json:"archive" validate:"required"`
}

type MessageReferenceDTO struct {
	Key MessageKeyDTO `json:"key" validate:"required"`
}

type MessageKeyDTO struct {
	RemoteJID string `json:"remoteJid" validate:"required,jid"`
	FromMe    *bool  `json:"fromMe" validate:"required"`
	ID        string `json:"id" validate:"required"`
}

type FetchProfilePictureRequest struct {
	Number    *string `json:"number,omitempty"`
	Chat      *string `json:"chat,omitempty"`
	Recipient *string `json:"recipient,omitempty"`
}

func (r FetchProfilePictureRequest) ResolveRecipient() (string, error) {
	values := make([]string, 0, 3)
	for _, value := range []*string{r.Number, r.Chat, r.Recipient} {
		if value == nil {
			continue
		}
		trimmed := strings.TrimSpace(*value)
		if trimmed == "" {
			return "", fmt.Errorf("%w: recipient cannot be empty", ErrInvalidRecipient)
		}
		values = append(values, trimmed)
	}
	if len(values) != 1 {
		return "", fmt.Errorf("%w: exactly one recipient alias is required", ErrInvalidRecipient)
	}
	return values[0], nil
}

type ProfilePictureResponse struct {
	ProfilePictureURL *string `json:"profilePictureUrl"`
}

type RejectCallRequest struct {
	CallID   string `json:"callId" validate:"required"`
	CallFrom string `json:"callFrom" validate:"required,jid"`
}

type MessageIdentifier struct {
	NumericID *int64
	KeyID     *string
}

func (m *MessageIdentifier) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.New("message identifier is nil")
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return errors.New("id is required")
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return errors.New("id must not be empty")
		}
		m.KeyID = &value
		m.NumericID = nil
		return nil
	}
	var numeric int64
	if err := json.Unmarshal(data, &numeric); err != nil {
		return errors.New("id must be a positive integer or non-empty string")
	}
	if numeric <= 0 {
		return errors.New("id must be greater than 0")
	}
	m.NumericID = &numeric
	m.KeyID = nil
	return nil
}

func (m MessageIdentifier) String() string {
	if m.NumericID != nil {
		return strconv.FormatInt(*m.NumericID, 10)
	}
	if m.KeyID != nil {
		return *m.KeyID
	}
	return ""
}

type EditMessageRequest struct {
	ID   MessageIdentifier `json:"id" validate:"required,message_identifier"`
	Text string            `json:"text" validate:"required,min=1,max=65536,unicode_text"`
}

type ReadMessagesResponse struct {
	Message string `json:"message"`
	Read    string `json:"read"`
}

type MediaDataRequest struct {
	ID          *int64          `json:"id,omitempty"`
	KeyID       *string         `json:"keyId,omitempty"`
	MessageType string          `json:"messageType,omitempty"`
	Content     json.RawMessage `json:"content,omitempty"`
}

type MediaDataMode string

const (
	MediaDataModeID      MediaDataMode = "id"
	MediaDataModeKeyID   MediaDataMode = "key_id"
	MediaDataModePayload MediaDataMode = "payload"
)

type MediaMetadata struct {
	MediaType string         `json:"mediaType"`
	MIMEType  string         `json:"mimetype"`
	FileName  string         `json:"fileName"`
	Size      map[string]any `json:"size"`
}

type MediaDownloadResult struct {
	Data []byte
	MediaMetadata
}

func (r *MediaDataRequest) Validate() (MediaDataMode, error) {
	if r == nil {
		return "", ValidationError{Messages: []string{"body is required"}}
	}

	hasID := r.ID != nil
	hasKeyID := r.KeyID != nil
	hasMessageType := strings.TrimSpace(r.MessageType) != ""
	hasContent := len(bytes.TrimSpace(r.Content)) > 0

	switch {
	case hasID && !hasKeyID && !hasMessageType && !hasContent:
		if *r.ID <= 0 || *r.ID > math.MaxInt32 {
			return "", ValidationError{Messages: []string{"id must be a positive integer"}}
		}
		return MediaDataModeID, nil
	case hasKeyID && !hasID && !hasMessageType && !hasContent:
		trimmed := strings.TrimSpace(*r.KeyID)
		if trimmed == "" {
			return "", ValidationError{Messages: []string{"keyId is required"}}
		}
		if len(trimmed) > MaxMediaKeyIDLength {
			return "", ValidationError{Messages: []string{"keyId must be less than 256"}}
		}
		r.KeyID = &trimmed
		return MediaDataModeKeyID, nil
	case hasMessageType && hasContent:
		if hasID {
			return "", ValidationError{Messages: []string{"exactly one media request mode is required"}}
		}
		if err := validateMediaPayloadContent(r.MessageType, r.Content); err != nil {
			return "", err
		}
		if hasKeyID {
			trimmed := strings.TrimSpace(*r.KeyID)
			if len(trimmed) > MaxMediaKeyIDLength {
				return "", ValidationError{Messages: []string{"keyId must be less than 256"}}
			}
			if trimmed == "" {
				r.KeyID = nil
			} else {
				r.KeyID = &trimmed
			}
		}
		r.MessageType = strings.TrimSpace(r.MessageType)
		return MediaDataModePayload, nil
	default:
		return "", ValidationError{Messages: []string{"exactly one media request mode is required"}}
	}
}

func validateMediaPayloadContent(messageType string, content json.RawMessage) error {
	if !IsSupportedMediaType(strings.TrimSpace(messageType)) {
		return fmt.Errorf("%w: %s", ErrUnsupportedMediaType, messageType)
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ValidationError{Messages: []string{"content is required"}}
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || len(object) == 0 {
		return fmt.Errorf("%w: content must be a non-empty object", ErrInvalidMediaContent)
	}
	return nil
}
