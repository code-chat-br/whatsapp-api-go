package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type ChatRepository interface {
	Create(ctx context.Context, input types.CreateChatInput) (types.Chat, error)
	List(ctx context.Context, instanceID int32, filter *types.ChatType) ([]types.Chat, error)
}

type chatRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewChatRepository(pool *pgxpool.Pool, logger zerolog.Logger) ChatRepository {
	return &chatRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "chat_repository").Logger(),
	}
}

func (r *chatRepository) Create(ctx context.Context, input types.CreateChatInput) (types.Chat, error) {
	if len(input.Content) > 0 && !json.Valid(input.Content) {
		return types.Chat{}, fmt.Errorf("%w: content", ErrInvalidJSON)
	}
	exists, err := r.q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Chat{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Chat{}, ErrInstanceNotFound
	}
	chat, err := r.q.CreateChat(ctx, db.CreateChatParams{
		Remotejid:  input.RemoteJid,
		Content:    nullableJSON(input.Content),
		Instanceid: input.InstanceID,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			return types.Chat{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "chat.create").Int32("instanceId", input.InstanceID).Msg("failed to create chat")
		return types.Chat{}, fmt.Errorf("create chat: %w", err)
	}
	return mapChat(chat), nil
}

func (r *chatRepository) List(ctx context.Context, instanceID int32, filter *types.ChatType) ([]types.Chat, error) {
	chatType := ""
	if filter != nil {
		if !filter.IsValid() {
			return nil, fmt.Errorf("%w: chat type", ErrInvalidEnum)
		}
		chatType = string(*filter)
	}
	rows, err := r.q.ListChats(ctx, db.ListChatsParams{Instanceid: instanceID, Chattype: chatType})
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "chat.list").Int32("instanceId", instanceID).Msg("failed to list chats")
		return nil, fmt.Errorf("list chats: %w", err)
	}
	result := make([]types.Chat, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapChat(row))
	}
	return result, nil
}
