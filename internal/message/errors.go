package message

import "errors"

var (
	ErrInvalidRequest          = errors.New("invalid message request")
	ErrRecipientInvalid        = errors.New("invalid recipient")
	ErrPresenceInvalid         = errors.New("invalid presence")
	ErrDelayInvalid            = errors.New("invalid delay")
	ErrQuotedMessageInvalid    = errors.New("invalid quoted message")
	ErrQuotedMessageLookup     = errors.New("quoted message lookup failed")
	ErrPersistenceFailed       = errors.New("message sent but could not be persisted")
	ErrDownloadFailed          = errors.New("media download failed")
	ErrUploadFailed            = errors.New("media upload failed")
	ErrSendFailed              = errors.New("whatsapp send failed")
	ErrPayloadTooLarge         = errors.New("payload too large")
	ErrUnsupportedMediaType    = errors.New("unsupported media type")
	ErrAudioProcessing         = errors.New("audio processing failed")
	ErrInvalidAudioDuration    = errors.New("invalid audio duration")
	ErrMentionAllRequiresGroup = errors.New("mention all requires group")
	ErrMentionAllUnsupported   = errors.New("mention all is not supported for message type")
	ErrMessageQueueFull        = errors.New("message processing queue is full")
	ErrMessageProcessorStopped = errors.New("message processing manager is stopped")
	ErrGroupInfoFetchFailed    = errors.New("group info fetch failed")
	ErrGroupHasNoParticipants  = errors.New("group has no participants")
	ErrGroupMentionProcessing  = errors.New("group mention processing failed")
)
