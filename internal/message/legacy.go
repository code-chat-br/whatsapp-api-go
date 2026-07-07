package message

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	waCommon "go.mau.fi/whatsmeow/proto/waCommon"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"whatsapp-go-api/internal/whatsapp"
	"whatsapp-go-api/internal/whatsapp/address"
)

type preparedContact struct {
	DisplayName  string
	WUID         string
	PhoneNumber  string
	Organization *string
	VCard        string
}

type preparedLocation struct {
	Name      *string
	Address   *string
	URL       *string
	Latitude  float64
	Longitude float64
}

type preparedReaction struct {
	RemoteJID   watypes.JID
	Key         *waCommon.MessageKey
	Reaction    string
	Content     map[string]any
	MessageType string
}

func (s *MessageService) SendMediaFile(ctx context.Context, instanceName string, bearerToken string, number string, file multipart.File, header *multipart.FileHeader, mediaType string, caption *string, options *MessageOptions) (SendResult, error) {
	if strings.TrimSpace(number) == "" {
		return SendResult{}, fmt.Errorf("%w: number is required", ErrInvalidRequest)
	}
	kind, err := validateMediaFileType(mediaType)
	if err != nil {
		return SendResult{}, err
	}
	data, mimeType, filename, err := readMediaFile(file, header, kind)
	if err != nil {
		return SendResult{}, err
	}
	if !mimeCompatible(kind, mimeType) {
		return SendResult{}, fmt.Errorf("%w: incompatible mimetype", ErrInvalidRequest)
	}
	media := &MediaMessage{
		MediaType: string(kind),
		Media:     filename,
		FileName:  &filename,
		Caption:   optionalString(caption),
	}
	dataCopy := append([]byte(nil), data...)

	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(&number, nil, nil),
		Options:   options,
		Kind:      kind,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			thumbnail := s.generateMediaThumbnail(ctx, instanceName, kind, mimeType, dataCopy)
			upload, err := client.Upload(ctx, dataCopy, uploadMediaType(kind))
			if err != nil {
				return nil, "", nil, fmt.Errorf("%w: %w", ErrUploadFailed, err)
			}
			contentMedia := *media
			return buildMediaProto(kind, &contentMedia, mimeType, upload, quoted, thumbnail.Bytes)
		},
	})
}

func (s *MessageService) SendContact(ctx context.Context, instanceName string, bearerToken string, input SendContactRequest) (SendResult, error) {
	contacts, err := validateContacts(input.ContactMessage)
	if err != nil {
		return SendResult{}, err
	}
	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(input.Number, input.Chat, input.Recipient),
		Options:   input.Options,
		Kind:      KindContact,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			_ = ctx
			_ = client
			return buildContactProto(contacts, quoted)
		},
	})
}

func (s *MessageService) SendLocation(ctx context.Context, instanceName string, bearerToken string, input SendLocationRequest) (SendResult, error) {
	location, err := validateLocation(input.LocationMessage)
	if err != nil {
		return SendResult{}, err
	}
	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(input.Number, input.Chat, input.Recipient),
		Options:   input.Options,
		Kind:      KindLocation,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			_ = ctx
			_ = client
			return buildLocationProto(location, quoted)
		},
	})
}

func (s *MessageService) SendReaction(ctx context.Context, instanceName string, bearerToken string, input SendReactionRequest) (SendResult, error) {
	if mentionAllEnabled(input.Options) {
		return SendResult{}, ErrMentionAllUnsupported
	}
	reaction, err := validateReaction(input.ReactionMessage)
	if err != nil {
		return SendResult{}, err
	}
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return SendResult{}, err
	}
	managed, err := s.clients.ResolveConnectedClient(ctx, instance.Instance.Name)
	if err != nil {
		return SendResult{}, err
	}
	if managed == nil || managed.Client == nil || !managed.IsReady() {
		return SendResult{}, whatsapp.ErrClientNotConnected
	}

	now := time.Now()
	protoMessage := &wae2e.Message{ReactionMessage: &wae2e.ReactionMessage{
		Key:               reaction.Key,
		Text:              proto.String(reaction.Reaction),
		SenderTimestampMS: proto.Int64(now.UnixMilli()),
	}}
	reaction.Content["senderTimestampMs"] = strconv.FormatInt(now.UnixMilli(), 10)

	s.logger.Info().
		Str("operation", "message.reaction.send").
		Int32("instanceId", instance.Instance.ID).
		Str("instanceName", instance.Instance.Name).
		Str("remoteJid", address.MaskAddress(reaction.RemoteJID.String())).
		Msg("sending WhatsApp reaction")

	id, _ := uuid.NewV7()
	sendResp, err := managed.Client.SendMessage(ctx, reaction.RemoteJID, protoMessage, whatsmeow.SendRequestExtra{ID: id.String()})
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}
	persisted, err := s.persistSentMessage(ctx, instance.Instance, reaction.RemoteJID, string(sendResp.ID), sendResp.Timestamp, reaction.MessageType, reaction.Content, input.Options)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("keyId", string(sendResp.ID)).
			Int32("instanceId", instance.Instance.ID).
			Str("keyRemoteJid", address.MaskAddress(reaction.RemoteJID.String())).
			Msg("reaction sent but persistence failed")
		return SendResult{}, ErrPersistenceFailed
	}
	s.dispatchSendMessageWebhook(ctx, instance.Instance, persisted)
	return SendResult{Message: persisted}, nil
}

func validateMediaFileType(value string) (MessageKind, error) {
	kind := MessageKind(strings.ToLower(strings.TrimSpace(value)))
	switch kind {
	case KindImage, KindDocument, KindVideo, KindAudio, KindPTV:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: invalid media type", ErrInvalidRequest)
	}
}

func readMediaFile(file multipart.File, header *multipart.FileHeader, kind MessageKind) ([]byte, string, string, error) {
	if file == nil || header == nil {
		return nil, "", "", fmt.Errorf("%w: attachment is required", ErrInvalidRequest)
	}
	maxBytes := maxBytesForKind(kind)
	if header.Size > maxBytes {
		return nil, "", "", ErrPayloadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: read attachment", ErrInvalidRequest)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", "", ErrPayloadTooLarge
	}
	if len(data) == 0 {
		return nil, "", "", fmt.Errorf("%w: empty attachment", ErrInvalidRequest)
	}
	filename := safeFilename(header.Filename)
	if filename == "" {
		filename = string(kind)
	}
	return data, detectMediaMIME(data, header.Header.Get("Content-Type")), filename, nil
}

func detectMediaMIME(data []byte, header string) string {
	headerType := strings.TrimSpace(strings.Split(header, ";")[0])
	if headerType != "" && headerType != "application/octet-stream" {
		return headerType
	}
	return http.DetectContentType(data)
}

func validateContacts(input []ContactMessage) ([]preparedContact, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("%w: contactMessage is required", ErrInvalidRequest)
	}
	contacts := make([]preparedContact, 0, len(input))
	for _, item := range input {
		displayName := strings.TrimSpace(item.FullName)
		if displayName == "" {
			return nil, fmt.Errorf("%w: contact fullName is required", ErrInvalidRequest)
		}
		wuid := strings.TrimSpace(item.WUID)
		phone := strings.TrimSpace(item.PhoneNumber)
		vcard := ""
		if item.VCard != nil {
			vcard = strings.TrimSpace(*item.VCard)
		}
		if vcard == "" && wuid == "" && phone == "" {
			return nil, fmt.Errorf("%w: contact phoneNumber or wuid is required", ErrInvalidRequest)
		}
		organization := optionalString(item.Organization)
		if vcard == "" {
			vcard = buildVCard(displayName, wuid, phone, organization)
		}
		contacts = append(contacts, preparedContact{
			DisplayName:  displayName,
			WUID:         wuid,
			PhoneNumber:  phone,
			Organization: organization,
			VCard:        vcard,
		})
	}
	return contacts, nil
}

func buildContactProto(contacts []preparedContact, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
	if len(contacts) == 1 {
		contact := contacts[0]
		message := &wae2e.ContactMessage{
			DisplayName: proto.String(contact.DisplayName),
			Vcard:       proto.String(contact.VCard),
			ContextInfo: quoted,
		}
		content := contactContent(contact)
		if quoted != nil {
			content["contextInfo"] = contextInfoContent(quoted)
		}
		return &wae2e.Message{ContactMessage: message}, "contactMessage", content, nil
	}

	array := make([]*wae2e.ContactMessage, 0, len(contacts))
	contentContacts := make([]map[string]any, 0, len(contacts))
	for _, contact := range contacts {
		array = append(array, &wae2e.ContactMessage{
			DisplayName: proto.String(contact.DisplayName),
			Vcard:       proto.String(contact.VCard),
		})
		contentContacts = append(contentContacts, contactContent(contact))
	}
	displayName := contacts[0].DisplayName
	message := &wae2e.ContactsArrayMessage{
		DisplayName: proto.String(displayName),
		Contacts:    array,
		ContextInfo: quoted,
	}
	content := map[string]any{
		"displayName": displayName,
		"contacts":    contentContacts,
	}
	if quoted != nil {
		content["contextInfo"] = contextInfoContent(quoted)
	}
	return &wae2e.Message{ContactsArrayMessage: message}, "contactsArrayMessage", content, nil
}

func contactContent(contact preparedContact) map[string]any {
	content := map[string]any{
		"displayName": contact.DisplayName,
		"fullName":    contact.DisplayName,
		"vcard":       contact.VCard,
	}
	if contact.WUID != "" {
		content["wuid"] = contact.WUID
	}
	if contact.PhoneNumber != "" {
		content["phoneNumber"] = contact.PhoneNumber
	}
	if contact.Organization != nil {
		content["organization"] = *contact.Organization
	}
	return content
}

func buildVCard(fullName string, wuid string, phone string, organization *string) string {
	safeName := cleanVCardValue(fullName)
	safePhone := cleanVCardValue(phone)
	if safePhone == "" {
		safePhone = cleanVCardValue(wuid)
	}
	waid := cleanVCardValue(wuid)
	if at := strings.Index(waid, "@"); at >= 0 {
		waid = waid[:at]
	}
	var builder strings.Builder
	builder.WriteString("BEGIN:VCARD\n")
	builder.WriteString("VERSION:3.0\n")
	builder.WriteString("FN:")
	builder.WriteString(safeName)
	builder.WriteString("\n")
	if organization != nil {
		builder.WriteString("ORG:")
		builder.WriteString(cleanVCardValue(*organization))
		builder.WriteString("\n")
	}
	builder.WriteString("TEL;type=CELL")
	if waid != "" {
		builder.WriteString(";waid=")
		builder.WriteString(waid)
	}
	builder.WriteString(":")
	builder.WriteString(safePhone)
	builder.WriteString("\nEND:VCARD")
	return builder.String()
}

func cleanVCardValue(value string) string {
	replacer := strings.NewReplacer("\r", " ", "\n", " ")
	return strings.TrimSpace(replacer.Replace(value))
}

func validateLocation(input *LocationMessage) (preparedLocation, error) {
	if input == nil {
		return preparedLocation{}, fmt.Errorf("%w: locationMessage is required", ErrInvalidRequest)
	}
	if input.Latitude == nil {
		return preparedLocation{}, fmt.Errorf("%w: locationMessage.latitude is required", ErrInvalidRequest)
	}
	if input.Longitude == nil {
		return preparedLocation{}, fmt.Errorf("%w: locationMessage.longitude is required", ErrInvalidRequest)
	}
	latitude := *input.Latitude
	longitude := *input.Longitude
	if latitude < -90 || latitude > 90 {
		return preparedLocation{}, fmt.Errorf("%w: invalid latitude", ErrInvalidRequest)
	}
	if longitude < -180 || longitude > 180 {
		return preparedLocation{}, fmt.Errorf("%w: invalid longitude", ErrInvalidRequest)
	}
	return preparedLocation{
		Name:      optionalString(input.Name),
		Address:   optionalString(input.Address),
		URL:       optionalString(input.URL),
		Latitude:  latitude,
		Longitude: longitude,
	}, nil
}

func buildLocationProto(location preparedLocation, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
	message := &wae2e.LocationMessage{
		DegreesLatitude:  proto.Float64(location.Latitude),
		DegreesLongitude: proto.Float64(location.Longitude),
		Name:             location.Name,
		Address:          location.Address,
		URL:              location.URL,
		ContextInfo:      quoted,
	}
	content := map[string]any{
		"latitude":  location.Latitude,
		"longitude": location.Longitude,
	}
	if location.Name != nil {
		content["name"] = *location.Name
	}
	if location.Address != nil {
		content["address"] = *location.Address
	}
	if location.URL != nil {
		content["url"] = *location.URL
	}
	if quoted != nil {
		content["contextInfo"] = contextInfoContent(quoted)
	}
	return &wae2e.Message{LocationMessage: message}, "locationMessage", content, nil
}

func validateReaction(input *ReactionMessage) (preparedReaction, error) {
	if input == nil {
		return preparedReaction{}, fmt.Errorf("%w: reactionMessage is required", ErrInvalidRequest)
	}
	remoteRaw := strings.TrimSpace(input.Key.RemoteJID)
	if remoteRaw == "" {
		return preparedReaction{}, fmt.Errorf("%w: reactionMessage.key.remoteJid is required", ErrInvalidRequest)
	}
	remoteJID, err := watypes.ParseJID(remoteRaw)
	if err != nil {
		return preparedReaction{}, fmt.Errorf("%w: reactionMessage.key.remoteJid", ErrInvalidRequest)
	}
	id := strings.TrimSpace(input.Key.ID)
	if id == "" {
		return preparedReaction{}, fmt.Errorf("%w: reactionMessage.key.id is required", ErrInvalidRequest)
	}
	key := &waCommon.MessageKey{
		RemoteJID: proto.String(remoteJID.String()),
		ID:        proto.String(id),
	}
	contentKey := map[string]any{
		"remoteJid": remoteJID.String(),
		"id":        id,
	}
	if input.Key.FromMe != nil {
		key.FromMe = proto.Bool(*input.Key.FromMe)
		contentKey["fromMe"] = *input.Key.FromMe
	}
	if participant := optionalString(input.Key.Participant); participant != nil {
		key.Participant = participant
		contentKey["participant"] = *participant
	}
	reaction := input.Reaction
	content := map[string]any{
		"key":      contentKey,
		"reaction": reaction,
	}
	return preparedReaction{
		RemoteJID:   remoteJID,
		Key:         key,
		Reaction:    reaction,
		Content:     content,
		MessageType: "reactionMessage",
	}, nil
}

func ParseMultipartMessageOptions(delayRaw string, presenceRaw string, quotedIDRaw string, quotedRaw string, mentionAllRaw string) (*MessageOptions, error) {
	options := &MessageOptions{}
	hasValue := false
	if strings.TrimSpace(delayRaw) != "" {
		delay, err := strconv.ParseInt(strings.TrimSpace(delayRaw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: delay must be integer", ErrInvalidRequest)
		}
		options.Delay = &delay
		hasValue = true
	}
	if strings.TrimSpace(presenceRaw) != "" {
		presence := strings.TrimSpace(presenceRaw)
		options.Presence = &presence
		hasValue = true
	}
	if strings.TrimSpace(quotedIDRaw) != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(quotedIDRaw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: quotedMessageId must be integer", ErrInvalidRequest)
		}
		options.QuotedMessageID = &id
		hasValue = true
	}
	if strings.TrimSpace(quotedRaw) != "" {
		var quoted map[string]any
		if err := json.Unmarshal([]byte(quotedRaw), &quoted); err != nil {
			return nil, fmt.Errorf("%w: quotedMessage must be object", ErrQuotedMessageInvalid)
		}
		options.QuotedMessage = quoted
		hasValue = true
	}
	if strings.TrimSpace(mentionAllRaw) != "" {
		switch strings.ToLower(strings.TrimSpace(mentionAllRaw)) {
		case "true":
			value := true
			options.MentionAll = &value
		case "false":
			value := false
			options.MentionAll = &value
		default:
			return nil, fmt.Errorf("%w: mentionAll must be boolean", ErrInvalidRequest)
		}
		hasValue = true
	}
	if !hasValue {
		return nil, nil
	}
	return options, nil
}
