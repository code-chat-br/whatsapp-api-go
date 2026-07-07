package handler

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/http/request"
	"whatsapp-go-api/internal/http/response"
	"whatsapp-go-api/internal/http/validation"
	"whatsapp-go-api/internal/instance"
)

type InstanceHandler struct {
	service   instance.Service
	validator validation.RequestValidator
	logger    zerolog.Logger
}

func NewInstanceHandler(service instance.Service, validator validation.RequestValidator, logger zerolog.Logger) *InstanceHandler {
	return &InstanceHandler{
		service:   service,
		validator: validator,
		logger:    logger.With().Str("component", "instance_handler").Logger(),
	}
}

func (h *InstanceHandler) Create(c fiber.Ctx) error {
	var body request.CreateInstanceRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.validator.Validate(&body); err != nil {
		h.logger.Debug().Err(err).Str("operation", "instance.create").Msg("request validation failed")
		return fiber.NewError(fiber.StatusUnprocessableEntity)
	}

	result, err := h.service.Create(c.Context(), instance.CreateInstanceInput{
		InstanceName:       body.InstanceName,
		Description:        body.Description,
		ExternalAttributes: body.ExternalAttributes,
	})
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(response.CreateInstanceResponse{
		ID:          result.Instance.ID,
		Name:        result.Instance.Name,
		Description: result.Instance.Description,
		CreatedAt:   timePtr(result.Instance.CreatedAt),
		UpdatedAt:   timePtr(result.Instance.UpdatedAt),
		Auth: response.CreateInstanceAuthResponse{
			ID:    result.Auth.ID,
			Token: result.Auth.Token,
		},
	})
}

func (h *InstanceHandler) List(c fiber.Ctx) error {
	var filter *string
	if c.Request().URI().QueryArgs().Has("instanceName") {
		raw := c.Query("instanceName")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return fiber.NewError(fiber.StatusUnprocessableEntity)
		}
		filter = &trimmed
	}

	items, err := h.service.List(c.Context(), filter)
	if err != nil {
		return err
	}

	output := make([]response.InstanceListItemResponse, 0, len(items))
	for _, item := range items {
		output = append(output, mapListItem(item))
	}
	return c.Status(fiber.StatusOK).JSON(output)
}

func (h *InstanceHandler) Fetch(c fiber.Ctx) error {
	item, err := h.service.FetchByName(c.Context(), c.Params("instanceName"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(mapFetchItem(item))
}

func (h *InstanceHandler) RefreshToken(c fiber.Ctx) error {
	var body request.RefreshInstanceTokenRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.validator.Validate(&body); err != nil {
		h.logger.Debug().Err(err).Str("operation", "instance.refresh_token").Msg("request validation failed")
		return err
	}
	if strings.TrimSpace(body.OldToken) == "" {
		return fiber.NewError(fiber.StatusBadRequest)
	}

	token, err := bearerToken(c)
	if err != nil {
		return err
	}

	auth, err := h.service.RefreshToken(c.Context(), c.Params("instanceName"), token, body.OldToken)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(response.RefreshInstanceTokenResponse{
		ID:         auth.ID,
		CreatedAt:  auth.CreatedAt,
		UpdatedAt:  auth.UpdatedAt,
		Token:      auth.Token,
		InstanceID: auth.InstanceID,
	})
}

func mapListItem(item types.InstanceDetails) response.InstanceListItemResponse {
	return response.InstanceListItemResponse{
		ID:               item.Instance.ID,
		Name:             item.Instance.Name,
		Description:      item.Instance.Description,
		Status:           item.Instance.Status,
		ConnectionStatus: item.Instance.ConnectionStatus,
		OwnerJid:         item.Instance.OwnerJid,
		ProfilePicURL:    item.Instance.ProfilePicUrl,
		CreatedAt:        timePtr(item.Instance.CreatedAt),
		UpdatedAt:        timePtr(item.Instance.UpdatedAt),
		Auth:             mapAuth(item.Auth),
		Webhook:          mapWebhook(item.Webhook),
	}
}

func mapFetchItem(item types.InstanceDetails) response.InstanceFetchResponse {
	return response.InstanceFetchResponse{
		ID:               item.Instance.ID,
		Name:             item.Instance.Name,
		Description:      item.Instance.Description,
		Status:           item.Instance.Status,
		ConnectionStatus: item.Instance.ConnectionStatus,
		OwnerJid:         item.Instance.OwnerJid,
		ProfilePicURL:    item.Instance.ProfilePicUrl,
		CreatedAt:        timePtr(item.Instance.CreatedAt),
		UpdatedAt:        timePtr(item.Instance.UpdatedAt),
		Webhook:          mapWebhook(item.Webhook),
	}
}

func mapAuth(auth *types.Auth) *response.InstanceAuthResponse {
	if auth == nil {
		return nil
	}
	return &response.InstanceAuthResponse{
		ID:        auth.ID,
		Token:     auth.Token,
		CreatedAt: auth.CreatedAt,
		UpdatedAt: auth.UpdatedAt,
	}
}

func mapWebhook(webhook *types.Webhook) *response.WebhookResponse {
	if webhook == nil {
		return nil
	}
	return &response.WebhookResponse{
		ID:         webhook.ID,
		URL:        webhook.URL,
		Enabled:    webhook.Enabled,
		Events:     webhookEvents(webhook.Events),
		CreatedAt:  timePtr(webhook.CreatedAt),
		UpdatedAt:  webhook.UpdatedAt,
		InstanceID: webhook.InstanceID,
	}
}

func webhookEvents(events json.RawMessage) json.RawMessage {
	if len(events) == 0 {
		return json.RawMessage("{}")
	}
	return json.RawMessage(events)
}

func timePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}
