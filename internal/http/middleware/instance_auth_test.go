package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/config"
)

func TestInstanceAuthAcceptsValidBearerWithAndWithoutExp(t *testing.T) {
	for _, expiresIn := range []int64{0, 3600} {
		t.Run("valid", func(t *testing.T) {
			validator := newJWTValidator(t, "secret", 3600)
			token := newToken(t, "secret", "codechat", expiresIn, gojwt.SigningMethodHS256)
			app := fiber.New()
			app.Get("/instance/fetchInstance/:instanceName", InstanceAuth(validator), func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/instance/fetchInstance/codechat", nil)
			req.Header.Set("Authorization", "Bearer "+token)
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

func TestInstanceAuthRejectsInvalidAuthorization(t *testing.T) {
	validator := newJWTValidator(t, "secret", 3600)
	validOther := newToken(t, "secret", "other", 3600, gojwt.SigningMethodHS256)
	badSignature := newToken(t, "other-secret", "codechat", 3600, gojwt.SigningMethodHS256)
	expired := newExpiredToken(t, "secret", "codechat")
	badAlg := newToken(t, "secret", "codechat", 3600, gojwt.SigningMethodHS384)

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "missing", want: fiber.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic value", want: fiber.StatusUnauthorized},
		{name: "empty token", header: "Bearer ", want: fiber.StatusUnauthorized},
		{name: "malformed", header: "Bearer not-a-token", want: fiber.StatusUnauthorized},
		{name: "bad signature", header: "Bearer " + badSignature, want: fiber.StatusUnauthorized},
		{name: "expired", header: "Bearer " + expired, want: fiber.StatusUnauthorized},
		{name: "bad algorithm", header: "Bearer " + badAlg, want: fiber.StatusUnauthorized},
		{name: "other instance", header: "Bearer " + validOther, want: fiber.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/instance/fetchInstance/:instanceName", InstanceAuth(validator), func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodGet, "/instance/fetchInstance/codechat", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			if resp.StatusCode != tt.want {
				t.Fatalf("expected status %d, got %d", tt.want, resp.StatusCode)
			}
		})
	}
}

func TestInstanceAuthAcceptsExpiredBearerWhenJWTExpirationDisabled(t *testing.T) {
	validator := newJWTValidator(t, "secret", 0)
	token := newExpiredToken(t, "secret", "codechat")
	app := fiber.New()
	app.Get("/instance/fetchInstance/:instanceName", InstanceAuth(validator), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/instance/fetchInstance/codechat", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("expected status 204, got %d", resp.StatusCode)
	}
}

func newJWTValidator(t *testing.T, secret string, expiresInSeconds int64) *authjwt.JWTValidator {
	t.Helper()
	validator, err := authjwt.NewJWTValidator(config.AuthenticationConfig{
		JWTSecret:           secret,
		JWTExpiresInSeconds: expiresInSeconds,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewJWTValidator() error = %v", err)
	}
	return validator
}

func newToken(t *testing.T, secret string, instanceName string, expiresIn int64, method gojwt.SigningMethod) string {
	t.Helper()
	claims := authjwt.InstanceClaims{InstanceName: instanceName}
	if expiresIn > 0 {
		claims.ExpiresAt = gojwt.NewNumericDate(time.Now().UTC().Add(time.Duration(expiresIn) * time.Second))
	}
	token, err := gojwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func newExpiredToken(t *testing.T, secret string, instanceName string) string {
	t.Helper()
	claims := authjwt.InstanceClaims{
		InstanceName: instanceName,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().UTC().Add(-time.Hour)),
		},
	}
	token, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}
