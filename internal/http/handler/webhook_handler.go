package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/http/request"
	"whatsapp-go-api/internal/http/response"
	"whatsapp-go-api/internal/http/validation"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

type WebhookHandler struct {
	service   webhooksvc.Service
	validator validation.RequestValidator
	logger    zerolog.Logger
}

func NewWebhookHandler(service webhooksvc.Service, validator validation.RequestValidator, logger zerolog.Logger) *WebhookHandler {
	return &WebhookHandler{
		service:   service,
		validator: validator,
		logger:    logger.With().Str("component", "webhook_handler").Logger(),
	}
}

func (h *WebhookHandler) Set(c fiber.Ctx) error {
	var body request.SetWebhookRequest
	if err := decodeStrictBody(c, &body); err != nil {
		return err
	}
	if err := h.validator.Validate(&body); err != nil {
		h.logger.Debug().Err(err).Str("operation", "webhook.set").Msg("request validation failed")
		return err
	}
	token, err := bearerToken(c)
	if err != nil {
		return err
	}

	webhook, err := h.service.Set(c.Context(), c.Params("instanceName"), token, webhooksvc.SetInput{
		URL:       body.URL,
		Enabled:   body.Enabled,
		Events:    body.Events,
		EventsSet: body.EventsSet,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(mapWebhookResponse(webhook))
}

func (h *WebhookHandler) Find(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	webhook, err := h.service.Find(c.Context(), c.Params("instanceName"), token)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(mapWebhookResponse(webhook))
}

func mapWebhookResponse(webhook types.Webhook) response.WebhookResponse {
	return response.WebhookResponse{
		ID:         webhook.ID,
		URL:        webhook.URL,
		Enabled:    webhook.Enabled,
		Events:     webhookEvents(webhook.Events),
		CreatedAt:  timePtr(webhook.CreatedAt),
		UpdatedAt:  webhook.UpdatedAt,
		InstanceID: webhook.InstanceID,
	}
}
