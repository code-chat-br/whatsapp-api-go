package message

import "whatsapp-go-api/internal/http/request"

type RecipientInput struct {
	Number    *string
	Chat      *string
	Recipient *string
}

type MessageOptions = request.MessageOptions
type SendTextRequest = request.SendTextRequest
type TextMessage = request.TextMessage
type SendLinkRequest = request.SendLinkRequest
type LinkMessage = request.LinkMessage
type SendMediaRequest = request.SendMediaRequest
type MediaMessage = request.MediaMessage
type SendWhatsAppAudioRequest = request.SendWhatsAppAudioRequest
type AudioMessageRequest = request.AudioMessageRequest
type SendContactRequest = request.SendContactRequest
type ContactMessage = request.ContactMessage
type SendLocationRequest = request.SendLocationRequest
type LocationMessage = request.LocationMessage
type SendReactionRequest = request.SendReactionRequest
type ReactionMessage = request.ReactionMessage
type ReactionKey = request.ReactionKey

func recipientInput(number *string, chat *string, recipient *string) RecipientInput {
	return RecipientInput{Number: number, Chat: chat, Recipient: recipient}
}
