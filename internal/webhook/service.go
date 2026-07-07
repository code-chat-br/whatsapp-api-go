package webhook

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/postgres"
	"whatsapp-go-api/internal/database/repository"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

const maxWebhookURLLength = 500

type Service interface {
	Set(ctx context.Context, instanceName string, bearerToken string, input SetInput) (types.Webhook, error)
	Find(ctx context.Context, instanceName string, bearerToken string) (types.Webhook, error)
}

type SetInput struct {
	URL       string
	Enabled   *bool
	Events    map[string]bool
	EventsSet bool
}

type WebhookService struct {
	pool      *pgxpool.Pool
	instances repository.InstanceRepository
	webhooks  repository.WebhookRepository
	cache     WebhookCache
	logger    zerolog.Logger
}

func NewService(
	pool *pgxpool.Pool,
	instances repository.InstanceRepository,
	webhooks repository.WebhookRepository,
	cache WebhookCache,
	logger zerolog.Logger,
) *WebhookService {
	return &WebhookService{
		pool:      pool,
		instances: instances,
		webhooks:  webhooks,
		cache:     cache,
		logger:    logger.With().Str("component", "webhook_service").Logger(),
	}
}

func (s *WebhookService) Set(ctx context.Context, instanceName string, bearerToken string, input SetInput) (types.Webhook, error) {
	name, err := normalizeInstanceName(instanceName)
	if err != nil {
		return types.Webhook{}, err
	}
	webhookURL, err := normalizeWebhookURL(input.URL)
	if err != nil {
		return types.Webhook{}, err
	}
	if err := validateEvents(input.Events); err != nil {
		return types.Webhook{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	var output types.Webhook
	err = postgres.WithTransaction(ctx, s.pool, s.logger, func(q *db.Queries) error {
		instance, err := s.authorize(ctx, q, name, bearerToken)
		if err != nil {
			return err
		}

		current, err := s.webhooks.FindByInstanceNameTx(ctx, q, name)
		if err != nil {
			if !errors.Is(err, repository.ErrWebhookNotFound) {
				return err
			}
			current, err = s.webhooks.CreateTx(ctx, q, types.CreateWebhookInput{
				URL:        webhookURL,
				Enabled:    &enabled,
				InstanceID: instance.Instance.ID,
			})
			if err != nil {
				return err
			}
		} else {
			current, err = s.webhooks.UpdateTx(ctx, q, current.ID, types.UpdateWebhookInput{
				URL:     &webhookURL,
				Enabled: &enabled,
			})
			if err != nil {
				return err
			}
		}

		if input.EventsSet {
			current, err = s.webhooks.UpsertEventsTx(ctx, q, current.ID, input.Events)
			if err != nil {
				return err
			}
		}

		output = current
		return nil
	})
	if err != nil {
		return types.Webhook{}, err
	}
	s.syncCache(name, output)

	s.logger.Info().
		Str("operation", "webhook.set").
		Str("instanceName", name).
		Int32("webhookId", output.ID).
		Msg("webhook configured")

	return output, nil
}

func (s *WebhookService) syncCache(instanceName string, model types.Webhook) {
	if s.cache == nil {
		return
	}
	if !model.Enabled {
		s.cache.Delete(int64(model.InstanceID), instanceName)
		return
	}
	cached, err := CachedWebhookFromModel(model, instanceName)
	if err != nil {
		s.cache.Delete(int64(model.InstanceID), instanceName)
		s.logger.Warn().
			Err(err).
			Int32("webhookId", model.ID).
			Int32("instanceId", model.InstanceID).
			Str("instanceName", instanceName).
			Msg("webhook removed from cache after invalid configuration")
		return
	}
	s.cache.Set(int64(model.InstanceID), instanceName, cached)
}

func (s *WebhookService) Find(ctx context.Context, instanceName string, bearerToken string) (types.Webhook, error) {
	name, err := normalizeInstanceName(instanceName)
	if err != nil {
		return types.Webhook{}, err
	}

	var output types.Webhook
	err = postgres.WithTransaction(ctx, s.pool, s.logger, func(q *db.Queries) error {
		if _, err := s.authorize(ctx, q, name, bearerToken); err != nil {
			return err
		}
		webhook, err := s.webhooks.FindByInstanceNameTx(ctx, q, name)
		if err != nil {
			return err
		}
		output = webhook
		return nil
	})
	if err != nil {
		return types.Webhook{}, err
	}
	return output, nil
}

func (s *WebhookService) authorize(ctx context.Context, q *db.Queries, instanceName string, bearerToken string) (types.InstanceWithAuth, error) {
	token := strings.TrimSpace(bearerToken)
	if token == "" {
		return types.InstanceWithAuth{}, repository.ErrInvalidOldToken
	}
	instance, err := s.instances.FindByNameTx(ctx, q, instanceName)
	if err != nil {
		return types.InstanceWithAuth{}, err
	}
	if instance.Auth == nil {
		return types.InstanceWithAuth{}, repository.ErrAuthNotFound
	}
	if subtle.ConstantTimeCompare([]byte(instance.Auth.Token), []byte(token)) != 1 {
		return types.InstanceWithAuth{}, repository.ErrInvalidOldToken
	}
	return instance, nil
}

func normalizeInstanceName(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > 255 {
		return "", repository.ErrInvalidInput
	}
	return normalized, nil
}

func normalizeWebhookURL(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > maxWebhookURLLength {
		return "", repository.ErrInvalidWebhookURL
	}
	normalized, err := NormalizeURL(normalized)
	if err != nil {
		return "", repository.ErrInvalidWebhookURL
	}
	return normalized, nil
}

func validateEvents(events map[string]bool) error {
	for event := range events {
		if !types.IsWebhookEventField(event) {
			return fmt.Errorf("%w: %s", repository.ErrInvalidWebhookEvent, event)
		}
	}
	return nil
}
