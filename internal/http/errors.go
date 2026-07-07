package http

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/chat"
	"whatsapp-go-api/internal/database/repository"
	"whatsapp-go-api/internal/group"
	"whatsapp-go-api/internal/http/response"
	"whatsapp-go-api/internal/http/validation"
	"whatsapp-go-api/internal/message"
	"whatsapp-go-api/internal/whatsapp"
	"whatsapp-go-api/internal/whatsapp/address"
)

func ErrorHandler(logger zerolog.Logger) fiber.ErrorHandler {
	return func(c fiber.Ctx, err error) error {
		status, code := mapError(err)
		if status >= fiber.StatusInternalServerError {
			logger.Error().Err(err).Str("path", c.Path()).Msg("request failed")
		} else {
			logger.Debug().Err(err).Str("path", c.Path()).Msg("request rejected")
		}

		return c.Status(status).JSON(response.ErrorResponse{
			StatusCode: status,
			Error:      code,
			Code:       publicErrorCode(err),
			Message:    errorMessages(err, status),
		})
	}
}

func errorMessages(err error, status int) []string {
	var chatValidation chat.ValidationError
	if errors.As(err, &chatValidation) && len(chatValidation.Messages) > 0 {
		return chatValidation.Messages
	}
	var requestValidation validation.ValidationError
	if errors.As(err, &requestValidation) && len(requestValidation.Messages) > 0 {
		return requestValidation.Messages
	}

	var deps *repository.InstanceDependenciesError
	if errors.As(err, &deps) {
		parts := make([]string, 0, 4)
		if deps.Chats > 0 {
			parts = append(parts, fmt.Sprintf("chats=%d", deps.Chats))
		}
		if deps.Messages > 0 {
			parts = append(parts, fmt.Sprintf("messages=%d", deps.Messages))
		}
		if deps.Webhooks > 0 {
			parts = append(parts, fmt.Sprintf("webhooks=%d", deps.Webhooks))
		}
		if deps.Contacts > 0 {
			parts = append(parts, fmt.Sprintf("contacts=%d", deps.Contacts))
		}
		return []string{
			"A instancia possui dados relacionados e nao pode ser removida sem force=true.",
			"Relacionamentos encontrados: " + strings.Join(parts, ", ") + ".",
		}
	}

	switch status {
	case fiber.StatusBadRequest:
		if errors.Is(err, message.ErrMentionAllRequiresGroup) {
			return []string{"A opcao mentionAll somente pode ser utilizada em grupos."}
		}
		if errors.Is(err, message.ErrMentionAllUnsupported) {
			return []string{"A opcao mentionAll nao e suportada para este tipo de mensagem."}
		}
		if errors.Is(err, repository.ErrInvalidWebhookEvent) {
			event := strings.TrimSpace(strings.TrimPrefix(err.Error(), repository.ErrInvalidWebhookEvent.Error()+":"))
			if event == "" {
				return []string{"unsupported webhook event"}
			}
			return []string{"unsupported webhook event: " + event}
		}
		if errors.Is(err, repository.ErrInvalidWebhookURL) {
			return []string{"url must be a valid URL"}
		}
		if errors.Is(err, repository.ErrInvalidOldToken) || errors.Is(err, repository.ErrTokenDoesNotMatch) {
			return []string{"old token is invalid"}
		}
		return []string{"Requisicao invalida."}
	case fiber.StatusUnauthorized:
		if errors.Is(err, repository.ErrInvalidOldToken) || errors.Is(err, repository.ErrTokenDoesNotMatch) {
			return []string{"old token is invalid"}
		}
		return []string{"Token ausente ou invalido."}
	case fiber.StatusForbidden:
		return []string{"Acesso negado."}
	case fiber.StatusNotFound:
		if code := passkeyErrorCode(err); code != "" {
			return []string{code}
		}
		if errors.Is(err, repository.ErrInstanceNotFound) {
			return []string{"instance not found"}
		}
		if errors.Is(err, repository.ErrAuthNotFound) {
			return []string{"authentication record not found"}
		}
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return []string{"webhook not found"}
		}
		if errors.Is(err, address.ErrRecipientNotOnWhatsApp) {
			return []string{"O numero informado nao esta registrado no WhatsApp."}
		}
		return []string{"Recurso nao encontrado."}
	case fiber.StatusNotAcceptable:
		if errors.Is(err, chat.ErrDatabaseOperation) {
			return []string{"Unable to retrieve the message from the database"}
		}
		return []string{"Mensagem enviada, mas nao foi possivel persistir no banco."}
	case fiber.StatusConflict:
		if errors.Is(err, repository.ErrWebhookAlreadyExists) {
			return []string{"webhook already exists"}
		}
		if errors.Is(err, address.ErrAmbiguousRecipient) {
			return []string{"O numero informado corresponde a mais de uma conta do WhatsApp."}
		}
		if code := passkeyErrorCode(err); code != "" {
			return []string{code}
		}
		return []string{"Conflito ao processar a requisicao."}
	case fiber.StatusGone:
		if code := passkeyErrorCode(err); code != "" {
			return []string{code}
		}
		return []string{"Recurso expirado."}
	case fiber.StatusUnprocessableEntity:
		if code := passkeyErrorCode(err); code != "" {
			return []string{code}
		}
		return []string{"Requisicao semanticamente invalida."}
	case fiber.StatusRequestEntityTooLarge:
		return []string{"Arquivo acima do limite permitido."}
	case fiber.StatusUnsupportedMediaType:
		return []string{"Tipo de midia nao suportado."}
	case fiber.StatusServiceUnavailable:
		if errors.Is(err, message.ErrMessageQueueFull) {
			return []string{"O servico de processamento de mensagens esta temporariamente ocupado."}
		}
		if errors.Is(err, message.ErrMessageProcessorStopped) {
			return []string{"O servico de processamento de mensagens nao esta disponivel."}
		}
		if code := passkeyErrorCode(err); code != "" {
			return []string{code}
		}
		return []string{"Servico indisponivel."}
	default:
		return []string{"Erro interno do servidor."}
	}
}

func mapError(err error) (int, string) {
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return statusCodeError(fiberErr.Code)
	}

	switch {
	case errors.Is(err, repository.ErrInstanceNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, repository.ErrAuthNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, repository.ErrWebhookNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, repository.ErrMessageNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, whatsapp.ErrPasskeyInstanceNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, whatsapp.ErrPairingSessionNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, chat.ErrMediaMessageNotFound):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, address.ErrRecipientNotOnWhatsApp):
		return fiber.StatusNotFound, response.ErrorNotFound
	case errors.Is(err, repository.ErrInstanceNameAlreadyExists):
		return fiber.StatusConflict, response.ErrorConflict
	case errors.Is(err, repository.ErrWebhookAlreadyExists):
		return fiber.StatusConflict, response.ErrorConflict
	case errors.Is(err, message.ErrPersistenceFailed),
		errors.Is(err, message.ErrQuotedMessageLookup),
		errors.Is(err, chat.ErrDatabaseOperation):
		return fiber.StatusNotAcceptable, response.ErrorNotAcceptable
	case errors.Is(err, whatsapp.ErrConnectionInProgress),
		errors.Is(err, whatsapp.ErrInstanceConnected),
		errors.Is(err, whatsapp.ErrPairingSessionNotActive),
		errors.Is(err, whatsapp.ErrInvalidPairingState),
		errors.Is(err, whatsapp.ErrPasskeyRequestMismatch),
		errors.Is(err, whatsapp.ErrPasskeyChallengeAlreadyUsed),
		errors.Is(err, address.ErrAmbiguousRecipient):
		return fiber.StatusConflict, response.ErrorConflict
	case errors.Is(err, whatsapp.ErrPasskeyChallengeExpired):
		return fiber.StatusGone, response.ErrorGone
	case errors.Is(err, whatsapp.ErrInvalidPasskeyAssertion),
		errors.Is(err, whatsapp.ErrPasskeyNotAvailable):
		return fiber.StatusUnprocessableEntity, response.ErrorUnprocessableEntity
	case errors.Is(err, whatsapp.ErrQRCodeTimeout):
		return fiber.StatusRequestTimeout, response.ErrorRequestTimeout
	case errors.Is(err, whatsapp.ErrQRChannelClosed),
		errors.Is(err, whatsapp.ErrPairingFailed),
		errors.Is(err, whatsapp.ErrClientOutdated):
		return fiber.StatusServiceUnavailable, response.ErrorServiceUnavailable
	case errors.Is(err, repository.ErrInvalidOldToken),
		errors.Is(err, repository.ErrTokenDoesNotMatch),
		errors.Is(err, authjwt.ErrJWTInvalid),
		errors.Is(err, whatsapp.ErrInvalidInstanceToken):
		return fiber.StatusUnauthorized, response.ErrorUnauthorized
	case errors.Is(err, whatsapp.ErrInstanceInactive),
		errors.Is(err, chat.ErrMessageNotOutgoing),
		errors.Is(err, repository.ErrTokenInstanceMismatch):
		return fiber.StatusForbidden, response.ErrorForbidden
	case errors.Is(err, repository.ErrInvalidInput),
		errors.Is(err, repository.ErrInvalidJSON),
		errors.Is(err, repository.ErrInvalidWebhookEvent),
		errors.Is(err, repository.ErrInvalidWebhookURL),
		errors.Is(err, whatsapp.ErrInvalidPhoneNumber):
		return fiber.StatusBadRequest, response.ErrorBadRequest
	case errors.As(err, new(validation.ValidationError)):
		return fiber.StatusBadRequest, response.ErrorBadRequest
	case errors.Is(err, message.ErrInvalidRequest),
		errors.Is(err, message.ErrMentionAllRequiresGroup),
		errors.Is(err, message.ErrMentionAllUnsupported),
		errors.Is(err, address.ErrInvalidAddress),
		errors.Is(err, message.ErrRecipientInvalid),
		errors.Is(err, message.ErrPresenceInvalid),
		errors.Is(err, message.ErrDelayInvalid),
		errors.Is(err, message.ErrQuotedMessageInvalid),
		errors.Is(err, message.ErrInvalidAudioDuration),
		errors.Is(err, chat.ErrInvalidRecipient),
		errors.Is(err, chat.ErrInvalidRequestMode),
		errors.Is(err, chat.ErrInvalidMediaRequest),
		errors.Is(err, chat.ErrUnsupportedMediaType),
		errors.Is(err, chat.ErrInvalidMediaContent),
		errors.Is(err, chat.ErrMessageIsNotMedia),
		errors.Is(err, group.ErrInvalidGroupJID),
		errors.Is(err, group.ErrInvalidParticipant),
		errors.Is(err, group.ErrInvalidRequest):
		return fiber.StatusBadRequest, response.ErrorBadRequest
	case errors.Is(err, chat.ErrMessageNotEditable):
		return fiber.StatusUnprocessableEntity, response.ErrorUnprocessableEntity
	case errors.Is(err, message.ErrPayloadTooLarge),
		errors.Is(err, chat.ErrMediaTooLarge):
		return fiber.StatusRequestEntityTooLarge, response.ErrorPayloadTooLarge
	case errors.Is(err, message.ErrUnsupportedMediaType):
		return fiber.StatusUnsupportedMediaType, response.ErrorUnsupportedMedia
	case errors.Is(err, whatsapp.ErrWhatsAppUnavailable),
		errors.Is(err, message.ErrMessageQueueFull),
		errors.Is(err, message.ErrMessageProcessorStopped),
		errors.Is(err, whatsapp.ErrClientNotConnected),
		errors.Is(err, whatsapp.ErrPasskeyServiceUnavailable),
		errors.Is(err, chat.ErrInstanceDisconnected),
		errors.Is(err, chat.ErrRemoteOperation),
		errors.Is(err, chat.ErrMediaDownloadFailed),
		errors.Is(err, group.ErrRemoteOperation),
		errors.Is(err, group.ErrDownloadFailed),
		errors.Is(err, whatsapp.ErrSessionMissing),
		errors.Is(err, whatsapp.ErrDeviceMismatch):
		return fiber.StatusServiceUnavailable, response.ErrorServiceUnavailable
	case errors.Is(err, message.ErrDownloadFailed),
		errors.Is(err, message.ErrUploadFailed),
		errors.Is(err, message.ErrSendFailed),
		errors.Is(err, message.ErrAudioProcessing),
		errors.Is(err, authjwt.ErrJWTGeneration):
		return fiber.StatusInternalServerError, response.ErrorInternalServer
	case errors.Is(err, group.ErrInstanceDisconnected):
		return fiber.StatusServiceUnavailable, response.ErrorServiceUnavailable
	case errors.Is(err, group.ErrImageTooLarge):
		return fiber.StatusRequestEntityTooLarge, response.ErrorPayloadTooLarge
	default:
		return fiber.StatusInternalServerError, response.ErrorInternalServer
	}
}

func publicErrorCode(err error) string {
	switch {
	case errors.Is(err, message.ErrMentionAllRequiresGroup):
		return "MENTION_ALL_REQUIRES_GROUP"
	case errors.Is(err, message.ErrMentionAllUnsupported):
		return "MENTION_ALL_NOT_SUPPORTED_FOR_MESSAGE_TYPE"
	case errors.Is(err, message.ErrMessageQueueFull):
		return "MESSAGE_PROCESSING_QUEUE_FULL"
	case errors.Is(err, message.ErrMessageProcessorStopped):
		return "MESSAGE_PROCESSOR_STOPPED"
	default:
		return ""
	}
}

func passkeyErrorCode(err error) string {
	switch {
	case errors.Is(err, whatsapp.ErrPasskeyInstanceNotFound):
		return "INSTANCE_NOT_FOUND"
	case errors.Is(err, whatsapp.ErrPairingSessionNotFound):
		return "PAIRING_SESSION_NOT_FOUND"
	case errors.Is(err, whatsapp.ErrPairingSessionNotActive):
		return "PAIRING_SESSION_NOT_ACTIVE"
	case errors.Is(err, whatsapp.ErrInvalidPairingState):
		return "INVALID_PAIRING_STATE"
	case errors.Is(err, whatsapp.ErrPasskeyRequestMismatch):
		return "PASSKEY_REQUEST_MISMATCH"
	case errors.Is(err, whatsapp.ErrPasskeyChallengeAlreadyUsed):
		return "PASSKEY_CHALLENGE_ALREADY_USED"
	case errors.Is(err, whatsapp.ErrInstanceConnected):
		return "INSTANCE_ALREADY_CONNECTED"
	case errors.Is(err, whatsapp.ErrPasskeyChallengeExpired):
		return "PASSKEY_CHALLENGE_EXPIRED"
	case errors.Is(err, whatsapp.ErrInvalidPasskeyAssertion):
		return "INVALID_PASSKEY_ASSERTION"
	case errors.Is(err, whatsapp.ErrPasskeyNotAvailable):
		return "PASSKEY_NOT_AVAILABLE"
	case errors.Is(err, whatsapp.ErrClientNotConnected):
		return "WHATSAPP_CLIENT_NOT_CONNECTED"
	case errors.Is(err, whatsapp.ErrPasskeyServiceUnavailable):
		return "PASSKEY_SERVICE_UNAVAILABLE"
	default:
		return ""
	}
}

func statusCodeError(status int) (int, string) {
	switch status {
	case fiber.StatusBadRequest:
		return status, response.ErrorBadRequest
	case fiber.StatusUnauthorized:
		return status, response.ErrorUnauthorized
	case fiber.StatusForbidden:
		return status, response.ErrorForbidden
	case fiber.StatusNotFound:
		return status, response.ErrorNotFound
	case fiber.StatusNotAcceptable:
		return status, response.ErrorNotAcceptable
	case fiber.StatusConflict:
		return status, response.ErrorConflict
	case fiber.StatusRequestTimeout:
		return status, response.ErrorRequestTimeout
	case fiber.StatusGone:
		return status, response.ErrorGone
	case fiber.StatusRequestEntityTooLarge:
		return status, response.ErrorPayloadTooLarge
	case fiber.StatusUnsupportedMediaType:
		return status, response.ErrorUnsupportedMedia
	case fiber.StatusUnprocessableEntity:
		return status, response.ErrorUnprocessableEntity
	case fiber.StatusTooManyRequests:
		return status, response.ErrorTooManyRequests
	case fiber.StatusServiceUnavailable:
		return status, response.ErrorServiceUnavailable
	default:
		return fiber.StatusInternalServerError, response.ErrorInternalServer
	}
}
