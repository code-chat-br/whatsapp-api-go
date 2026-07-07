package chat

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/go-playground/validator/v10"
	watypes "go.mau.fi/whatsmeow/types"
)

type ValidationError struct {
	Messages []string
}

func (e ValidationError) Error() string {
	return strings.Join(e.Messages, "; ")
}

var requestValidator = newRequestValidator()

func newRequestValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	_ = v.RegisterValidation("jid", validateJID)
	_ = v.RegisterValidation("message_id", validateMessageID)
	_ = v.RegisterValidation("message_identifier", validateMessageIdentifier)
	_ = v.RegisterValidation("unicode_text", validateUnicodeText)
	return v
}

func validateStruct(value any) error {
	if err := requestValidator.Struct(value); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			messages := make([]string, 0, len(validationErrors))
			for _, validationErr := range validationErrors {
				messages = append(messages, readableValidationMessage(validationErr))
			}
			return ValidationError{Messages: messages}
		}
		return err
	}
	return nil
}

func readableValidationMessage(err validator.FieldError) string {
	field := jsonFieldName(err.StructField())
	switch err.Tag() {
	case "required":
		return field + " is required"
	case "min":
		if err.Kind().String() == "slice" {
			return field + " must contain at least one item"
		}
		return field + " must be greater than " + err.Param()
	case "max":
		if err.Kind().String() == "slice" || err.Kind().String() == "array" {
			return field + " must contain at most " + err.Param() + " items"
		}
		return field + " must be less than " + err.Param()
	case "gt":
		return field + " must be greater than " + err.Param()
	case "jid":
		return field + " must be a valid WhatsApp JID"
	case "message_id":
		return field + " must be a valid message id"
	case "message_identifier":
		return field + " must be a positive integer or non-empty string"
	case "unicode_text":
		return field + " must be valid unicode text"
	default:
		return field + " is invalid"
	}
}

func jsonFieldName(field string) string {
	switch field {
	case "Numbers":
		return "numbers"
	case "IDs":
		return "ids"
	case "Sender":
		return "sender"
	case "Chat":
		return "chat"
	case "MessageIDs":
		return "messageIds"
	case "LastMessage":
		return "lastMessage"
	case "Archive":
		return "archive"
	case "RemoteJID":
		return "remoteJid"
	case "FromMe":
		return "fromMe"
	case "ID":
		return "id"
	case "Number":
		return "number"
	case "Recipient":
		return "recipient"
	case "CallID":
		return "callId"
	case "CallFrom":
		return "callFrom"
	case "Text":
		return "text"
	default:
		return strings.ToLower(field)
	}
}

func validateJID(fl validator.FieldLevel) bool {
	value := strings.TrimSpace(fl.Field().String())
	if value == "" {
		return false
	}
	_, err := watypes.ParseJID(value)
	return err == nil
}

func validateMessageID(fl validator.FieldLevel) bool {
	return strings.TrimSpace(fl.Field().String()) != ""
}

func validateMessageIdentifier(fl validator.FieldLevel) bool {
	identifier, ok := fl.Field().Interface().(MessageIdentifier)
	if !ok {
		return false
	}
	if identifier.NumericID != nil {
		return *identifier.NumericID > 0
	}
	return identifier.KeyID != nil && strings.TrimSpace(*identifier.KeyID) != ""
}

func validateUnicodeText(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return strings.TrimSpace(value) != "" && utf8.ValidString(value)
}

func validateWhatsAppNumbers(input WhatsAppNumbersRequest, limit int) error {
	if err := validateStruct(input); err != nil {
		return err
	}
	if input.Numbers == nil {
		return ValidationError{Messages: []string{"numbers is required"}}
	}
	if len(input.Numbers) == 0 {
		return ValidationError{Messages: []string{"numbers must contain at least one item"}}
	}
	if limit <= 0 {
		limit = DefaultWhatsAppNumbersLimit
	}
	if len(input.Numbers) > limit {
		return ValidationError{Messages: []string{fmt.Sprintf("numbers must contain at most %d items", limit)}}
	}
	for i, number := range input.Numbers {
		if strings.TrimSpace(number) == "" {
			return ValidationError{Messages: []string{fmt.Sprintf("numbers[%d] is required", i)}}
		}
	}
	return nil
}

func validateReadMessages(input ReadMessagesRequest) error {
	if err := validateStruct(input); err != nil {
		return err
	}
	hasIDs := len(input.IDs) > 0
	hasDirect := input.Sender != nil || input.Chat != nil || len(input.MessageIDs) > 0
	if hasIDs == hasDirect {
		return ValidationError{Messages: []string{"exactly one read mode is required"}}
	}
	if hasIDs {
		for _, id := range input.IDs {
			if id <= 0 || id > math.MaxInt32 {
				return ValidationError{Messages: []string{"ids must contain positive integers"}}
			}
		}
		return nil
	}
	if input.Sender == nil || strings.TrimSpace(*input.Sender) == "" {
		return ValidationError{Messages: []string{"sender is required"}}
	}
	if input.Chat == nil || strings.TrimSpace(*input.Chat) == "" {
		return ValidationError{Messages: []string{"chat is required"}}
	}
	if len(input.MessageIDs) == 0 {
		return ValidationError{Messages: []string{"messageIds must contain at least one item"}}
	}
	for _, id := range input.MessageIDs {
		if strings.TrimSpace(id) == "" {
			return ValidationError{Messages: []string{"messageIds must not contain empty items"}}
		}
	}
	return nil
}

func validateArchiveChat(input ArchiveChatRequest) error {
	if err := validateStruct(input); err != nil {
		return err
	}
	if input.Archive == nil {
		return ValidationError{Messages: []string{"archive is required"}}
	}
	if strings.TrimSpace(input.LastMessage.Key.RemoteJID) == "" {
		return ValidationError{Messages: []string{"lastMessage.key.remoteJid is required"}}
	}
	if input.LastMessage.Key.FromMe == nil {
		return ValidationError{Messages: []string{"lastMessage.key.fromMe is required"}}
	}
	if strings.TrimSpace(input.LastMessage.Key.ID) == "" {
		return ValidationError{Messages: []string{"lastMessage.key.id is required"}}
	}
	return nil
}

func validateFetchProfilePicture(input FetchProfilePictureRequest) error {
	if _, err := input.ResolveRecipient(); err != nil {
		return ValidationError{Messages: []string{"exactly one of number, chat or recipient is required"}}
	}
	return nil
}

func validateRejectCall(input RejectCallRequest) error {
	if err := validateStruct(input); err != nil {
		return err
	}
	if strings.TrimSpace(input.CallID) == "" {
		return ValidationError{Messages: []string{"callId is required"}}
	}
	if strings.TrimSpace(input.CallFrom) == "" {
		return ValidationError{Messages: []string{"callFrom is required"}}
	}
	return nil
}

func validateEditMessage(input EditMessageRequest) error {
	if err := validateStruct(input); err != nil {
		return err
	}
	if input.ID.NumericID == nil && input.ID.KeyID == nil {
		return ValidationError{Messages: []string{"id is required"}}
	}
	if strings.TrimSpace(input.Text) == "" {
		return ValidationError{Messages: []string{"text is required"}}
	}
	if len(input.Text) > MaxEditTextLength {
		return ValidationError{Messages: []string{"text must be less than 65536"}}
	}
	return nil
}
