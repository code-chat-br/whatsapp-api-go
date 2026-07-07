package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type MessageRepository interface {
	Create(ctx context.Context, input types.CreateMessageInput) (types.Message, error)
	CreateOrIgnore(ctx context.Context, input types.CreateMessageInput) error
	FindByIDForInstance(ctx context.Context, instanceID int32, id int32) (types.Message, error)
	FindByKeyIDForInstance(ctx context.Context, instanceID int32, keyID string) (types.Message, error)
	FindByIDsForInstance(ctx context.Context, instanceID int32, ids []int32) ([]types.Message, error)
	FindOutgoingByIDForInstance(ctx context.Context, instanceID int32, id int32) (types.Message, error)
	FindOutgoingByKeyIDForInstance(ctx context.Context, instanceID int32, keyID string) (types.Message, error)
	MarkReadForInstance(ctx context.Context, instanceID int32, ids []int32) error
	UpdateContentForInstance(ctx context.Context, instanceID int32, id int32, content json.RawMessage) (types.Message, error)
	Count(ctx context.Context, instanceID int32, filters types.MessageFilters) (int64, error)
	List(ctx context.Context, instanceID int32, input types.ListMessagesInput) (types.MessageListResult, error)
}

type messageRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewMessageRepository(pool *pgxpool.Pool, logger zerolog.Logger) MessageRepository {
	return &messageRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "message_repository").Logger(),
	}
}

func (r *messageRepository) Create(ctx context.Context, input types.CreateMessageInput) (types.Message, error) {
	if len(input.Content) == 0 || !json.Valid(input.Content) {
		return types.Message{}, fmt.Errorf("%w: content", ErrInvalidJSON)
	}
	if len(input.Metadata) > 0 && !json.Valid(input.Metadata) {
		return types.Message{}, fmt.Errorf("%w: metadata", ErrInvalidJSON)
	}
	if !input.Device.IsValid() {
		return types.Message{}, fmt.Errorf("%w: device", ErrInvalidEnum)
	}
	exists, err := r.q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Message{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Message{}, ErrInstanceNotFound
	}

	message, err := r.q.CreateMessage(ctx, createMessageParams(input))
	if err != nil {
		if isForeignKeyViolation(err) {
			return types.Message{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "message.create").Int32("instanceId", input.InstanceID).Msg("failed to create message")
		return types.Message{}, fmt.Errorf("create message: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) CreateOrIgnore(ctx context.Context, input types.CreateMessageInput) error {
	if len(input.Content) == 0 || !json.Valid(input.Content) {
		return fmt.Errorf("%w: content", ErrInvalidJSON)
	}
	if len(input.Metadata) > 0 && !json.Valid(input.Metadata) {
		return fmt.Errorf("%w: metadata", ErrInvalidJSON)
	}
	if !input.Device.IsValid() {
		return fmt.Errorf("%w: device", ErrInvalidEnum)
	}
	exists, err := r.q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return ErrInstanceNotFound
	}
	params := createMessageParams(input)
	err = r.q.CreateMessageOrIgnore(ctx, db.CreateMessageOrIgnoreParams(params))
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "message.create_or_ignore").Int32("instanceId", input.InstanceID).Str("messageKeyId", input.KeyID).Msg("failed to create message")
		return fmt.Errorf("create message or ignore: %w", err)
	}
	return nil
}

func createMessageParams(input types.CreateMessageInput) db.CreateMessageParams {
	return db.CreateMessageParams{
		Keyid:             input.KeyID,
		KeyRemoteJid:      textFromPtr(input.KeyRemoteJid),
		KeyLid:            textFromPtr(input.KeyLid),
		Keyfromme:         input.KeyFromMe,
		KeyParticipant:    textFromPtr(input.KeyParticipant),
		KeyParticipantLid: textFromPtr(input.KeyParticipantLid),
		PushName:          textFromPtr(input.PushName),
		Messagetype:       input.MessageType,
		Content:           input.Content,
		Messagetimestamp:  input.MessageTimestamp,
		Device:            db.DeviceMessage(input.Device),
		IsGroup:           boolFromPtr(input.IsGroup),
		Instanceid:        input.InstanceID,
		Metadata:          nullableJSON(input.Metadata),
	}
}

func (r *messageRepository) FindByIDForInstance(ctx context.Context, instanceID int32, id int32) (types.Message, error) {
	message, err := r.q.FindMessageByIDForInstance(ctx, db.FindMessageByIDForInstanceParams{Instanceid: instanceID, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Message{}, ErrMessageNotFound
		}
		r.logger.Error().Err(err).Str("operation", "message.find_by_id").Int32("instanceId", instanceID).Int32("messageId", id).Msg("failed to find message")
		return types.Message{}, fmt.Errorf("find message by id: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) FindByKeyIDForInstance(ctx context.Context, instanceID int32, keyID string) (types.Message, error) {
	message, err := r.q.FindMessageByKeyIDForInstance(ctx, db.FindMessageByKeyIDForInstanceParams{Instanceid: instanceID, Keyid: keyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Message{}, ErrMessageNotFound
		}
		r.logger.Error().Err(err).Str("operation", "message.find_by_key_id").Int32("instanceId", instanceID).Str("keyId", keyID).Msg("failed to find message")
		return types.Message{}, fmt.Errorf("find message by key id: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) FindByIDsForInstance(ctx context.Context, instanceID int32, ids []int32) ([]types.Message, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: ids", ErrInvalidInput)
	}
	rows, err := r.q.FindMessagesByIDsForInstance(ctx, db.FindMessagesByIDsForInstanceParams{Instanceid: instanceID, Ids: ids})
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "message.find_by_ids").Int32("instanceId", instanceID).Msg("failed to find messages")
		return nil, fmt.Errorf("find messages by ids: %w", err)
	}
	result := make([]types.Message, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapMessage(row))
	}
	return result, nil
}

func (r *messageRepository) FindOutgoingByIDForInstance(ctx context.Context, instanceID int32, id int32) (types.Message, error) {
	message, err := r.q.FindOutgoingMessageByIDForInstance(ctx, db.FindOutgoingMessageByIDForInstanceParams{Instanceid: instanceID, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Message{}, ErrMessageNotFound
		}
		r.logger.Error().Err(err).Str("operation", "message.find_outgoing_by_id").Int32("instanceId", instanceID).Int32("messageId", id).Msg("failed to find outgoing message")
		return types.Message{}, fmt.Errorf("find outgoing message by id: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) FindOutgoingByKeyIDForInstance(ctx context.Context, instanceID int32, keyID string) (types.Message, error) {
	message, err := r.q.FindOutgoingMessageByKeyIDForInstance(ctx, db.FindOutgoingMessageByKeyIDForInstanceParams{Instanceid: instanceID, Keyid: keyID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Message{}, ErrMessageNotFound
		}
		r.logger.Error().Err(err).Str("operation", "message.find_outgoing_by_key_id").Int32("instanceId", instanceID).Str("keyId", keyID).Msg("failed to find outgoing message")
		return types.Message{}, fmt.Errorf("find outgoing message by key id: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) MarkReadForInstance(ctx context.Context, instanceID int32, ids []int32) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: ids", ErrInvalidInput)
	}
	rows, err := r.q.MarkMessagesReadForInstance(ctx, db.MarkMessagesReadForInstanceParams{
		Datetime:   pgTimestamp(time.Now().UTC()),
		Instanceid: instanceID,
		Ids:        ids,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "message.mark_read").Int32("instanceId", instanceID).Msg("failed to mark messages read")
		return fmt.Errorf("mark messages read: %w", err)
	}
	if rows != int64(len(ids)) {
		return ErrMessageNotFound
	}
	return nil
}

func (r *messageRepository) UpdateContentForInstance(ctx context.Context, instanceID int32, id int32, content json.RawMessage) (types.Message, error) {
	if len(content) == 0 || !json.Valid(content) {
		return types.Message{}, fmt.Errorf("%w: content", ErrInvalidJSON)
	}
	message, err := r.q.UpdateMessageContentForInstance(ctx, db.UpdateMessageContentForInstanceParams{
		Content:    content,
		Instanceid: instanceID,
		ID:         id,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Message{}, ErrMessageNotFound
		}
		r.logger.Error().Err(err).Str("operation", "message.update_content").Int32("instanceId", instanceID).Int32("messageId", id).Msg("failed to update message content")
		return types.Message{}, fmt.Errorf("update message content: %w", err)
	}
	return mapMessage(message), nil
}

func (r *messageRepository) Count(ctx context.Context, instanceID int32, filters types.MessageFilters) (int64, error) {
	if filters.ID != nil {
		_, err := r.q.FindMessageByIDForInstance(ctx, db.FindMessageByIDForInstanceParams{Instanceid: instanceID, ID: *filters.ID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return 0, nil
			}
			return 0, fmt.Errorf("find message by id: %w", err)
		}
		return 1, nil
	}
	count, err := r.q.CountMessages(ctx, countMessagesParams(instanceID, filters))
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "message.count").Int32("instanceId", instanceID).Msg("failed to count messages")
		return 0, fmt.Errorf("count messages: %w", err)
	}
	return count, nil
}

func (r *messageRepository) List(ctx context.Context, instanceID int32, input types.ListMessagesInput) (types.MessageListResult, error) {
	if input.Limit <= 0 {
		return types.MessageListResult{}, fmt.Errorf("%w: limit must be positive", ErrInvalidInput)
	}
	if input.Direction == "" {
		input.Direction = types.CursorDirectionNext
	}
	if input.Direction != types.CursorDirectionNext && input.Direction != types.CursorDirectionPrevious {
		return types.MessageListResult{}, fmt.Errorf("%w: cursor direction", ErrInvalidEnum)
	}

	if input.Filters.ID != nil {
		return r.listByID(ctx, instanceID, *input.Filters.ID)
	}

	var messages []db.Message
	var err error
	if input.Direction == types.CursorDirectionPrevious {
		messages, err = r.q.ListMessagesPrevious(ctx, listMessagesPreviousParams(instanceID, input))
		slices.Reverse(messages)
	} else {
		messages, err = r.q.ListMessagesNext(ctx, listMessagesNextParams(instanceID, input))
	}
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "message.list").Int32("instanceId", instanceID).Msg("failed to list messages")
		return types.MessageListResult{}, fmt.Errorf("list messages: %w", err)
	}

	total, err := r.Count(ctx, instanceID, input.Filters)
	if err != nil {
		return types.MessageListResult{}, err
	}
	records, err := r.composeMessages(ctx, messages)
	if err != nil {
		return types.MessageListResult{}, err
	}

	pages := int64(math.Ceil(float64(total) / float64(input.Limit)))
	currentPage := int64(0)
	if len(messages) > 0 {
		before, err := r.q.CountMessagesBeforeID(ctx, countMessagesBeforeIDParams(instanceID, messages[0].ID, input.Filters))
		if err != nil {
			return types.MessageListResult{}, fmt.Errorf("count messages before page: %w", err)
		}
		currentPage = before/int64(input.Limit) + 1
	}

	return types.MessageListResult{
		Messages: types.MessagePage{
			Total:       total,
			Pages:       pages,
			CurrentPage: currentPage,
			Records:     records,
		},
	}, nil
}

func (r *messageRepository) listByID(ctx context.Context, instanceID int32, id int32) (types.MessageListResult, error) {
	message, err := r.q.FindMessageByIDForInstance(ctx, db.FindMessageByIDForInstanceParams{Instanceid: instanceID, ID: id})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.MessageListResult{Messages: types.MessagePage{CurrentPage: 1}}, nil
		}
		return types.MessageListResult{}, fmt.Errorf("find message by id: %w", err)
	}
	records, err := r.composeMessages(ctx, []db.Message{message})
	if err != nil {
		return types.MessageListResult{}, err
	}
	return types.MessageListResult{
		Messages: types.MessagePage{
			Total:       1,
			Pages:       1,
			CurrentPage: 1,
			Records:     records,
		},
	}, nil
}

func (r *messageRepository) composeMessages(ctx context.Context, messages []db.Message) ([]types.MessageWithUpdates, error) {
	if len(messages) == 0 {
		return []types.MessageWithUpdates{}, nil
	}
	ids := make([]int32, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	updates, err := r.q.ListMessageUpdatesByMessageIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list message updates: %w", err)
	}
	updatesByMessage := make(map[int32][]types.MessageUpdateSummary, len(messages))
	for _, update := range updates {
		updatesByMessage[update.MessageId] = append(updatesByMessage[update.MessageId], types.MessageUpdateSummary{
			Status:   update.Status,
			DateTime: timestamp(update.DateTime),
		})
	}
	records := make([]types.MessageWithUpdates, 0, len(messages))
	for _, message := range messages {
		records = append(records, types.MessageWithUpdates{
			Message:       mapMessage(message),
			MessageUpdate: updatesByMessage[message.ID],
		})
	}
	return records, nil
}

func countMessagesParams(instanceID int32, filters types.MessageFilters) db.CountMessagesParams {
	params := db.CountMessagesParams{Instanceid: instanceID}
	applyMessageFilters(&params, filters)
	return params
}

func listMessagesNextParams(instanceID int32, input types.ListMessagesInput) db.ListMessagesNextParams {
	params := db.ListMessagesNextParams{
		Instanceid: instanceID,
		Limitcount: input.Limit,
	}
	if input.Cursor != nil {
		params.Hascursor = true
		params.Cursor = *input.Cursor
	}
	applyMessageFilters(&params, input.Filters)
	return params
}

func listMessagesPreviousParams(instanceID int32, input types.ListMessagesInput) db.ListMessagesPreviousParams {
	params := db.ListMessagesPreviousParams{
		Instanceid: instanceID,
		Limitcount: input.Limit,
	}
	if input.Cursor != nil {
		params.Hascursor = true
		params.Cursor = *input.Cursor
	}
	applyMessageFilters(&params, input.Filters)
	return params
}

func countMessagesBeforeIDParams(instanceID int32, id int32, filters types.MessageFilters) db.CountMessagesBeforeIDParams {
	params := db.CountMessagesBeforeIDParams{Instanceid: instanceID, ID: id}
	applyMessageFilters(&params, filters)
	return params
}

type messageFilterTarget interface {
	db.CountMessagesParams | db.ListMessagesNextParams | db.ListMessagesPreviousParams | db.CountMessagesBeforeIDParams
}

func applyMessageFilters[T messageFilterTarget](params *T, filters types.MessageFilters) {
	switch p := any(params).(type) {
	case *db.CountMessagesParams:
		applyCountFilters(p, filters)
	case *db.ListMessagesNextParams:
		applyNextFilters(p, filters)
	case *db.ListMessagesPreviousParams:
		applyPreviousFilters(p, filters)
	case *db.CountMessagesBeforeIDParams:
		applyBeforeFilters(p, filters)
	}
}

func applyCountFilters(p *db.CountMessagesParams, filters types.MessageFilters) {
	if filters.KeyID != nil {
		p.Filterkeyid, p.Keyid = true, *filters.KeyID
	}
	if filters.KeyRemoteJid != nil {
		p.Filterkeyremotejid, p.Keyremotejid = true, textFromPtr(filters.KeyRemoteJid)
	}
	if filters.KeyFromMe != nil {
		p.Filterkeyfromme, p.Keyfromme = true, *filters.KeyFromMe
	}
	if filters.MessageType != nil {
		p.Filtermessagetype, p.Messagetype = true, *filters.MessageType
	}
	if filters.Device != nil {
		p.Filterdevice, p.Device = true, db.DeviceMessage(*filters.Device)
	}
	if filters.MessageTimestampGTE != nil {
		p.Filtermessagetimestampgte, p.Messagetimestampgte = true, *filters.MessageTimestampGTE
	}
	if filters.MessageTimestampLTE != nil {
		p.Filtermessagetimestamplte, p.Messagetimestamplte = true, *filters.MessageTimestampLTE
	}
	if filters.MessageStatus != nil {
		p.Filtermessagestatus, p.Messagestatus = true, *filters.MessageStatus
	}
}

func applyNextFilters(p *db.ListMessagesNextParams, filters types.MessageFilters) {
	count := db.CountMessagesParams{}
	applyCountFilters(&count, filters)
	p.Filterkeyid, p.Keyid = count.Filterkeyid, count.Keyid
	p.Filterkeyremotejid, p.Keyremotejid = count.Filterkeyremotejid, count.Keyremotejid
	p.Filterkeyfromme, p.Keyfromme = count.Filterkeyfromme, count.Keyfromme
	p.Filtermessagetype, p.Messagetype = count.Filtermessagetype, count.Messagetype
	p.Filterdevice, p.Device = count.Filterdevice, count.Device
	p.Filtermessagetimestampgte, p.Messagetimestampgte = count.Filtermessagetimestampgte, count.Messagetimestampgte
	p.Filtermessagetimestamplte, p.Messagetimestamplte = count.Filtermessagetimestamplte, count.Messagetimestamplte
	p.Filtermessagestatus, p.Messagestatus = count.Filtermessagestatus, count.Messagestatus
}

func applyPreviousFilters(p *db.ListMessagesPreviousParams, filters types.MessageFilters) {
	next := db.ListMessagesNextParams{}
	applyNextFilters(&next, filters)
	p.Filterkeyid, p.Keyid = next.Filterkeyid, next.Keyid
	p.Filterkeyremotejid, p.Keyremotejid = next.Filterkeyremotejid, next.Keyremotejid
	p.Filterkeyfromme, p.Keyfromme = next.Filterkeyfromme, next.Keyfromme
	p.Filtermessagetype, p.Messagetype = next.Filtermessagetype, next.Messagetype
	p.Filterdevice, p.Device = next.Filterdevice, next.Device
	p.Filtermessagetimestampgte, p.Messagetimestampgte = next.Filtermessagetimestampgte, next.Messagetimestampgte
	p.Filtermessagetimestamplte, p.Messagetimestamplte = next.Filtermessagetimestamplte, next.Messagetimestamplte
	p.Filtermessagestatus, p.Messagestatus = next.Filtermessagestatus, next.Messagestatus
}

func applyBeforeFilters(p *db.CountMessagesBeforeIDParams, filters types.MessageFilters) {
	next := db.ListMessagesNextParams{}
	applyNextFilters(&next, filters)
	p.Filterkeyid, p.Keyid = next.Filterkeyid, next.Keyid
	p.Filterkeyremotejid, p.Keyremotejid = next.Filterkeyremotejid, next.Keyremotejid
	p.Filterkeyfromme, p.Keyfromme = next.Filterkeyfromme, next.Keyfromme
	p.Filtermessagetype, p.Messagetype = next.Filtermessagetype, next.Messagetype
	p.Filterdevice, p.Device = next.Filterdevice, next.Device
	p.Filtermessagetimestampgte, p.Messagetimestampgte = next.Filtermessagetimestampgte, next.Messagetimestampgte
	p.Filtermessagetimestamplte, p.Messagetimestamplte = next.Filtermessagetimestamplte, next.Messagetimestamplte
	p.Filtermessagestatus, p.Messagestatus = next.Filtermessagestatus, next.Messagestatus
}
