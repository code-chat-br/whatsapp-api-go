package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/group"
)

type GroupHandler struct {
	service group.Service
	logger  zerolog.Logger
}

func NewGroupHandler(service group.Service, logger zerolog.Logger) *GroupHandler {
	return &GroupHandler{
		service: service,
		logger:  logger.With().Str("component", "group_handler").Logger(),
	}
}

func (h *GroupHandler) Create(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body group.CreateRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.Create(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *GroupHandler) UpdatePicture(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body group.UpdatePictureRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	result, err := h.service.UpdatePicture(c.Context(), c.Params("instanceName"), token, body)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func (h *GroupHandler) InviteCode(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	result, err := h.service.InviteCode(c.Context(), c.Params("instanceName"), token, c.Query("groupJid"))
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func (h *GroupHandler) RevokeInviteCode(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	if err := h.service.RevokeInviteCode(c.Context(), c.Params("instanceName"), token, c.Query("groupJid")); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).SendString("")
}

func (h *GroupHandler) UpdateParticipant(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	var body group.UpdateParticipantRequest
	if err := c.Bind().Body(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := h.service.UpdateParticipant(c.Context(), c.Params("instanceName"), token, c.Query("groupJid"), body); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).SendString("")
}

func (h *GroupHandler) Leave(c fiber.Ctx) error {
	token, err := bearerToken(c)
	if err != nil {
		return err
	}
	if err := h.service.Leave(c.Context(), c.Params("instanceName"), token, c.Query("groupJid")); err != nil {
		return err
	}
	return c.Status(fiber.StatusOK).SendString("")
}
