package chat

import "errors"

var (
	ErrInstanceDisconnected = errors.New("instance disconnected")
	ErrMessageNotOutgoing   = errors.New("message was not sent by this instance")
	ErrMessageNotEditable   = errors.New("message is not editable")
	ErrInvalidRecipient     = errors.New("invalid recipient")
	ErrInvalidRequestMode   = errors.New("invalid request mode")
	ErrDatabaseOperation    = errors.New("database operation failed")
	ErrRemoteOperation      = errors.New("whatsapp operation failed")
	ErrInvalidMediaRequest  = errors.New("invalid media request")
	ErrMediaMessageNotFound = errors.New("media message not found")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrInvalidMediaContent  = errors.New("invalid media content")
	ErrMessageIsNotMedia    = errors.New("message is not media")
	ErrMediaDownloadFailed  = errors.New("media download failed")
	ErrMediaTooLarge        = errors.New("media too large")
)
