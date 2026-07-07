package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"mime/multipart"
	"net/textproto"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/chat"
)

type ChatHandler struct {
	service chat.Service
	logger  zerolog.Logger
}

func NewChatHandler(service chat.Service, logger zerolog.Logger) *ChatHandler {
	return &ChatHandler{
		service: service,
		logger:  logger.With().Str("component", "chat_handler").Logger(),
	}
}

func (h *ChatHandler) WhatsAppNumbers(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.WhatsAppNumbersRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.CheckWhatsAppNumbers(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func (h *ChatHandler) ReadMessages(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.ReadMessagesRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.service.ReadMessages(c.Context(), c.Params("instanceName"), token, body); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(chat.ReadMessagesResponse{Message: "Read messages", Read: "success"})
}

func (h *ChatHandler) ArchiveChat(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.ArchiveChatRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.service.ArchiveChat(c.Context(), c.Params("instanceName"), token, body); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ChatHandler) DeleteMessage(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	id, err := chat.Int64FromQuery(c.Query("id"))
	if err != nil {
		return err
	}
	if err := h.service.DeleteMessageForEveryone(c.Context(), c.Params("instanceName"), token, id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ChatHandler) FetchProfilePictureURL(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.FetchProfilePictureRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	url, err := h.service.FetchProfilePicture(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(chat.ProfilePictureResponse{ProfilePictureURL: url})
}

func (h *ChatHandler) RejectCall(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.RejectCallRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.service.RejectCall(c.Context(), c.Params("instanceName"), token, body); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ChatHandler) EditMessage(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body chat.EditMessageRequest
	if err := c.Bind().Body(&body); err != nil {
		var syntax *chat.ValidationError
		if errors.As(err, &syntax) {
			return syntax
		}
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.EditMessage(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func (h *ChatHandler) MediaData(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	binary, err := parseOptionalBool(c.Query("binary"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	var body chat.MediaDataRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if _, err := body.Validate(); err != nil {
		return err
	}
	result, err := h.service.MediaData(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	if binary {
		return writeBinaryMedia(c, result)
	}
	return writeMultipartMedia(c, result)
}

func writeBinaryMedia(c fiber.Ctx, result chat.MediaDownloadResult) error {
	c.Set(fiber.HeaderContentType, result.MIMEType)
	c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("inline", map[string]string{"filename": result.FileName}))
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentLength, strconv.Itoa(len(result.Data)))
	return c.Status(fiber.StatusOK).Send(result.Data)
}

func writeMultipartMedia(c fiber.Ctx, result chat.MediaDownloadResult) error {
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	if err := writer.WriteField("mediaType", result.MediaType); err != nil {
		return err
	}
	if err := writer.WriteField("fileName", result.FileName); err != nil {
		return err
	}
	sizeJSON, err := json.Marshal(result.Size)
	if err != nil {
		return err
	}
	if err := writer.WriteField("size", string(sizeJSON)); err != nil {
		return err
	}
	if err := writer.WriteField("mimetype", result.MIMEType); err != nil {
		return err
	}

	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name":     "file",
		"filename": result.FileName,
	}))
	headers.Set("Content-Type", result.MIMEType)
	filePart, err := writer.CreatePart(headers)
	if err != nil {
		return err
	}
	if _, err := filePart.Write(result.Data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	c.Set(fiber.HeaderContentType, writer.FormDataContentType())
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set(fiber.HeaderContentLength, strconv.Itoa(buffer.Len()))
	return c.Status(fiber.StatusOK).Send(buffer.Bytes())
}
