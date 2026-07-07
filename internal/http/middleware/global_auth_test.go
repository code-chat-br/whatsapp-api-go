package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestGlobalAuthAcceptedHeaders(t *testing.T) {
	for _, header := range acceptedAPIKeyHeaders {
		t.Run(header, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", GlobalAuth("secret"), func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set(header, " secret ")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != fiber.StatusNoContent {
				t.Fatalf("expected status 204, got %d", resp.StatusCode)
			}
		})
	}
}

func TestGlobalAuthRejectsMissingWrongAndConflictingHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header func(*http.Request)
		want   int
	}{
		{name: "missing", want: fiber.StatusUnauthorized},
		{
			name: "wrong",
			header: func(req *http.Request) {
				req.Header.Set("apikey", "wrong")
			},
			want: fiber.StatusUnauthorized,
		},
		{
			name: "conflicting",
			header: func(req *http.Request) {
				req.Header.Set("apikey", "secret")
				req.Header.Set("x-api-key", "other")
			},
			want: fiber.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", GlobalAuth("secret"), func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != nil {
				tt.header(req)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != tt.want {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("expected status %d, got %d body=%s", tt.want, resp.StatusCode, strings.TrimSpace(string(body)))
			}
		})
	}
}
