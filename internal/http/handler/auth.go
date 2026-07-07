package handler

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

func bearerToken(c fiber.Ctx) (string, error) {
	authorization := strings.TrimSpace(c.Get("Authorization"))
	parts := strings.Fields(authorization)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", fiber.NewError(fiber.StatusUnauthorized)
	}
	return strings.TrimSpace(parts[1]), nil
}
