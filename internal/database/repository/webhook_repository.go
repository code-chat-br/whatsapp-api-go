package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

type WebhookRepository interface {
	Create(ctx context.Context, input types.CreateWebhookInput) (types.Webhook, error)
	CreateTx(ctx context.Context, q *db.Queries, input types.CreateWebhookInput) (types.Webhook, error)
	FindByInstanceName(ctx context.Context, instanceName string) (types.Webhook, error)
	FindByInstanceNameTx(ctx context.Context, q *db.Queries, instanceName string) (types.Webhook, error)
	ListEnabledWithInstance(ctx context.Context) ([]types.WebhookWithInstance, error)
	Update(ctx context.Context, webhookID int32, input types.UpdateWebhookInput) (types.Webhook, error)
	UpdateTx(ctx context.Context, q *db.Queries, webhookID int32, input types.UpdateWebhookInput) (types.Webhook, error)
	UpsertEvents(ctx context.Context, webhookID int32, events map[string]bool) (types.Webhook, error)
	UpsertEventsTx(ctx context.Context, q *db.Queries, webhookID int32, events map[string]bool) (types.Webhook, error)
}

type webhookRepository struct {
	q      *db.Queries
	logger zerolog.Logger
}

func NewWebhookRepository(pool *pgxpool.Pool, logger zerolog.Logger) WebhookRepository {
	return &webhookRepository{
		q:      db.New(pool),
		logger: logger.With().Str("component", "webhook_repository").Logger(),
	}
}

func (r *webhookRepository) Create(ctx context.Context, input types.CreateWebhookInput) (types.Webhook, error) {
	return r.CreateTx(ctx, r.q, input)
}

func (r *webhookRepository) CreateTx(ctx context.Context, q *db.Queries, input types.CreateWebhookInput) (types.Webhook, error) {
	if err := validateWebhookEventsJSON(input.Events); err != nil {
		return types.Webhook{}, err
	}
	exists, err := q.InstanceExists(ctx, input.InstanceID)
	if err != nil {
		return types.Webhook{}, fmt.Errorf("check instance exists: %w", err)
	}
	if !exists {
		return types.Webhook{}, ErrInstanceNotFound
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	webhook, err := q.CreateWebhook(ctx, db.CreateWebhookParams{
		Url:        input.URL,
		Enabled:    enabled,
		Events:     nullableJSON(input.Events),
		Instanceid: input.InstanceID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return types.Webhook{}, fmt.Errorf("%w: %w", ErrWebhookAlreadyExists, err)
		}
		if isForeignKeyViolation(err) {
			return types.Webhook{}, fmt.Errorf("%w: %w", ErrInstanceNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "webhook.create").Int32("instanceId", input.InstanceID).Msg("failed to create webhook")
		return types.Webhook{}, fmt.Errorf("create webhook: %w", err)
	}
	return mapWebhook(webhook), nil
}

func (r *webhookRepository) FindByInstanceName(ctx context.Context, instanceName string) (types.Webhook, error) {
	return r.FindByInstanceNameTx(ctx, r.q, instanceName)
}

func (r *webhookRepository) FindByInstanceNameTx(ctx context.Context, q *db.Queries, instanceName string) (types.Webhook, error) {
	webhook, err := q.FindWebhookByInstanceName(ctx, instanceName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Webhook{}, fmt.Errorf("%w: %w", ErrWebhookNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "webhook.find_by_instance_name").Msg("failed to find webhook")
		return types.Webhook{}, fmt.Errorf("find webhook by instance name: %w", err)
	}
	return mapWebhook(webhook), nil
}

func (r *webhookRepository) ListEnabledWithInstance(ctx context.Context) ([]types.WebhookWithInstance, error) {
	rows, err := r.q.ListEnabledWebhooksWithInstance(ctx)
	if err != nil {
		r.logger.Error().Err(err).Str("operation", "webhook.list_enabled").Msg("failed to list enabled webhooks")
		return nil, fmt.Errorf("list enabled webhooks: %w", err)
	}
	result := make([]types.WebhookWithInstance, 0, len(rows))
	for _, row := range rows {
		result = append(result, types.WebhookWithInstance{
			Webhook: types.Webhook{
				ID:         row.ID,
				URL:        row.Url,
				Enabled:    row.Enabled,
				Events:     json.RawMessage(row.Events),
				CreatedAt:  timestamp(row.CreatedAt),
				UpdatedAt:  timestamp(row.UpdatedAt),
				InstanceID: row.InstanceId,
			},
			InstanceName: row.InstanceName,
		})
	}
	return result, nil
}

func (r *webhookRepository) Update(ctx context.Context, webhookID int32, input types.UpdateWebhookInput) (types.Webhook, error) {
	return r.UpdateTx(ctx, r.q, webhookID, input)
}

func (r *webhookRepository) UpdateTx(ctx context.Context, q *db.Queries, webhookID int32, input types.UpdateWebhookInput) (types.Webhook, error) {
	params := db.UpdateWebhookParams{ID: webhookID}
	if input.URL != nil {
		params.Seturl = true
		params.Url = *input.URL
	}
	if input.Enabled != nil {
		params.Setenabled = true
		params.Enabled = *input.Enabled
	}
	webhook, err := q.UpdateWebhook(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Webhook{}, fmt.Errorf("%w: %w", ErrWebhookNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "webhook.update").Int32("webhookId", webhookID).Msg("failed to update webhook")
		return types.Webhook{}, fmt.Errorf("update webhook: %w", err)
	}
	return mapWebhook(webhook), nil
}

func (r *webhookRepository) UpsertEvents(ctx context.Context, webhookID int32, events map[string]bool) (types.Webhook, error) {
	return r.UpsertEventsTx(ctx, r.q, webhookID, events)
}

func (r *webhookRepository) UpsertEventsTx(ctx context.Context, q *db.Queries, webhookID int32, events map[string]bool) (types.Webhook, error) {
	for event := range events {
		if !types.IsWebhookEventField(event) {
			return types.Webhook{}, fmt.Errorf("%w: %s", ErrInvalidWebhookEvent, event)
		}
	}

	var webhook db.Webhook
	var err error
	if len(events) == 0 {
		webhook, err = q.ClearWebhookEvents(ctx, webhookID)
	} else {
		payload, marshalErr := json.Marshal(events)
		if marshalErr != nil {
			return types.Webhook{}, fmt.Errorf("marshal webhook events: %w", marshalErr)
		}
		webhook, err = q.MergeWebhookEvents(ctx, db.MergeWebhookEventsParams{
			Events: payload,
			ID:     webhookID,
		})
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Webhook{}, fmt.Errorf("%w: %w", ErrWebhookNotFound, err)
		}
		r.logger.Error().Err(err).Str("operation", "webhook.upsert_events").Int32("webhookId", webhookID).Msg("failed to upsert webhook events")
		return types.Webhook{}, fmt.Errorf("upsert webhook events: %w", err)
	}
	return mapWebhook(webhook), nil
}

func validateWebhookEventsJSON(value json.RawMessage) error {
	if len(value) == 0 {
		return nil
	}
	if !json.Valid(value) {
		return fmt.Errorf("%w: events", ErrInvalidJSON)
	}
	var events map[string]bool
	if err := json.Unmarshal(value, &events); err != nil {
		return fmt.Errorf("%w: events", ErrInvalidJSON)
	}
	for event := range events {
		if !types.IsWebhookEventField(event) {
			return fmt.Errorf("%w: %s", ErrInvalidWebhookEvent, event)
		}
	}
	return nil
}
