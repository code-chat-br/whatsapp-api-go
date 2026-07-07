package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type MessageUpdateRepository interface {
	Create(ctx context.Context, input types.CreateMessageUpdateInput) (types.MessageUpdate, error)
	CreateOrIgnore(ctx context.Context, input types.CreateMessageUpdateInput) error
	ListByMessageID(ctx context.Context, messageID int32) ([]types.MessageUpdate, error)
}

type messageUpdateRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewMessageUpdateRepository(pool *pgxpool.Pool, logger zerolog.Logger) MessageUpdateRepository {
	return &messageUpdateRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "message_update_repository").Logger(),
	}
}

func (r *messageUpdateRepository) Create(ctx context.Context, input types.CreateMessageUpdateInput) (types.MessageUpdate, error) {
	exists, err := r.q.MessageExists(ctx, input.MessageID)
	if err != nil {
		return types.MessageUpdate{}, fmt.Errorf("check message exists: %w", err)
	}
	if !exists {
		return types.MessageUpdate{}, ErrMessageNotFound
	}
	update, err := r.q.CreateMessageUpdate(ctx, db.CreateMessageUpdateParams{
		Datetime:  pgTimestamp(input.DateTime),
		Status:    input.Status,
		Messageid: input.MessageID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return types.MessageUpdate{}, fmt.Errorf("%w: %w", ErrMessageNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "message_update.create").Int32("messageId", input.MessageID).Msg("failed to create message update")
		return types.MessageUpdate{}, fmt.Errorf("create message update: %w", err)
	}
	return mapMessageUpdate(update), nil
}

func (r *messageUpdateRepository) CreateOrIgnore(ctx context.Context, input types.CreateMessageUpdateInput) error {
	exists, err := r.q.MessageExists(ctx, input.MessageID)
	if err != nil {
		return fmt.Errorf("check message exists: %w", err)
	}
	if !exists {
		return ErrMessageNotFound
	}
	err = r.q.CreateMessageUpdateOrIgnore(ctx, db.CreateMessageUpdateOrIgnoreParams{
		Datetime:  pgTimestamp(input.DateTime),
		Status:    input.Status,
		Messageid: input.MessageID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: %w", ErrMessageNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "message_update.create_or_ignore").Int32("messageId", input.MessageID).Msg("failed to create message update")
		return fmt.Errorf("create message update or ignore: %w", err)
	}
	return nil
}

func (r *messageUpdateRepository) ListByMessageID(ctx context.Context, messageID int32) ([]types.MessageUpdate, error) {
	rows, err := r.q.ListMessageUpdatesByMessageID(ctx, messageID)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "message_update.list_by_message_id").Int32("messageId", messageID).Msg("failed to list message updates")
		return nil, fmt.Errorf("list message updates by message id: %w", err)
	}
	result := make([]types.MessageUpdate, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapMessageUpdate(row))
	}
	return result, nil
}
