package types

import (
	"encoding/json"
	"fmt"
	"time"
)

type WebhookEvent string

const (
	WebhookEventQRCodeUpdated             WebhookEvent = "qrcode.updated"
	WebhookEventHistorySync               WebhookEvent = "history.sync"
	WebhookEventMessagesUpsert            WebhookEvent = "messages.upsert"
	WebhookEventMessagesUpdated           WebhookEvent = "messages.update"
	WebhookEventMessagesDeleted           WebhookEvent = "messages.delete"
	WebhookEventMessagesStarred           WebhookEvent = "messages.star"
	WebhookEventMessagesUndecryptable     WebhookEvent = "messages.undecryptable"
	WebhookEventSendMessage               WebhookEvent = "send.message"
	WebhookEventContactsUpsert            WebhookEvent = "contacts.upsert"
	WebhookEventContactsUpdated           WebhookEvent = "contacts.update"
	WebhookEventChatsUpdated              WebhookEvent = "chats.updated"
	WebhookEventChatsDeleted              WebhookEvent = "chats.delete"
	WebhookEventPresenceUpdated           WebhookEvent = "presence.updated"
	WebhookEventGroupsUpsert              WebhookEvent = "groups.upsert"
	WebhookEventGroupsUpdated             WebhookEvent = "groups.update"
	WebhookEventGroupsParticipantsUpdated WebhookEvent = "groups.participants.update"
	WebhookEventConnectionUpdated         WebhookEvent = "connection.update"
	WebhookEventStatusInstance            WebhookEvent = "status.instance"
	WebhookEventNewsletter                WebhookEvent = "news.letter"
	WebhookEventCallUpsert                WebhookEvent = "call.upsert"
	WebhookEventLabelsAssociation         WebhookEvent = "labels.association"
	WebhookEventLabelsEdit                WebhookEvent = "labels.edit"
	WebhookEventProfilePictureUpdated     WebhookEvent = "profile.picture.update"
	WebhookEventUserAboutUpdated          WebhookEvent = "user.about.update"
	WebhookEventIdentityUpdated           WebhookEvent = "identity.update"
	WebhookEventMediaRetry                WebhookEvent = "media.retry"
	WebhookEventSettingsUpdated           WebhookEvent = "settings.update"
)

type WebhookEvents struct {
	QRCodeUpdated             bool `json:"qrcodeUpdated"`
	HistorySync               bool `json:"historySync"`
	MessagesUpsert            bool `json:"messagesUpsert"`
	MessagesUpdated           bool `json:"messagesUpdated"`
	MessagesDeleted           bool `json:"messagesDeleted"`
	MessagesStarred           bool `json:"messagesStarred"`
	MessagesUndecryptable     bool `json:"messagesUndecryptable"`
	SendMessage               bool `json:"sendMessage"`
	ContactsUpsert            bool `json:"contactsUpsert"`
	ContactsUpdated           bool `json:"contactsUpdated"`
	ChatsUpdated              bool `json:"chatsUpdated"`
	ChatsDeleted              bool `json:"chatsDeleted"`
	PresenceUpdated           bool `json:"presenceUpdated"`
	GroupsUpsert              bool `json:"groupsUpsert"`
	GroupsUpdated             bool `json:"groupsUpdated"`
	GroupsParticipantsUpdated bool `json:"groupsParticipantsUpdated"`
	ConnectionUpdated         bool `json:"connectionUpdated"`
	StatusInstance            bool `json:"statusInstance"`
	Newsletter                bool `json:"newsLetter"`
	CallUpsert                bool `json:"callUpsert"`
	LabelsAssociation         bool `json:"labelsAssociation"`
	LabelsEdit                bool `json:"labelsEdit"`
	ProfilePictureUpdated     bool `json:"profilePictureUpdated"`
	UserAboutUpdated          bool `json:"userAboutUpdated"`
	IdentityUpdated           bool `json:"identityUpdated"`
	MediaRetry                bool `json:"mediaRetry"`
	SettingsUpdated           bool `json:"settingsUpdated"`
}

func (e WebhookEvents) IsEnabled(event WebhookEvent) bool {
	switch event {
	case WebhookEventQRCodeUpdated:
		return e.QRCodeUpdated
	case WebhookEventHistorySync:
		return e.HistorySync
	case WebhookEventMessagesUpsert:
		return e.MessagesUpsert
	case WebhookEventMessagesUpdated:
		return e.MessagesUpdated
	case WebhookEventMessagesDeleted:
		return e.MessagesDeleted
	case WebhookEventMessagesStarred:
		return e.MessagesStarred
	case WebhookEventMessagesUndecryptable:
		return e.MessagesUndecryptable
	case WebhookEventSendMessage:
		return e.SendMessage
	case WebhookEventContactsUpsert:
		return e.ContactsUpsert
	case WebhookEventContactsUpdated:
		return e.ContactsUpdated
	case WebhookEventChatsUpdated:
		return e.ChatsUpdated
	case WebhookEventChatsDeleted:
		return e.ChatsDeleted
	case WebhookEventPresenceUpdated:
		return e.PresenceUpdated
	case WebhookEventGroupsUpsert:
		return e.GroupsUpsert
	case WebhookEventGroupsUpdated:
		return e.GroupsUpdated
	case WebhookEventGroupsParticipantsUpdated:
		return e.GroupsParticipantsUpdated
	case WebhookEventConnectionUpdated:
		return e.ConnectionUpdated
	case WebhookEventStatusInstance:
		return e.StatusInstance
	case WebhookEventNewsletter:
		return e.Newsletter
	case WebhookEventCallUpsert:
		return e.CallUpsert
	case WebhookEventLabelsAssociation:
		return e.LabelsAssociation
	case WebhookEventLabelsEdit:
		return e.LabelsEdit
	case WebhookEventProfilePictureUpdated:
		return e.ProfilePictureUpdated
	case WebhookEventUserAboutUpdated:
		return e.UserAboutUpdated
	case WebhookEventIdentityUpdated:
		return e.IdentityUpdated
	case WebhookEventMediaRetry:
		return e.MediaRetry
	case WebhookEventSettingsUpdated:
		return e.SettingsUpdated
	default:
		return false
	}
}

func (event WebhookEvent) IsSupported() bool {
	_, ok := webhookEventsByName[event]
	return ok
}

func ParseWebhookEvents(raw json.RawMessage) (WebhookEvents, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return WebhookEvents{}, nil
	}
	var events WebhookEvents
	if err := json.Unmarshal(raw, &events); err != nil {
		return WebhookEvents{}, err
	}
	return events, nil
}

func ValidateWebhookEventFields(events map[string]bool) error {
	for event := range events {
		if !IsWebhookEventField(event) {
			return fmt.Errorf("invalid webhook event: %s", event)
		}
	}
	return nil
}

func IsWebhookEventField(field string) bool {
	_, ok := webhookFieldsByName[field]
	return ok
}

func WebhookEventFields() map[string]WebhookEvent {
	fields := make(map[string]WebhookEvent, len(webhookFieldsByName))
	for field, event := range webhookFieldsByName {
		fields[field] = event
	}
	return fields
}

func SupportedWebhookEvents() []WebhookEvent {
	events := make([]WebhookEvent, 0, len(webhookEventsByName))
	for event := range webhookEventsByName {
		events = append(events, event)
	}
	return events
}

var webhookEventsByName = map[WebhookEvent]struct{}{
	WebhookEventQRCodeUpdated:             {},
	WebhookEventHistorySync:               {},
	WebhookEventMessagesUpsert:            {},
	WebhookEventMessagesUpdated:           {},
	WebhookEventMessagesDeleted:           {},
	WebhookEventMessagesStarred:           {},
	WebhookEventMessagesUndecryptable:     {},
	WebhookEventSendMessage:               {},
	WebhookEventContactsUpsert:            {},
	WebhookEventContactsUpdated:           {},
	WebhookEventChatsUpdated:              {},
	WebhookEventChatsDeleted:              {},
	WebhookEventPresenceUpdated:           {},
	WebhookEventGroupsUpsert:              {},
	WebhookEventGroupsUpdated:             {},
	WebhookEventGroupsParticipantsUpdated: {},
	WebhookEventConnectionUpdated:         {},
	WebhookEventStatusInstance:            {},
	WebhookEventNewsletter:                {},
	WebhookEventCallUpsert:                {},
	WebhookEventLabelsAssociation:         {},
	WebhookEventLabelsEdit:                {},
	WebhookEventProfilePictureUpdated:     {},
	WebhookEventUserAboutUpdated:          {},
	WebhookEventIdentityUpdated:           {},
	WebhookEventMediaRetry:                {},
	WebhookEventSettingsUpdated:           {},
}

var webhookFieldsByName = map[string]WebhookEvent{
	"qrcodeUpdated":             WebhookEventQRCodeUpdated,
	"historySync":               WebhookEventHistorySync,
	"messagesUpsert":            WebhookEventMessagesUpsert,
	"messagesUpdated":           WebhookEventMessagesUpdated,
	"messagesDeleted":           WebhookEventMessagesDeleted,
	"messagesStarred":           WebhookEventMessagesStarred,
	"messagesUndecryptable":     WebhookEventMessagesUndecryptable,
	"sendMessage":               WebhookEventSendMessage,
	"contactsUpsert":            WebhookEventContactsUpsert,
	"contactsUpdated":           WebhookEventContactsUpdated,
	"chatsUpdated":              WebhookEventChatsUpdated,
	"chatsDeleted":              WebhookEventChatsDeleted,
	"presenceUpdated":           WebhookEventPresenceUpdated,
	"groupsUpsert":              WebhookEventGroupsUpsert,
	"groupsUpdated":             WebhookEventGroupsUpdated,
	"groupsParticipantsUpdated": WebhookEventGroupsParticipantsUpdated,
	"connectionUpdated":         WebhookEventConnectionUpdated,
	"statusInstance":            WebhookEventStatusInstance,
	"newsLetter":                WebhookEventNewsletter,
	"callUpsert":                WebhookEventCallUpsert,
	"labelsAssociation":         WebhookEventLabelsAssociation,
	"labelsEdit":                WebhookEventLabelsEdit,
	"profilePictureUpdated":     WebhookEventProfilePictureUpdated,
	"userAboutUpdated":          WebhookEventUserAboutUpdated,
	"identityUpdated":           WebhookEventIdentityUpdated,
	"mediaRetry":                WebhookEventMediaRetry,
	"settingsUpdated":           WebhookEventSettingsUpdated,
}

type Webhook struct {
	ID         int32           `json:"id"`
	URL        string          `json:"url"`
	Enabled    bool            `json:"enabled"`
	Events     json.RawMessage `json:"events"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	InstanceID int32           `json:"instanceId"`
}

type CreateWebhookInput struct {
	URL        string
	Enabled    *bool
	Events     json.RawMessage
	InstanceID int32
}

type UpdateWebhookInput struct {
	URL     *string
	Enabled *bool
}

type WebhookWithInstance struct {
	Webhook      Webhook
	InstanceName string
}
