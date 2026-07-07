package whatsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	armadillo "go.mau.fi/whatsmeow/proto"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	waevents "go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	dbtypes "whatsapp-go-api/internal/database/types"
)

var ErrUnsupportedMessageContent = errors.New("unsupported message content")

type MessageEventNormalizer struct {
	marshal protojson.MarshalOptions
}

type normalizedContent struct {
	MessageType string
	Content     json.RawMessage
}

func NewMessageEventNormalizer() MessageEventNormalizer {
	return MessageEventNormalizer{
		marshal: protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false},
	}
}

func (n MessageEventNormalizer) NormalizeMessage(instanceID int32, event *waevents.Message) (dbtypes.CreateMessageInput, error) {
	if event == nil {
		return dbtypes.CreateMessageInput{}, fmt.Errorf("%w: nil message event", ErrUnsupportedMessageContent)
	}
	content, err := n.NormalizeMessageContent(event.Message)
	if err != nil {
		return dbtypes.CreateMessageInput{}, err
	}
	message := n.normalizeFromInfo(instanceID, event.Info, content, n.messageMetadata(event))
	message.MessageTimestamp = messageTimestampFromMessage(event.Info, event.Message)
	return message, nil
}

func (n MessageEventNormalizer) NormalizeFBMessage(instanceID int32, event *waevents.FBMessage) (dbtypes.CreateMessageInput, error) {
	if event == nil {
		return dbtypes.CreateMessageInput{}, fmt.Errorf("%w: nil fb message event", ErrUnsupportedMessageContent)
	}
	content, err := n.normalizeFBContent(event.Message)
	if err != nil {
		return dbtypes.CreateMessageInput{}, err
	}
	return n.normalizeFromInfo(instanceID, event.Info, content, n.fbMessageMetadata(event)), nil
}

func (n MessageEventNormalizer) normalizeFromInfo(instanceID int32, info watypes.MessageInfo, content normalizedContent, metadata json.RawMessage) dbtypes.CreateMessageInput {
	isGroup := info.IsGroup
	return dbtypes.CreateMessageInput{
		KeyID:             string(info.ID),
		KeyRemoteJid:      stringPtrFromJID(messageRemoteJID(info)),
		KeyLid:            stringPtrFromJID(messageLID(info)),
		KeyFromMe:         info.IsFromMe,
		KeyParticipant:    stringPtrFromJID(messageParticipant(info)),
		KeyParticipantLid: stringPtrFromJID(messageParticipantLID(info)),
		PushName:          stringPtr(info.PushName),
		MessageType:       content.MessageType,
		Content:           content.Content,
		MessageTimestamp:  messageTimestampFromInfo(info),
		Device:            dbtypes.DeviceMessageUnknown,
		IsGroup:           &isGroup,
		InstanceID:        instanceID,
		Metadata:          metadata,
	}
}

func (n MessageEventNormalizer) NormalizeMessageContent(message *wae2e.Message) (normalizedContent, error) {
	if message == nil {
		return normalizedContent{}, fmt.Errorf("%w: nil protobuf message", ErrUnsupportedMessageContent)
	}
	if text := strings.TrimSpace(message.GetConversation()); text != "" {
		content, err := json.Marshal(map[string]string{"text": message.GetConversation()})
		return normalizedContent{MessageType: "extendedTextMessage", Content: content}, err
	}

	if payload := message.GetExtendedTextMessage(); payload != nil {
		return n.protoContent("extendedTextMessage", payload)
	}
	if payload := message.GetImageMessage(); payload != nil {
		return n.protoContent("imageMessage", payload)
	}
	if payload := message.GetVideoMessage(); payload != nil {
		return n.protoContent("videoMessage", payload)
	}
	if payload := message.GetAudioMessage(); payload != nil {
		return n.protoContent("audioMessage", payload)
	}
	if payload := message.GetDocumentMessage(); payload != nil {
		return n.protoContent("documentMessage", payload)
	}
	if payload := message.GetStickerMessage(); payload != nil {
		return n.protoContent("stickerMessage", payload)
	}
	if payload := message.GetContactMessage(); payload != nil {
		return n.protoContent("contactMessage", payload)
	}
	if payload := message.GetContactsArrayMessage(); payload != nil {
		return n.protoContent("contactsArrayMessage", payload)
	}
	if payload := message.GetLocationMessage(); payload != nil {
		return n.protoContent("locationMessage", payload)
	}
	if payload := message.GetLiveLocationMessage(); payload != nil {
		return n.protoContent("liveLocationMessage", payload)
	}
	if payload := message.GetReactionMessage(); payload != nil {
		return n.protoContent("reactionMessage", payload)
	}
	if payload := message.GetProtocolMessage(); payload != nil {
		return n.protoContent("protocolMessage", payload)
	}
	if payload := message.GetButtonsMessage(); payload != nil {
		return n.protoContent("buttonsMessage", payload)
	}
	if payload := message.GetButtonsResponseMessage(); payload != nil {
		return n.protoContent("buttonsResponseMessage", payload)
	}
	if payload := message.GetListMessage(); payload != nil {
		return n.protoContent("listMessage", payload)
	}
	if payload := message.GetListResponseMessage(); payload != nil {
		return n.protoContent("listResponseMessage", payload)
	}
	if payload := message.GetTemplateMessage(); payload != nil {
		return n.protoContent("templateMessage", payload)
	}
	if payload := message.GetTemplateButtonReplyMessage(); payload != nil {
		return n.protoContent("templateButtonReplyMessage", payload)
	}
	if payload := message.GetInteractiveMessage(); payload != nil {
		return n.protoContent("interactiveMessage", payload)
	}
	if payload := message.GetInteractiveResponseMessage(); payload != nil {
		return n.protoContent("interactiveResponseMessage", payload)
	}
	if payload := message.GetPollCreationMessage(); payload != nil {
		return n.protoContent("pollCreationMessage", payload)
	}
	if payload := message.GetPollUpdateMessage(); payload != nil {
		return n.protoContent("pollUpdateMessage", payload)
	}
	if payload := message.GetRequestPhoneNumberMessage(); payload != nil {
		return n.protoContent("requestPhoneNumberMessage", payload)
	}
	if payload := message.GetViewOnceMessage(); payload != nil {
		return n.protoContent("viewOnceMessage", payload)
	}
	if payload := message.GetViewOnceMessageV2(); payload != nil {
		return n.protoContent("viewOnceMessageV2", payload)
	}
	if payload := message.GetViewOnceMessageV2Extension(); payload != nil {
		return n.protoContent("viewOnceMessageV2Extension", payload)
	}
	if payload := message.GetDocumentWithCaptionMessage(); payload != nil {
		return n.protoContent("documentWithCaptionMessage", payload)
	}
	if payload := message.GetEditedMessage(); payload != nil {
		return n.protoContent("editedMessage", payload)
	}
	if payload := message.GetAlbumMessage(); payload != nil {
		return n.protoContent("albumMessage", payload)
	}
	if payload := message.GetEventMessage(); payload != nil {
		return n.protoContent("eventMessage", payload)
	}

	content, _ := json.Marshal(map[string]any{"unhandled": true})
	return normalizedContent{MessageType: "unknownMessage", Content: content}, nil
}

func (n MessageEventNormalizer) normalizeFBContent(message armadillo.MessageApplicationSub) (normalizedContent, error) {
	if message == nil {
		return normalizedContent{}, fmt.Errorf("%w: nil fb message", ErrUnsupportedMessageContent)
	}
	if protoMessage, ok := message.(proto.Message); ok {
		content, err := n.marshal.Marshal(protoMessage)
		if err != nil {
			return normalizedContent{}, fmt.Errorf("marshal fb message content: %w", err)
		}
		return normalizedContent{MessageType: "fbMessage", Content: json.RawMessage(content)}, nil
	}
	content, _ := json.Marshal(map[string]any{"unhandled": true})
	return normalizedContent{MessageType: "fbMessage", Content: content}, nil
}

func (n MessageEventNormalizer) protoContent(messageType string, payload proto.Message) (normalizedContent, error) {
	content, err := n.marshal.Marshal(payload)
	if err != nil {
		return normalizedContent{}, fmt.Errorf("marshal %s content: %w", messageType, err)
	}
	return normalizedContent{MessageType: messageType, Content: json.RawMessage(content)}, nil
}

func (n MessageEventNormalizer) messageMetadata(event *waevents.Message) json.RawMessage {
	metadata := map[string]any{
		"eventType":             "Message",
		"source":                "whatsmeow",
		"infoTimestamp":         event.Info.Timestamp.Format(time.RFC3339),
		"addressingMode":        string(event.Info.AddressingMode),
		"category":              event.Info.Category,
		"mediaType":             event.Info.MediaType,
		"isBotInvoke":           event.IsBotInvoke,
		"isEdit":                event.IsEdit,
		"edit":                  string(event.Info.Edit),
		"isEphemeral":           event.IsEphemeral,
		"isViewOnce":            event.IsViewOnce,
		"isViewOnceV2":          event.IsViewOnceV2,
		"isViewOnceV2Extension": event.IsViewOnceV2Extension,
		"isDocumentWithCaption": event.IsDocumentWithCaption,
		"isLottieSticker":       event.IsLottieSticker,
		"retryCount":            event.RetryCount,
		"serverId":              int64(event.Info.ServerID),
	}
	raw, _ := json.Marshal(metadata)
	return raw
}

func (n MessageEventNormalizer) fbMessageMetadata(event *waevents.FBMessage) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{
		"eventType":  "FBMessage",
		"source":     "whatsmeow",
		"retryCount": event.RetryCount,
	})
	return raw
}

func messageTimestampFromMessage(info watypes.MessageInfo, message *wae2e.Message) int32 {
	if ts := senderTimestampFromMessage(message); ts > 0 {
		return safeUnix(ts)
	}
	return messageTimestampFromInfo(info)
}

func messageTimestampFromInfo(info watypes.MessageInfo) int32 {
	if !info.Timestamp.IsZero() {
		return safeUnix(info.Timestamp.Unix())
	}
	return safeUnix(time.Now().UTC().Unix())
}

func senderTimestampFromMessage(message *wae2e.Message) int64 {
	if message == nil {
		return 0
	}
	metadata := message.GetMessageContextInfo().GetDeviceListMetadata()
	if metadata == nil || metadata.GetSenderTimestamp() == 0 {
		return 0
	}
	return int64(metadata.GetSenderTimestamp())
}

func safeUnix(value int64) int32 {
	if value <= 0 {
		return 0
	}
	const maxInt32 = int64(1<<31 - 1)
	if value > maxInt32 {
		return int32(maxInt32)
	}
	return int32(value)
}

func messageRemoteJID(info watypes.MessageInfo) watypes.JID {
	if info.IsGroup {
		return info.Chat
	}
	if info.IsFromMe {
		return firstTraditionalJID(info.RecipientAlt, info.Chat, jidFromString(deviceSentMetaDestination(info)))
	}
	return firstTraditionalJID(info.SenderAlt, info.Sender)
}

func messageLID(info watypes.MessageInfo) watypes.JID {
	return firstLIDJID(info.Chat, info.Sender, info.SenderAlt, info.RecipientAlt)
}

func messageParticipant(info watypes.MessageInfo) watypes.JID {
	if !info.IsGroup {
		return watypes.EmptyJID
	}
	return firstTraditionalJID(info.SenderAlt, info.Sender)
}

func messageParticipantLID(info watypes.MessageInfo) watypes.JID {
	if !info.IsGroup {
		return watypes.EmptyJID
	}
	if isLID(info.Sender) {
		return info.Sender
	}
	return watypes.EmptyJID
}

func deviceSentMetaDestination(info watypes.MessageInfo) string {
	if info.DeviceSentMeta == nil {
		return ""
	}
	return info.DeviceSentMeta.DestinationJID
}

func firstTraditionalJID(values ...watypes.JID) watypes.JID {
	for _, value := range values {
		if !value.IsEmpty() && !isLID(value) {
			return value
		}
	}
	return watypes.EmptyJID
}

func firstLIDJID(values ...watypes.JID) watypes.JID {
	for _, value := range values {
		if isLID(value) {
			return value
		}
	}
	return watypes.EmptyJID
}

func jidFromString(value string) watypes.JID {
	if strings.TrimSpace(value) == "" {
		return watypes.EmptyJID
	}
	jid, err := watypes.ParseJID(value)
	if err != nil {
		return watypes.EmptyJID
	}
	return jid
}

func isLID(jid watypes.JID) bool {
	return !jid.IsEmpty() && jid.Server == watypes.HiddenUserServer
}

func stringPtrFromJID(jid watypes.JID) *string {
	if jid.IsEmpty() {
		return nil
	}
	value := jid.String()
	return &value
}

func stringPtr(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}
