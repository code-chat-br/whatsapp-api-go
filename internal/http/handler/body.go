package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v3"
)

func decodeStrictBody(c fiber.Ctx, dst any) error {
	decoder := json.NewDecoder(bytes.NewReader(c.Body()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return fiber.NewError(fiber.StatusBadRequest)
	}
	return nil
}
