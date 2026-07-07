package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
)

func InstanceAuth(validator authjwt.Validator) fiber.Handler {
	return func(c fiber.Ctx) error {
		authorization := strings.TrimSpace(c.Get("Authorization"))
		if authorization == "" {
			return fiber.NewError(fiber.StatusUnauthorized)
		}

		parts := strings.Fields(authorization)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			return fiber.NewError(fiber.StatusUnauthorized)
		}

		claims, err := validator.Validate(parts[1])
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized)
		}

		if claims.InstanceName != c.Params("instanceName") {
			return fiber.NewError(fiber.StatusForbidden)
		}

		return c.Next()
	}
}
