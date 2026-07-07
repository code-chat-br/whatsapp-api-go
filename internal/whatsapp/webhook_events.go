package whatsapp

import (
	"fmt"
	"time"

	"go.mau.fi/whatsmeow/types/events"

	webhooksvc "whatsapp-go-api/internal/webhook"
)

func chatUpdatedWebhookData(event any, processingTime time.Time) (map[string]any, error) {
	switch value := event.(type) {
	case *events.Blocklist:
		return normalizedFlattenedWebhookData("blocklist", value, processingTime)
	case *events.BlocklistChange:
		return normalizedFlattenedWebhookData("blocklist.change", value, processingTime)
	case *events.Archive:
		return normalizedFlattenedWebhookData("archive", value, eventDateTime(value.Timestamp, processingTime))
	case *events.UnarchiveChatsSetting:
		return normalizedFlattenedWebhookData("unarchive.setting", value, eventDateTime(value.Timestamp, processingTime))
	case *events.ClearChat:
		return normalizedFlattenedWebhookData("clear", value, eventDateTime(value.Timestamp, processingTime))
	case *events.Pin:
		return normalizedFlattenedWebhookData("pin", value, eventDateTime(value.Timestamp, processingTime))
	case *events.Mute:
		return normalizedFlattenedWebhookData("mute", value, eventDateTime(value.Timestamp, processingTime))
	case *events.MarkChatAsRead:
		return normalizedFlattenedWebhookData("mark.read", value, eventDateTime(value.Timestamp, processingTime))
	case *events.UserStatusMute:
		return normalizedFlattenedWebhookData("user.status.mute", value, eventDateTime(value.Timestamp, processingTime))
	default:
		return map[string]any{}, nil
	}
}

func chatDeletedWebhookData(event *events.DeleteChat, processingTime time.Time) (map[string]any, error) {
	return normalizedFlattenedWebhookData("", event, eventDateTime(event.Timestamp, processingTime))
}

func presenceUpdatedWebhookData(event any, processingTime time.Time) (map[string]any, error) {
	switch value := event.(type) {
	case *events.ChatPresence:
		source, err := webhooksvc.NewEventNormalizer().ToJSONMap(value)
		if err != nil {
			return nil, err
		}
		normalizeMessageSourceKeys(source)
		return webhooksvc.MergeEventData("", source, processingTime), nil
	case *events.Presence:
		output := map[string]any{
			"type":        "presence",
			"jid":         jidString(value.From),
			"unavailable": value.Unavailable,
			"dateTime":    processingTime.UTC(),
		}
		if !value.LastSeen.IsZero() {
			output["lastSeen"] = value.LastSeen.UTC()
		}
		return output, nil
	default:
		return map[string]any{}, nil
	}
}

func historySyncWebhookData(event *events.HistorySync, processingTime time.Time) (map[string]any, error) {
	source, err := webhooksvc.NewEventNormalizer().ToJSONMap(event)
	if err != nil {
		return nil, err
	}
	return webhooksvc.MergeEventData("history.sync", source, processingTime), nil
}

func profilePictureUpdatedWebhookData(event *events.Picture, processingTime time.Time) webhooksvc.ProfilePictureUpdatedWebhookData {
	dateTime := eventDateTime(event.Timestamp, processingTime)
	return webhooksvc.ProfilePictureUpdatedWebhookData{
		JID:       jidString(event.JID),
		Author:    jidString(event.Author),
		DateTime:  dateTime,
		Remove:    event.Remove,
		PictureID: event.PictureID,
		IsGroup:   event.JID.Server == "g.us",
	}
}

func userAboutUpdatedWebhookData(event *events.UserAbout, processingTime time.Time) webhooksvc.UserAboutUpdatedWebhookData {
	return webhooksvc.UserAboutUpdatedWebhookData{
		JID:      jidString(event.JID),
		Status:   event.Status,
		DateTime: eventDateTime(event.Timestamp, processingTime),
	}
}

func identityUpdatedWebhookData(event *events.IdentityChange, processingTime time.Time) webhooksvc.IdentityUpdatedWebhookData {
	return webhooksvc.IdentityUpdatedWebhookData{
		JID:      jidString(event.JID),
		DateTime: eventDateTime(event.Timestamp, processingTime),
		Implicit: event.Implicit,
	}
}

func mediaRetryWebhookData(event *events.MediaRetry, processingTime time.Time) webhooksvc.MediaRetryWebhookData {
	var errorCode *int
	if event.Error != nil {
		errorCode = intPtr(event.Error.Code)
	}
	return webhooksvc.MediaRetryWebhookData{
		MessageID:     string(event.MessageID),
		ChatJID:       jidString(event.ChatID),
		SenderJID:     stringPtrFromJID(event.SenderID),
		KeyFromMe:     event.FromMe,
		HasCiphertext: len(event.Ciphertext) > 0,
		ErrorCode:     errorCode,
		DateTime:      eventDateTime(event.Timestamp, processingTime),
	}
}

func messageDeletedWebhookData(event *events.DeleteForMe, processingTime time.Time) webhooksvc.MessageDeletedWebhookData {
	var messageTime *time.Time
	if event.Action != nil {
		if timestamp := event.Action.GetMessageTimestamp(); timestamp > 0 {
			value := time.Unix(timestamp, 0).UTC()
			messageTime = &value
		}
	}
	return webhooksvc.MessageDeletedWebhookData{
		ChatJID:      jidString(event.ChatJID),
		SenderJID:    stringPtrFromJID(event.SenderJID),
		KeyFromMe:    event.IsFromMe,
		MessageID:    event.MessageID,
		DeleteMedia:  event.Action != nil && event.Action.GetDeleteMedia(),
		FromFullSync: event.FromFullSync,
		DateTime:     eventDateTime(event.Timestamp, processingTime),
		MessageTime:  messageTime,
	}
}

func messageStarredWebhookData(event *events.Star, processingTime time.Time) webhooksvc.MessageStarredWebhookData {
	return webhooksvc.MessageStarredWebhookData{
		ChatJID:      jidString(event.ChatJID),
		SenderJID:    stringPtrFromJID(event.SenderJID),
		KeyFromMe:    event.IsFromMe,
		MessageID:    event.MessageID,
		Starred:      event.Action != nil && event.Action.GetStarred(),
		FromFullSync: event.FromFullSync,
		DateTime:     eventDateTime(event.Timestamp, processingTime),
	}
}

func messageUndecryptableWebhookData(event *events.UndecryptableMessage, processingTime time.Time) webhooksvc.MessageUndecryptableWebhookData {
	dateTime := eventDateTime(event.Info.Timestamp, processingTime)
	return webhooksvc.MessageUndecryptableWebhookData{
		KeyID:           string(event.Info.ID),
		ChatJID:         jidString(event.Info.Chat),
		SenderJID:       stringPtrFromJID(event.Info.Sender),
		KeyFromMe:       event.Info.IsFromMe,
		IsUnavailable:   event.IsUnavailable,
		UnavailableType: string(event.UnavailableType),
		DecryptFailMode: string(event.DecryptFailMode),
		DateTime:        dateTime,
	}
}

func settingsUpdatedWebhookData(event any, processingTime time.Time) (webhooksvc.SettingsUpdatedWebhookData, error) {
	switch value := event.(type) {
	case *events.PushNameSetting:
		var name *string
		if value.Action != nil {
			name = stringPtr(value.Action.GetName())
		}
		return webhooksvc.SettingsUpdatedWebhookData{
			Type:         "push.name",
			Name:         name,
			FromFullSync: value.FromFullSync,
			DateTime:     eventDateTime(value.Timestamp, processingTime),
		}, nil
	case *events.UserStatusMute:
		var muted *bool
		if value.Action != nil {
			muted = boolPtr(value.Action.GetMuted())
		}
		return webhooksvc.SettingsUpdatedWebhookData{
			Type:         "user.status.mute",
			JID:          stringPtrFromJID(value.JID),
			Muted:        muted,
			FromFullSync: value.FromFullSync,
			DateTime:     eventDateTime(value.Timestamp, processingTime),
		}, nil
	default:
		return webhooksvc.SettingsUpdatedWebhookData{}, fmt.Errorf("unsupported settings event %T", event)
	}
}

func normalizedFlattenedWebhookData(eventType string, value any, dateTime time.Time) (map[string]any, error) {
	source, err := webhooksvc.NewEventNormalizer().ToJSONMap(value)
	if err != nil {
		return nil, err
	}
	normalizeChatEventKeys(source)
	flattenAction(source)
	delete(source, "timestamp")
	return webhooksvc.MergeEventData(eventType, source, dateTime), nil
}

func flattenAction(source map[string]any) {
	action, ok := source["action"].(map[string]any)
	if !ok {
		return
	}
	for key, value := range action {
		source[key] = value
	}
	delete(source, "action")
}

func normalizeChatEventKeys(source map[string]any) {
	renameMapKey(source, "jid", "chatJid")
}

func normalizeMessageSourceKeys(source map[string]any) {
	renameMapKey(source, "chat", "chatJid")
	renameMapKey(source, "sender", "senderJid")
	renameMapKey(source, "senderAlt", "senderAltJid")
	renameMapKey(source, "recipientAlt", "recipientAltJid")
	renameMapKey(source, "broadcastListOwner", "broadcastListOwnerJid")
}

func renameMapKey(source map[string]any, oldKey string, newKey string) {
	value, ok := source[oldKey]
	if !ok {
		return
	}
	if _, exists := source[newKey]; !exists {
		source[newKey] = value
	}
	delete(source, oldKey)
}

func eventDateTime(value time.Time, fallback time.Time) time.Time {
	if !value.IsZero() {
		return value.UTC()
	}
	if !fallback.IsZero() {
		return fallback.UTC()
	}
	return time.Now().UTC()
}
