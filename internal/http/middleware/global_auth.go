package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"
)

var acceptedAPIKeyHeaders = []string{"apikey", "x-api-key", "apiKey"}

func GlobalAuth(expected string) fiber.Handler {
	expected = strings.TrimSpace(expected)
	return func(c fiber.Ctx) error {
		values := make([]string, 0, len(acceptedAPIKeyHeaders))
		for _, header := range acceptedAPIKeyHeaders {
			value := c.Get(header)
			if value != "" {
				values = append(values, strings.TrimSpace(value))
			}
		}
		if len(values) == 0 {
			return fiber.NewError(fiber.StatusUnauthorized)
		}

		first := values[0]
		for _, value := range values[1:] {
			if value != first {
				return fiber.NewError(fiber.StatusBadRequest)
			}
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(first)) != 1 {
			return fiber.NewError(fiber.StatusUnauthorized)
		}

		return c.Next()
	}
}
