package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/rs/zerolog"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/chat"
	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/group"
	"whatsapp-go-api/internal/http/handler"
	"whatsapp-go-api/internal/http/middleware"
	"whatsapp-go-api/internal/http/validation"
	"whatsapp-go-api/internal/instance"
	"whatsapp-go-api/internal/message"
	webhooksvc "whatsapp-go-api/internal/webhook"
	"whatsapp-go-api/internal/whatsapp"
)

func NewServer(
	logger zerolog.Logger,
	config config.Config,
	instanceService instance.Service,
	webhookService webhooksvc.Service,
	whatsAppService whatsapp.ConnectionService,
	messageService message.Service,
	chatService chat.Service,
	groupService group.Service,
	readiness Readiness,
) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      "whatsapp-go-api",
		ErrorHandler: ErrorHandler(logger),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		BodyLimit:    2 * 1024 * 1024,
	})

	app.Use(recover.New())
	app.Use(middleware.RequestLogger(logger))

	requestValidator := validation.New()
	instanceHandler := handler.NewInstanceHandler(instanceService, requestValidator, logger)
	var webhookHandler *handler.WebhookHandler
	if webhookService != nil {
		webhookHandler = handler.NewWebhookHandler(webhookService, requestValidator, logger)
	}
	var tokenValidator authjwt.Validator
	tokenValidator, err := authjwt.NewJWTValidator(config.Authentication, logger)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create JWT validator")
		tokenValidator = rejectingJWTValidator{}
	}

	var whatsAppHandler *handler.WhatsAppConnectionHandler
	if whatsAppService != nil {
		whatsAppHandler = handler.NewWhatsAppConnectionHandler(whatsAppService, logger)
	}
	var messageHandler *handler.MessageHandler
	if messageService != nil {
		messageHandler = handler.NewMessageHandler(messageService, logger)
	}
	var chatHandler *handler.ChatHandler
	if chatService != nil {
		chatHandler = handler.NewChatHandler(chatService, logger)
	}
	var groupHandler *handler.GroupHandler
	if groupService != nil {
		groupHandler = handler.NewGroupHandler(groupService, logger)
	}
	RegisterRoutes(app, config, instanceHandler, webhookHandler, whatsAppHandler, messageHandler, chatHandler, groupHandler, tokenValidator, readiness)

	return app
}

type rejectingJWTValidator struct{}

func (rejectingJWTValidator) Validate(string) (authjwt.InstanceClaims, error) {
	return authjwt.InstanceClaims{}, errors.New("JWT validator is not configured")
}
