package http

import (
	"github.com/gofiber/fiber/v3"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/http/handler"
	"whatsapp-go-api/internal/http/middleware"
	"whatsapp-go-api/internal/http/response"
)

type Readiness interface {
	Ready() bool
}

func RegisterRoutes(
	app *fiber.App,
	config config.Config,
	instanceHandler *handler.InstanceHandler,
	webhookHandler *handler.WebhookHandler,
	whatsAppHandler *handler.WhatsAppConnectionHandler,
	messageHandler *handler.MessageHandler,
	chatHandler *handler.ChatHandler,
	groupHandler *handler.GroupHandler,
	tokenValidator authjwt.Validator,
	readiness Readiness,
) {
	globalAuthMiddleware := middleware.GlobalAuth(config.Authentication.GlobalAuthToken)
	instanceAuthMiddleware := middleware.InstanceAuth(tokenValidator)

	app.Get("/health", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/ready", func(c fiber.Ctx) error {
		if readiness != nil && readiness.Ready() {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ready"})
		}
		return c.Status(fiber.StatusServiceUnavailable).JSON(response.ErrorResponse{
			StatusCode: fiber.StatusServiceUnavailable,
			Error:      response.ErrorServiceUnavailable,
			Message:    []string{"Servico indisponivel."},
		})
	})

	instance := app.Group("/instance")

	instance.Post("/create", globalAuthMiddleware, instanceHandler.Create)
	instance.Get("/create", globalAuthMiddleware, instanceHandler.List)
	instance.Get("/fetchInstances", globalAuthMiddleware, instanceHandler.List)
	instance.Get("/fetchInstance/:instanceName", instanceAuthMiddleware, instanceHandler.Fetch)

	if whatsAppHandler != nil {
		instance.Get("/connect/:instanceName/code/:phoneNumber", instanceAuthMiddleware, whatsAppHandler.ConnectPhone)
		instance.Post("/connect/:instanceName/passkey/challenge", instanceAuthMiddleware, whatsAppHandler.RequestPasskeyChallenge)
		instance.Post("/connect/:instanceName/passkey/assertion", instanceAuthMiddleware, whatsAppHandler.SubmitPasskeyAssertion)
		instance.Get("/connect/:instanceName", instanceAuthMiddleware, whatsAppHandler.ConnectQRCode)
		instance.Post("/connect/:instanceName", instanceAuthMiddleware, whatsAppHandler.ConnectQRCode)
		instance.Delete("/logout/:instanceName", instanceAuthMiddleware, whatsAppHandler.Logout)
	}

	instance.Get("/connectionState/:instanceName", instanceAuthMiddleware, whatsAppHandler.ConnectionState)
	instance.Put("/refreshToken/:instanceName", instanceAuthMiddleware, instanceHandler.RefreshToken)
	instance.Delete("/delete/:instanceName", instanceAuthMiddleware, whatsAppHandler.Delete)

	if webhookHandler != nil {
		webhooks := app.Group("/webhook")
		webhooks.Put("/set/:instanceName", instanceAuthMiddleware, webhookHandler.Set)
		webhooks.Get("/find/:instanceName", instanceAuthMiddleware, webhookHandler.Find)
	}

	if messageHandler != nil {
		messages := app.Group("/message")
		messages.Post("/sendText/:instanceName", instanceAuthMiddleware, messageHandler.SendText)
		messages.Post("/sendLink/:instanceName", instanceAuthMiddleware, messageHandler.SendLink)
		messages.Post("/sendMedia/:instanceName", instanceAuthMiddleware, messageHandler.SendMedia)
		messages.Post("/sendMediaFile/:instanceName", instanceAuthMiddleware, messageHandler.SendMediaFile)
		messages.Post("/sendWhatsAppAudio/:instanceName", instanceAuthMiddleware, messageHandler.SendWhatsAppAudio)
		messages.Post("/sendWhatsAppAudioFile/:instanceName", instanceAuthMiddleware, messageHandler.SendWhatsAppAudioFile)
		messages.Post("/sendContact/:instanceName", instanceAuthMiddleware, messageHandler.SendContact)
		messages.Post("/sendLocation/:instanceName", instanceAuthMiddleware, messageHandler.SendLocation)
		messages.Post("/sendReaction/:instanceName", instanceAuthMiddleware, messageHandler.SendReaction)
	}

	if chatHandler != nil {
		chats := app.Group("/chat")
		chats.Post("/whatsappNumbers/:instanceName", instanceAuthMiddleware, chatHandler.WhatsAppNumbers)
		chats.Patch("/readMessages/:instanceName", instanceAuthMiddleware, chatHandler.ReadMessages)
		chats.Put("/archiveChat/:instanceName", instanceAuthMiddleware, chatHandler.ArchiveChat)
		chats.Delete("/deleteMessage/:instanceName", instanceAuthMiddleware, chatHandler.DeleteMessage)
		chats.Post("/fetchProfilePictureUrl/:instanceName", instanceAuthMiddleware, chatHandler.FetchProfilePictureURL)
		chats.Post("/rejectCall/:instanceName", instanceAuthMiddleware, chatHandler.RejectCall)
		chats.Post("/editMessage/:instanceName", instanceAuthMiddleware, chatHandler.EditMessage)
		chats.Post("/mediaData/:instanceName", instanceAuthMiddleware, chatHandler.MediaData)
	}

	if groupHandler != nil {
		groups := app.Group("/group")
		groups.Post("/create/:instanceName", instanceAuthMiddleware, groupHandler.Create)
		groups.Put("/updateGroupPicture/:instanceName", instanceAuthMiddleware, groupHandler.UpdatePicture)
		groups.Get("/inviteCode/:instanceName", instanceAuthMiddleware, groupHandler.InviteCode)
		groups.Put("/revokeInviteCode/:instanceName", instanceAuthMiddleware, groupHandler.RevokeInviteCode)
		groups.Put("/updateParticipant/:instanceName", instanceAuthMiddleware, groupHandler.UpdateParticipant)
		groups.Delete("/leaveGroup/:instanceName", instanceAuthMiddleware, groupHandler.Leave)
	}
}
