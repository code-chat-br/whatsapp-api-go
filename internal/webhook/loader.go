package webhook

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/repository"
	"whatsapp-go-api/internal/database/types"
)

func LoadCache(ctx context.Context, repo repository.WebhookRepository, cache WebhookCache, logger zerolog.Logger) error {
	if repo == nil || cache == nil {
		return nil
	}
	rows, err := repo.ListEnabledWithInstance(ctx)
	if err != nil {
		return fmt.Errorf("load enabled webhook cache: %w", err)
	}

	enabled := make([]CachedWebhook, 0, len(rows))
	invalid := 0
	for _, row := range rows {
		webhook, err := CachedWebhookFromModel(row.Webhook, row.InstanceName)
		if err != nil {
			invalid++
			logger.Warn().
				Err(err).
				Int32("webhookId", row.Webhook.ID).
				Int32("instanceId", row.Webhook.InstanceID).
				Str("instanceName", row.InstanceName).
				Msg("invalid webhook ignored during cache load")
			continue
		}
		enabled = append(enabled, webhook)
	}

	cache.Load(ctx, enabled)
	logger.Info().
		Int("enabledWebhooks", len(enabled)).
		Int("invalidWebhooks", invalid).
		Msg("webhook cache loaded")
	return nil
}

func CachedWebhookFromModel(model types.Webhook, instanceName string) (CachedWebhook, error) {
	normalizedURL, err := NormalizeURL(model.URL)
	if err != nil {
		return CachedWebhook{}, err
	}
	events, err := types.ParseWebhookEvents(model.Events)
	if err != nil {
		return CachedWebhook{}, err
	}
	return CachedWebhook{
		ID:           int64(model.ID),
		InstanceID:   int64(model.InstanceID),
		InstanceName: instanceName,
		URL:          normalizedURL,
		Enabled:      model.Enabled,
		Events:       events,
		UpdatedAt:    model.UpdatedAt,
	}, nil
}
