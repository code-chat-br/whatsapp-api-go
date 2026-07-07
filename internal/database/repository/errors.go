package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInstanceNotFound            = errors.New("instance not found")
	ErrInstanceNameAlreadyExists   = errors.New("instance name already exists")
	ErrInstanceHasDependencies     = errors.New("instance has dependencies")
	ErrAuthAlreadyExists           = errors.New("auth already exists")
	ErrAuthNotFound                = errors.New("auth not found")
	ErrInvalidOldToken             = errors.New("invalid old token")
	ErrTokenDoesNotMatch           = errors.New("token does not match")
	ErrTokenInstanceMismatch       = errors.New("token instance mismatch")
	ErrWhatsAppDeviceAlreadyLinked = errors.New("whatsapp device already linked")
	ErrWebhookAlreadyExists        = errors.New("webhook already exists")
	ErrMessageNotFound             = errors.New("message not found")
	ErrInvalidWebhookEvent         = errors.New("invalid webhook event")
	ErrInvalidWebhookURL           = errors.New("invalid webhook url")
	ErrInvalidJSON                 = errors.New("invalid json")
	ErrInvalidEnum                 = errors.New("invalid enum")
	ErrInvalidInput                = errors.New("invalid input")
	ErrWebhookNotFound             = errors.New("webhook not found")
)

const (
	postgresUniqueViolation     = "23505"
	postgresForeignKeyViolation = "23503"
)

type InstanceDependenciesError struct {
	InstanceID int32
	Messages   int64
	Chats      int64
	Contacts   int64
	Webhooks   int64
}

func (e *InstanceDependenciesError) Error() string {
	return fmt.Sprintf(
		"instance %d has dependencies: messages=%d chats=%d contacts=%d webhooks=%d",
		e.InstanceID,
		e.Messages,
		e.Chats,
		e.Contacts,
		e.Webhooks,
	)
}

func (e *InstanceDependenciesError) Unwrap() error {
	return ErrInstanceHasDependencies
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolation
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == postgresForeignKeyViolation
}
