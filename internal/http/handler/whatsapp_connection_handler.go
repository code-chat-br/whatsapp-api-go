package handler

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/http/response"
	"whatsapp-go-api/internal/http/validation"
	"whatsapp-go-api/internal/whatsapp"
)

type WhatsAppConnectionHandler struct {
	service   whatsapp.ConnectionService
	validator validation.RequestValidator
	logger    zerolog.Logger
}

func NewWhatsAppConnectionHandler(service whatsapp.ConnectionService, logger zerolog.Logger) *WhatsAppConnectionHandler {
	return &WhatsAppConnectionHandler{
		service:   service,
		validator: validation.New(),
		logger:    logger.With().Str("component", "whatsapp_connection_handler").Logger(),
	}
}

func (h *WhatsAppConnectionHandler) ConnectQRCode(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.ConnectQRCode(c.Context(), c.Params("instanceName"), token)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.QRCodeConnectionResponse{
		Count:             result.Count,
		Code:              result.Code,
		Base64:            result.Base64,
		InstanceName:      result.InstanceName,
		ConnectionStatus:  result.ConnectionStatus,
		AlreadyConnected:  result.AlreadyConnected,
		AlreadyConnecting: result.AlreadyConnecting,
		OwnerJid:          result.OwnerJid,
	})
}

func (h *WhatsAppConnectionHandler) ConnectPhone(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.ConnectPhone(c.Context(), c.Params("instanceName"), token, c.Params("phoneNumber"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.PhonePairingResponse{Code: result.Code})
}

func (h *WhatsAppConnectionHandler) RequestPasskeyChallenge(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.RequestPasskeyChallenge(c.Context(), c.Params("instanceName"), token)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.PasskeyChallengeResponse{
		RequestID: result.RequestID,
		State:     result.State,
		ExpiresAt: result.ExpiresAt,
		PublicKey: result.PublicKey,
	})
}

func (h *WhatsAppConnectionHandler) SubmitPasskeyAssertion(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body whatsapp.SubmitPasskeyAssertionRequest
	if err := decodeStrictBody(c, &body); err != nil {
		return err
	}
	if err := h.validator.Validate(&body); err != nil {
		return err
	}
	result, err := h.service.SubmitPasskeyAssertion(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(response.PasskeyAssertionResponse{
		State:   result.State,
		Message: result.Message,
	})
}

func (h *WhatsAppConnectionHandler) ConnectionState(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.ConnectionState(c.Context(), c.Params("instanceName"), token)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.ConnectionStateResponse{
		State:            result.State,
		StatusReason:     result.StatusReason,
		InstanceName:     result.InstanceName,
		ConnectionStatus: result.ConnectionStatus,
		Connected:        result.Connected,
		LoggedIn:         result.LoggedIn,
		OwnerJid:         result.OwnerJid,
	})
}

func (h *WhatsAppConnectionHandler) Logout(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.Logout(c.Context(), c.Params("instanceName"), token)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.InstanceLogoutResponse{
		InstanceName:     result.InstanceName,
		State:            result.State,
		ConnectionStatus: result.ConnectionStatus,
		Message:          result.Message,
	})
}

func (h *WhatsAppConnectionHandler) Delete(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	force, err := parseOptionalBool(c.Query("force", "false"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.DeleteInstance(c.Context(), c.Params("instanceName"), token, force)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(response.InstanceDeleteResponse{
		InstanceName: result.InstanceName,
		Deleted:      result.Deleted,
		Forced:       result.Forced,
		Message:      result.Message,
	})
}

func parseOptionalBool(value string) (bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "false" {
		return false, nil
	}
	if trimmed == "true" {
		return true, nil
	}
	return false, strconv.ErrSyntax
}
