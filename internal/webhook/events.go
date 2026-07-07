package webhook

import "whatsapp-go-api/internal/database/types"

type WebhookEvent = types.WebhookEvent
type WebhookEvents = types.WebhookEvents

const (
	WebhookEventQRCodeUpdated             = types.WebhookEventQRCodeUpdated
	WebhookEventHistorySync               = types.WebhookEventHistorySync
	WebhookEventMessagesUpsert            = types.WebhookEventMessagesUpsert
	WebhookEventMessagesUpdated           = types.WebhookEventMessagesUpdated
	WebhookEventMessagesDeleted           = types.WebhookEventMessagesDeleted
	WebhookEventMessagesStarred           = types.WebhookEventMessagesStarred
	WebhookEventMessagesUndecryptable     = types.WebhookEventMessagesUndecryptable
	WebhookEventSendMessage               = types.WebhookEventSendMessage
	WebhookEventContactsUpsert            = types.WebhookEventContactsUpsert
	WebhookEventContactsUpdated           = types.WebhookEventContactsUpdated
	WebhookEventChatsUpdated              = types.WebhookEventChatsUpdated
	WebhookEventChatsDeleted              = types.WebhookEventChatsDeleted
	WebhookEventPresenceUpdated           = types.WebhookEventPresenceUpdated
	WebhookEventGroupsUpsert              = types.WebhookEventGroupsUpsert
	WebhookEventGroupsUpdated             = types.WebhookEventGroupsUpdated
	WebhookEventGroupsParticipantsUpdated = types.WebhookEventGroupsParticipantsUpdated
	WebhookEventConnectionUpdated         = types.WebhookEventConnectionUpdated
	WebhookEventStatusInstance            = types.WebhookEventStatusInstance
	WebhookEventNewsletter                = types.WebhookEventNewsletter
	WebhookEventCallUpsert                = types.WebhookEventCallUpsert
	WebhookEventLabelsAssociation         = types.WebhookEventLabelsAssociation
	WebhookEventLabelsEdit                = types.WebhookEventLabelsEdit
	WebhookEventProfilePictureUpdated     = types.WebhookEventProfilePictureUpdated
	WebhookEventUserAboutUpdated          = types.WebhookEventUserAboutUpdated
	WebhookEventIdentityUpdated           = types.WebhookEventIdentityUpdated
	WebhookEventMediaRetry                = types.WebhookEventMediaRetry
	WebhookEventSettingsUpdated           = types.WebhookEventSettingsUpdated
)
