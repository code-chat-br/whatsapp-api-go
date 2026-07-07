package handler

import (
	"fmt"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/http/request"
	"whatsapp-go-api/internal/message"
)

type MessageHandler struct {
	service message.Service
	logger  zerolog.Logger
}

func NewMessageHandler(service message.Service, logger zerolog.Logger) *MessageHandler {
	return &MessageHandler{
		service: service,
		logger:  logger.With().Str("component", "message_handler").Logger(),
	}
}

func (h *MessageHandler) SendText(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendTextRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendText(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendLink(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendLinkRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendLink(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendMedia(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendMediaRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendMedia(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendMediaFile(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	options, err := message.ParseMultipartMessageOptions(
		c.FormValue("delay"),
		c.FormValue("presence"),
		c.FormValue("quotedMessageId"),
		c.FormValue("quotedMessage"),
		c.FormValue("mentionAll"),
	)
	if err != nil {
		return err
	}
	header, err := c.FormFile("attachment")
	if err != nil {
		return fmt.Errorf("%w: attachment is required", message.ErrInvalidRequest)
	}
	file, err := header.Open()
	if err != nil {
		return fmt.Errorf("%w: open attachment", message.ErrInvalidRequest)
	}
	defer file.Close()
	result, err := h.service.SendMediaFile(c.Context(), c.Params("instanceName"), token, c.FormValue("number"), file, header, c.FormValue("mediaType"), optionalFormString(c.FormValue("caption")), options)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendWhatsAppAudio(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendWhatsAppAudioRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendWhatsAppAudio(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendWhatsAppAudioFile(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	options, err := message.ParseMultipartAudioOptions(
		c.FormValue("delay"),
		c.FormValue("presence"),
		c.FormValue("quotedMessageId"),
		c.FormValue("quotedMessage"),
		c.FormValue("mentionAll"),
	)
	if err != nil {
		return err
	}
	header, err := c.FormFile("attachment")
	if err != nil {
		return fmt.Errorf("%w: attachment is required", message.ErrInvalidRequest)
	}
	file, err := header.Open()
	if err != nil {
		return fmt.Errorf("%w: open attachment", message.ErrInvalidRequest)
	}
	defer file.Close()
	result, err := h.service.SendWhatsAppAudioFile(c.Context(), c.Params("instanceName"), token, c.FormValue("number"), file, header, options)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendContact(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendContactRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendContact(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendLocation(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendLocationRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendLocation(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func (h *MessageHandler) SendReaction(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body request.SendReactionRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.SendReaction(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return sendMessageResult(c, result)
}

func sendMessageResult(c fiber.Ctx, result message.SendResult) error {
	if result.Accepted != nil {
		return c.Status(fiber.StatusAccepted).JSON(result.Accepted)
	}
	return c.Status(fiber.StatusOK).JSON(result.Message)
}

func optionalFormString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
