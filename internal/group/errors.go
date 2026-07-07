package group

import "errors"

var (
	ErrInstanceDisconnected = errors.New("instance disconnected")
	ErrInvalidGroupJID      = errors.New("invalid group jid")
	ErrInvalidParticipant   = errors.New("invalid participant")
	ErrInvalidRequest       = errors.New("invalid group request")
	ErrRemoteOperation      = errors.New("whatsapp group operation failed")
	ErrDownloadFailed       = errors.New("group image download failed")
	ErrImageTooLarge        = errors.New("group image too large")
)
