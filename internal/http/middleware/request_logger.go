package middleware

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

func RequestLogger(logger zerolog.Logger) fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			status = fiber.StatusInternalServerError
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				status = fiberErr.Code
			}
		}

		logger.Info().
			Str("method", c.Method()).
			Str("path", c.Path()).
			Str("route", c.Route().Path).
			Int("status", status).
			Dur("duration", time.Since(start)).
			Msg("http request")

		return err
	}
}
