package message

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
	webhooksvc "whatsapp-go-api/internal/webhook"
	"whatsapp-go-api/internal/whatsapp"
	"whatsapp-go-api/internal/whatsapp/address"
)

const (
	downloadTimeout      = 20 * time.Second
	thumbnailMaxBytes    = 512 * 1024
	defaultMediaMaxBytes = 64 * 1024 * 1024
	imageMaxBytes        = 16 * 1024 * 1024
	audioMaxBytes        = 32 * 1024 * 1024
	videoMaxBytes        = 64 * 1024 * 1024
)

type ConnectedClientResolver interface {
	ResolveConnectedClient(ctx context.Context, instanceName string) (*whatsapp.ManagedWhatsAppClient, error)
}

type Service interface {
	SendText(ctx context.Context, instanceName string, bearerToken string, input SendTextRequest) (SendResult, error)
	SendLink(ctx context.Context, instanceName string, bearerToken string, input SendLinkRequest) (SendResult, error)
	SendMedia(ctx context.Context, instanceName string, bearerToken string, input SendMediaRequest) (SendResult, error)
	SendMediaFile(ctx context.Context, instanceName string, bearerToken string, number string, file multipart.File, header *multipart.FileHeader, mediaType string, caption *string, options *MessageOptions) (SendResult, error)
	SendWhatsAppAudio(ctx context.Context, instanceName string, bearerToken string, input SendWhatsAppAudioRequest) (SendResult, error)
	SendWhatsAppAudioFile(ctx context.Context, instanceName string, bearerToken string, number string, file multipart.File, header *multipart.FileHeader, options *MessageOptions) (SendResult, error)
	SendContact(ctx context.Context, instanceName string, bearerToken string, input SendContactRequest) (SendResult, error)
	SendLocation(ctx context.Context, instanceName string, bearerToken string, input SendLocationRequest) (SendResult, error)
	SendReaction(ctx context.Context, instanceName string, bearerToken string, input SendReactionRequest) (SendResult, error)
}

type MessageService struct {
	instances  repository.InstanceRepository
	messages   repository.MessageRepository
	clients    ConnectedClientResolver
	resolver   address.Resolver
	thumbnails ThumbnailService
	audio      AudioProcessor
	http       *http.Client
	webhooks   webhooksvc.WebhookManager
	processor  *MessageProcessingManager
	logger     zerolog.Logger
}

func NewService(
	instances repository.InstanceRepository,
	messages repository.MessageRepository,
	clients ConnectedClientResolver,
	resolver address.Resolver,
	webhooks webhooksvc.WebhookManager,
	logger zerolog.Logger,
) *MessageService {
	return &MessageService{
		instances:  instances,
		messages:   messages,
		clients:    clients,
		resolver:   resolver,
		thumbnails: NewThumbnailService(DefaultThumbnailConfig(), logger),
		audio:      NewFFmpegAudioProcessor(DefaultAudioConfig(), logger),
		http:       &http.Client{Timeout: downloadTimeout},
		webhooks:   webhooks,
		logger:     logger.With().Str("component", "message_service").Logger(),
	}
}

func (s *MessageService) SetProcessor(processor *MessageProcessingManager) {
	s.processor = processor
}

func (s *MessageService) SendText(ctx context.Context, instanceName string, bearerToken string, input SendTextRequest) (SendResult, error) {
	text, err := validateText(input.TextMessage)
	if err != nil {
		return SendResult{}, err
	}
	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(input.Number, input.Chat, input.Recipient),
		Options:   input.Options,
		Kind:      KindText,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			_ = ctx
			_ = client
			msg := &wae2e.Message{ExtendedTextMessage: &wae2e.ExtendedTextMessage{
				Text:        proto.String(text),
				ContextInfo: quoted,
			}}
			content := map[string]any{"text": text}
			if quoted != nil {
				content["contextInfo"] = contextInfoContent(quoted)
			}
			return msg, "extendedTextMessage", content, nil
		},
	})
}

func (s *MessageService) SendLink(ctx context.Context, instanceName string, bearerToken string, input SendLinkRequest) (SendResult, error) {
	link, thumbnailURL, title, description, err := validateLink(input.LinkMessage)
	if err != nil {
		return SendResult{}, err
	}
	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(input.Number, input.Chat, input.Recipient),
		Options:   input.Options,
		Kind:      KindLink,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			ext := &wae2e.ExtendedTextMessage{
				Text:        proto.String(link),
				MatchedText: proto.String(link),
				ContextInfo: quoted,
				PreviewType: wae2e.ExtendedTextMessage_IMAGE.Enum(),
			}
			content := map[string]any{"text": link, "matchedText": link}
			if title != nil {
				ext.Title = title
				content["title"] = *title
			}
			if description != nil {
				ext.Description = description
				content["description"] = *description
			}
			if thumbnailURL != nil {
				data, mimeType, err := s.download(ctx, *thumbnailURL, thumbnailMaxBytes)
				if err != nil {
					return nil, "", nil, err
				}
				if !strings.HasPrefix(mimeType, "image/") {
					return nil, "", nil, fmt.Errorf("%w: thumbnail mimetype", ErrDownloadFailed)
				}
				upload, err := client.Upload(ctx, data, whatsmeow.MediaLinkThumbnail)
				if err != nil {
					return nil, "", nil, fmt.Errorf("%w: thumbnail upload: %w", ErrUploadFailed, err)
				}
				now := time.Now().Unix()
				ext.JPEGThumbnail = data
				ext.ThumbnailDirectPath = proto.String(upload.DirectPath)
				ext.ThumbnailSHA256 = upload.FileSHA256
				ext.ThumbnailEncSHA256 = upload.FileEncSHA256
				ext.MediaKey = upload.MediaKey
				ext.MediaKeyTimestamp = proto.Int64(now)
				content["thumbnailUrl"] = *thumbnailURL
				content["thumbnailDirectPath"] = upload.DirectPath
				content["thumbnailSHA256"] = base64.StdEncoding.EncodeToString(upload.FileSHA256)
				content["thumbnailEncSHA256"] = base64.StdEncoding.EncodeToString(upload.FileEncSHA256)
				content["mediaKeyTimestamp"] = now
			}
			if quoted != nil {
				content["contextInfo"] = contextInfoContent(quoted)
			}
			return &wae2e.Message{ExtendedTextMessage: ext}, "extendedTextMessage", content, nil
		},
	})
}

func (s *MessageService) SendMedia(ctx context.Context, instanceName string, bearerToken string, input SendMediaRequest) (SendResult, error) {
	media, kind, err := validateMedia(input.MediaMessage)
	if err != nil {
		return SendResult{}, err
	}
	return s.send(ctx, instanceName, bearerToken, outboundRequest{
		Recipient: recipientInput(input.Number, input.Chat, input.Recipient),
		Options:   input.Options,
		Kind:      kind,
		Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
			data, mimeType, err := s.download(ctx, media.Media, maxBytesForKind(kind))
			if err != nil {
				return nil, "", nil, err
			}
			if !mimeCompatible(kind, mimeType) {
				return nil, "", nil, fmt.Errorf("%w: incompatible mimetype", ErrInvalidRequest)
			}
			thumbnail := s.generateMediaThumbnail(ctx, instanceName, kind, mimeType, data)
			uploadKind := uploadMediaType(kind)
			upload, err := client.Upload(ctx, data, uploadKind)
			if err != nil {
				return nil, "", nil, fmt.Errorf("%w: %w", ErrUploadFailed, err)
			}
			return buildMediaProto(kind, media, mimeType, upload, quoted, thumbnail.Bytes)
		},
	})
}

type outboundRequest struct {
	Recipient RecipientInput
	Options   *MessageOptions
	Kind      MessageKind
	Build     func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error)
}

func (s *MessageService) send(ctx context.Context, instanceName string, bearerToken string, request outboundRequest) (SendResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return SendResult{}, err
	}
	recipientAddress, err := RecipientAddress(request.Recipient)
	if err != nil {
		return SendResult{}, err
	}
	presence, delay, err := validateOptions(request.Options, request.Kind)
	if err != nil {
		return SendResult{}, err
	}
	quoted, err := s.resolveQuoted(ctx, instance.Instance.ID, request.Options)
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
	if s.resolver == nil {
		return SendResult{}, fmt.Errorf("%w: address resolver unavailable", ErrRecipientInvalid)
	}
	resolved, err := s.resolver.Resolve(ctx, managed.Client, address.ResolveInput{
		InstanceID: instance.Instance.ID,
		Address:    recipientAddress,
	})
	if err != nil {
		return SendResult{}, err
	}
	recipient := resolved.CanonicalJID
	if mentionAllEnabled(request.Options) {
		return s.enqueueMentionAll(ctx, instance.Instance, recipient, request, quoted, presence, delay)
	}
	protoMessage, messageType, content, err := request.Build(ctx, managed.Client, quoted)
	if err != nil {
		return SendResult{}, err
	}

	s.logger.Info().
		Str("operation", "message.send").
		Str("messageType", messageType).
		Int32("instanceId", instance.Instance.ID).
		Str("instanceName", instance.Instance.Name).
		Str("remoteJid", address.MaskAddress(recipient.String())).
		Str("resolutionSource", string(resolved.Source)).
		Bool("removedNinthDigit", resolved.RemovedNinth).
		Msg("sending WhatsApp message")

	if err := applyPresenceAndDelay(ctx, managed.Client, recipient, presence, delay); err != nil {
		return SendResult{}, err
	}
	id, _ := uuid.NewV7()
	sendResp, err := managed.Client.SendMessage(ctx, recipient, protoMessage, whatsmeow.SendRequestExtra{
		ID: id.String(),
	})
	if err != nil {
		s.logger.Error().Err(err).Int32("instanceId", instance.Instance.ID).Str("remoteJid", address.MaskAddress(recipient.String())).Msg("failed to send WhatsApp message")
		return SendResult{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}
	persisted, err := s.persistSentMessage(ctx, instance.Instance, recipient, string(sendResp.ID), sendResp.Timestamp, messageType, content, request.Options)
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("keyId", string(sendResp.ID)).
			Int32("instanceId", instance.Instance.ID).
			Str("keyRemoteJid", recipient.String()).
			Msg("message sent but persistence failed")
		return SendResult{}, ErrPersistenceFailed
	}
	s.dispatchSendMessageWebhook(ctx, instance.Instance, persisted)
	return SendResult{Message: persisted}, nil
}

func (s *MessageService) enqueueMentionAll(ctx context.Context, instance dbtypes.Instance, recipient watypes.JID, request outboundRequest, quoted *wae2e.ContextInfo, presence *string, delay time.Duration) (SendResult, error) {
	if recipient.Server != watypes.GroupServer || recipient.User == "" {
		return SendResult{}, ErrMentionAllRequiresGroup
	}
	if s.processor == nil {
		return SendResult{}, ErrMessageProcessorStopped
	}
	options := cloneOptions(request.Options)
	request.Options = options
	processID := newProcessID()
	job := MessageProcessingJob{
		ProcessID:          processID,
		Instance:           instance,
		InstanceName:       instance.Name,
		RemoteJID:          recipient.ToNonAD(),
		Request:            request,
		Quoted:             quotedClone(quoted),
		Presence:           presence,
		Delay:              delay,
		ExternalAttributes: externalAttributesFromOptions(options),
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.processor.Submit(job); err != nil {
		return SendResult{}, err
	}
	return SendResult{Accepted: acceptedProcessing(processID, instance.Name)}, nil
}

func (s *MessageService) processMentionAllJob(ctx context.Context, job MessageProcessingJob, cfg ProcessingConfig, logger zerolog.Logger) (dbtypes.Message, error) {
	instanceWithAuth, err := s.instances.FindByName(ctx, job.InstanceName)
	if err != nil {
		return dbtypes.Message{}, err
	}
	instance := instanceWithAuth.Instance
	if instance.Status != dbtypes.InstanceStatusOnline {
		return dbtypes.Message{}, whatsapp.ErrClientNotConnected
	}
	managed, err := s.clients.ResolveConnectedClient(ctx, job.InstanceName)
	if err != nil {
		return dbtypes.Message{}, err
	}
	if managed == nil || managed.Client == nil || !managed.IsReady() {
		return dbtypes.Message{}, whatsapp.ErrClientNotConnected
	}
	if job.RemoteJID.Server != watypes.GroupServer || job.RemoteJID.User == "" {
		return dbtypes.Message{}, ErrMentionAllRequiresGroup
	}

	groupCtx, groupCancel := context.WithTimeout(ctx, cfg.GroupInfoTimeout)
	groupStarted := time.Now()
	info, err := managed.Client.GetGroupInfo(groupCtx, job.RemoteJID)
	groupDuration := time.Since(groupStarted)
	groupCancel()
	if err != nil {
		logger.Error().
			Err(err).
			Dur("groupInfoDuration", groupDuration).
			Msg("failed to fetch group info for mentionAll")
		return dbtypes.Message{}, fmt.Errorf("%w: %w", ErrGroupInfoFetchFailed, err)
	}
	mentioned := mentionedJIDsFromParticipants(info.Participants, s.logger, job.ProcessID, job.InstanceName, job.RemoteJID)
	if len(mentioned) == 0 {
		return dbtypes.Message{}, ErrGroupHasNoParticipants
	}
	logger.Info().
		Int("participantCount", len(mentioned)).
		Dur("groupInfoDuration", groupDuration).
		Msg("group participants loaded for mentionAll")

	protoMessage, messageType, content, err := job.Request.Build(ctx, managed.Client, quotedClone(job.Quoted))
	if err != nil {
		return dbtypes.Message{}, err
	}
	protoMessage = cloneMessage(protoMessage)
	applyMentionedJIDs(protoMessage, mentioned)
	if info := contextInfoFromMessage(protoMessage); info != nil {
		content["contextInfo"] = contextInfoContent(info)
	}

	sendCtx, sendCancel := context.WithTimeout(ctx, cfg.SendTimeout)
	sendStarted := time.Now()
	if err := applyPresenceAndDelay(sendCtx, managed.Client, job.RemoteJID, job.Presence, job.Delay); err != nil {
		sendCancel()
		return dbtypes.Message{}, err
	}
	id, _ := uuid.NewV7()
	sendResp, err := managed.Client.SendMessage(sendCtx, job.RemoteJID, protoMessage, whatsmeow.SendRequestExtra{
		ID: id.String(),
	})
	sendDuration := time.Since(sendStarted)
	sendCancel()
	if err != nil {
		logger.Error().
			Err(err).
			Dur("sendDuration", sendDuration).
			Int("participantCount", len(mentioned)).
			Msg("failed to send mentionAll message")
		return dbtypes.Message{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}
	logger.Info().
		Dur("sendDuration", sendDuration).
		Int("participantCount", len(mentioned)).
		Str("messageId", string(sendResp.ID)).
		Msg("mentionAll message sent")

	persisted, err := s.persistSentMessage(ctx, instance, job.RemoteJID, string(sendResp.ID), sendResp.Timestamp, messageType, content, job.Request.Options)
	if err != nil {
		return dbtypes.Message{}, ErrPersistenceFailed
	}
	persisted.ExternalAttributes = job.ExternalAttributes
	return persisted, nil
}

func (s *MessageService) persistSentMessage(ctx context.Context, instance dbtypes.Instance, recipient watypes.JID, messageID string, sentAt time.Time, messageType string, content map[string]any, options *MessageOptions) (dbtypes.Message, error) {
	content = SanitizeMessageContent(content).(map[string]any)
	raw, err := json.Marshal(content)
	if err != nil {
		return dbtypes.Message{}, fmt.Errorf("%w: marshal content: %w", ErrInvalidRequest, err)
	}
	remote := recipient.String()
	isGroup := recipient.Server == watypes.GroupServer
	timestamp := int32(sentAt.Unix())
	if timestamp <= 0 {
		timestamp = int32(time.Now().Unix())
	}
	meta := externalAttributesFromOptions(options)
	meta["from"] = "api"
	b, _ := json.Marshal(meta)
	persisted, err := s.messages.Create(ctx, dbtypes.CreateMessageInput{
		KeyID:            messageID,
		KeyRemoteJid:     &remote,
		KeyFromMe:        true,
		MessageType:      messageType,
		Content:          raw,
		MessageTimestamp: timestamp,
		Device:           dbtypes.DeviceMessageWeb,
		IsGroup:          &isGroup,
		InstanceID:       instance.ID,
		Metadata:         b,
	})
	if err != nil {
		return dbtypes.Message{}, err
	}
	persisted.ExternalAttributes = externalAttributesFromOptions(options)
	return persisted, nil
}

func (s *MessageService) dispatchSendMessageWebhook(ctx context.Context, instance dbtypes.Instance, message dbtypes.Message) {
	if s.webhooks == nil {
		return
	}
	if err := s.webhooks.Dispatch(ctx, webhooksvc.NewWebhookInstance(instance), dbtypes.WebhookEventSendMessage, webhooksvc.NewMessageUpsertWebhookData(message)); err != nil {
		s.logger.Warn().
			Err(err).
			Int32("instanceId", instance.ID).
			Str("instanceName", instance.Name).
			Str("event", string(dbtypes.WebhookEventSendMessage)).
			Msg("send.message webhook dispatch not queued")
	}
}

func (s *MessageService) dispatchMentionAllSuccess(ctx context.Context, instance dbtypes.Instance, processID string, message dbtypes.Message, external map[string]any) {
	if s.webhooks == nil {
		return
	}
	remote := ""
	if message.KeyRemoteJid != nil {
		remote = *message.KeyRemoteJid
	}
	timestamp := time.Unix(int64(message.MessageTimestamp), 0).UTC()
	data := successMentionAllWebhookData(processID, message.KeyID, remote, participantCountFromContent(message.Content), timestamp, external)
	if err := s.webhooks.Dispatch(ctx, webhooksvc.NewWebhookInstance(instance), dbtypes.WebhookEventSendMessage, data); err != nil {
		s.logger.Warn().
			Err(err).
			Int32("instanceId", instance.ID).
			Str("instanceName", instance.Name).
			Str("event", string(dbtypes.WebhookEventSendMessage)).
			Str("processId", processID).
			Msg("send.message mentionAll webhook dispatch not queued")
	}
}

func (s *MessageService) dispatchMentionAllFailure(ctx context.Context, instance dbtypes.Instance, processID string, code string, message string, external map[string]any) {
	if s.webhooks == nil {
		return
	}
	data := failedMentionAllWebhookData(processID, code, message, external)
	if err := s.webhooks.Dispatch(ctx, webhooksvc.NewWebhookInstance(instance), dbtypes.WebhookEventSendMessage, data); err != nil {
		s.logger.Warn().
			Err(err).
			Int32("instanceId", instance.ID).
			Str("instanceName", instance.Name).
			Str("event", string(dbtypes.WebhookEventSendMessage)).
			Str("processId", processID).
			Msg("send.message mentionAll failure webhook dispatch not queued")
	}
}

func (s *MessageService) authenticateInstance(ctx context.Context, instanceName string, bearerToken string) (dbtypes.InstanceWithAuth, error) {
	name := strings.TrimSpace(instanceName)
	token := strings.TrimSpace(bearerToken)
	if name == "" || token == "" {
		return dbtypes.InstanceWithAuth{}, repository.ErrInvalidInput
	}
	instance, err := s.instances.FindByName(ctx, name)
	if err != nil {
		return dbtypes.InstanceWithAuth{}, err
	}
	if instance.Auth == nil || subtle.ConstantTimeCompare([]byte(instance.Auth.Token), []byte(token)) != 1 {
		return dbtypes.InstanceWithAuth{}, whatsapp.ErrInvalidInstanceToken
	}
	if instance.Instance.Status != dbtypes.InstanceStatusOnline {
		return dbtypes.InstanceWithAuth{}, whatsapp.ErrInstanceInactive
	}
	return instance, nil
}

func (s *MessageService) resolveQuoted(ctx context.Context, instanceID int32, options *MessageOptions) (*wae2e.ContextInfo, error) {
	if options == nil {
		return nil, nil
	}
	if options.QuotedMessageID != nil {
		if *options.QuotedMessageID <= 0 || *options.QuotedMessageID > int64(^uint32(0)>>1) {
			return nil, fmt.Errorf("%w: invalid quotedMessageId", ErrQuotedMessageInvalid)
		}
		msg, err := s.messages.FindByIDForInstance(ctx, instanceID, int32(*options.QuotedMessageID))
		if err != nil {
			if errors.Is(err, repository.ErrMessageNotFound) {
				return nil, err
			}
			return nil, fmt.Errorf("%w: %w", ErrQuotedMessageLookup, err)
		}
		return contextInfoFromPersisted(msg)
	}
	if options.QuotedMessage != nil {
		return contextInfoFromMap(options.QuotedMessage)
	}
	return nil, nil
}

func validateText(input *TextMessage) (string, error) {
	if input == nil {
		return "", fmt.Errorf("%w: textMessage is required", ErrInvalidRequest)
	}
	if strings.TrimSpace(input.Text) == "" {
		return "", fmt.Errorf("%w: textMessage.text is required", ErrInvalidRequest)
	}
	if len(input.Text) > 65536 {
		return "", fmt.Errorf("%w: text too long", ErrInvalidRequest)
	}
	return input.Text, nil
}

func validateLink(input *LinkMessage) (string, *string, *string, *string, error) {
	if input == nil {
		return "", nil, nil, nil, fmt.Errorf("%w: linkMessage is required", ErrInvalidRequest)
	}
	link, err := validateHTTPURL(input.Link)
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("%w: link", ErrInvalidRequest)
	}
	thumbnail := optionalString(input.ThumbnailURL)
	if thumbnail != nil {
		if _, err := validateHTTPURL(*thumbnail); err != nil {
			return "", nil, nil, nil, fmt.Errorf("%w: thumbnailUrl", ErrInvalidRequest)
		}
	}
	return link, thumbnail, optionalString(input.Title), optionalString(input.Description), nil
}

func validateMedia(input *MediaMessage) (*MediaMessage, MessageKind, error) {
	if input == nil {
		return nil, "", fmt.Errorf("%w: mediaMessage is required", ErrInvalidRequest)
	}
	kind := MessageKind(strings.ToLower(strings.TrimSpace(input.MediaType)))
	switch kind {
	case KindImage, KindDocument, KindVideo, KindAudio, KindPTV:
	default:
		return nil, "", fmt.Errorf("%w: invalid media type", ErrInvalidRequest)
	}
	mediaURL, err := validateHTTPURL(input.Media)
	if err != nil {
		return nil, "", fmt.Errorf("%w: media", ErrInvalidRequest)
	}
	out := *input
	out.MediaType = string(kind)
	out.Media = mediaURL
	out.FileName = optionalString(input.FileName)
	out.Caption = optionalString(input.Caption)
	return &out, kind, nil
}

func validateHTTPURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrInvalidRequest
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidRequest
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidRequest
	}
	return parsed.String(), nil
}

func optionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *MessageService) download(ctx context.Context, rawURL string, maxBytes int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("%w: request", ErrDownloadFailed)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("%w: status %d", ErrDownloadFailed, resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, "", fmt.Errorf("%w: too large", ErrDownloadFailed)
	}
	limited := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", fmt.Errorf("%w: read", ErrDownloadFailed)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", fmt.Errorf("%w: too large", ErrDownloadFailed)
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(data)
	}
	return data, mimeType, nil
}

func (s *MessageService) generateMediaThumbnail(ctx context.Context, instanceName string, kind MessageKind, mimeType string, data []byte) Thumbnail {
	if s.thumbnails == nil {
		return Thumbnail{}
	}
	startedAt := time.Now()
	var (
		thumbnail Thumbnail
		err       error
	)
	switch kind {
	case KindImage:
		thumbnail, err = s.thumbnails.FromImage(ctx, data)
	case KindVideo, KindPTV:
		thumbnail, err = s.thumbnails.FromVideo(ctx, data)
	default:
		return Thumbnail{}
	}
	event := s.logger.With().
		Str("instanceName", instanceName).
		Str("messageType", string(kind)).
		Str("mimeType", mimeType).
		Int("inputSize", len(data)).
		Dur("duration", time.Since(startedAt)).
		Logger()
	if err != nil {
		event.Warn().Err(err).Msg("failed to generate media thumbnail")
		return Thumbnail{}
	}
	event.Debug().
		Int("thumbnailSize", len(thumbnail.Bytes)).
		Int("thumbnailWidth", thumbnail.Width).
		Int("thumbnailHeight", thumbnail.Height).
		Msg("media thumbnail generated")
	return thumbnail
}

func buildMediaProto(kind MessageKind, media *MediaMessage, mimeType string, upload whatsmeow.UploadResponse, quoted *wae2e.ContextInfo, thumbnail []byte) (*wae2e.Message, string, map[string]any, error) {
	now := time.Now().Unix()
	base := map[string]any{
		"url":               upload.URL,
		"mimetype":          mimeType,
		"fileLength":        strconv.FormatUint(upload.FileLength, 10),
		"fileSha256":        base64.StdEncoding.EncodeToString(upload.FileSHA256),
		"fileEncSha256":     base64.StdEncoding.EncodeToString(upload.FileEncSHA256),
		"mediaKey":          base64.StdEncoding.EncodeToString(upload.MediaKey),
		"directPath":        upload.DirectPath,
		"mediaKeyTimestamp": strconv.FormatInt(now, 10),
	}
	if media.Caption != nil {
		base["caption"] = *media.Caption
	}
	if quoted != nil {
		base["contextInfo"] = contextInfoContent(quoted)
	}
	switch kind {
	case KindImage:
		msg := &wae2e.ImageMessage{
			URL:               proto.String(upload.URL),
			Mimetype:          proto.String(mimeType),
			FileSHA256:        upload.FileSHA256,
			FileLength:        proto.Uint64(upload.FileLength),
			MediaKey:          upload.MediaKey,
			FileEncSHA256:     upload.FileEncSHA256,
			DirectPath:        proto.String(upload.DirectPath),
			MediaKeyTimestamp: proto.Int64(now),
			JPEGThumbnail:     thumbnail,
			ContextInfo:       quoted,
		}
		if media.Caption != nil {
			msg.Caption = media.Caption
		}
		return &wae2e.Message{ImageMessage: msg}, "imageMessage", base, nil
	case KindDocument:
		fileName := fileNameForMedia(media, mimeType)
		base["fileName"] = fileName
		msg := &wae2e.DocumentMessage{
			URL:               proto.String(upload.URL),
			Mimetype:          proto.String(mimeType),
			FileSHA256:        upload.FileSHA256,
			FileLength:        proto.Uint64(upload.FileLength),
			MediaKey:          upload.MediaKey,
			FileName:          proto.String(fileName),
			FileEncSHA256:     upload.FileEncSHA256,
			DirectPath:        proto.String(upload.DirectPath),
			MediaKeyTimestamp: proto.Int64(now),
			ContextInfo:       quoted,
		}
		if media.Caption != nil {
			msg.Caption = media.Caption
		}
		return &wae2e.Message{DocumentMessage: msg}, "documentMessage", base, nil
	case KindVideo:
		msg := videoMessage(media, mimeType, upload, quoted, now, thumbnail)
		return &wae2e.Message{VideoMessage: msg}, "videoMessage", base, nil
	case KindPTV:
		msg := videoMessage(media, mimeType, upload, quoted, now, thumbnail)
		return &wae2e.Message{PtvMessage: msg}, "ptvMessage", base, nil
	case KindAudio:
		msg := &wae2e.AudioMessage{
			URL:               proto.String(upload.URL),
			Mimetype:          proto.String(mimeType),
			FileSHA256:        upload.FileSHA256,
			FileLength:        proto.Uint64(upload.FileLength),
			PTT:               proto.Bool(false),
			MediaKey:          upload.MediaKey,
			FileEncSHA256:     upload.FileEncSHA256,
			DirectPath:        proto.String(upload.DirectPath),
			MediaKeyTimestamp: proto.Int64(now),
			ContextInfo:       quoted,
		}
		return &wae2e.Message{AudioMessage: msg}, "audioMessage", base, nil
	default:
		return nil, "", nil, fmt.Errorf("%w: media type", ErrInvalidRequest)
	}
}

func videoMessage(media *MediaMessage, mimeType string, upload whatsmeow.UploadResponse, quoted *wae2e.ContextInfo, now int64, thumbnail []byte) *wae2e.VideoMessage {
	msg := &wae2e.VideoMessage{
		URL:               proto.String(upload.URL),
		Mimetype:          proto.String(mimeType),
		FileSHA256:        upload.FileSHA256,
		FileLength:        proto.Uint64(upload.FileLength),
		MediaKey:          upload.MediaKey,
		FileEncSHA256:     upload.FileEncSHA256,
		DirectPath:        proto.String(upload.DirectPath),
		MediaKeyTimestamp: proto.Int64(now),
		JPEGThumbnail:     thumbnail,
		ContextInfo:       quoted,
	}
	if media.Caption != nil {
		msg.Caption = media.Caption
	}
	return msg
}

func maxBytesForKind(kind MessageKind) int64 {
	switch kind {
	case KindImage:
		return imageMaxBytes
	case KindVideo, KindPTV:
		return videoMaxBytes
	case KindAudio:
		return audioMaxBytes
	default:
		return defaultMediaMaxBytes
	}
}

func uploadMediaType(kind MessageKind) whatsmeow.MediaType {
	switch kind {
	case KindImage:
		return whatsmeow.MediaImage
	case KindVideo, KindPTV:
		return whatsmeow.MediaVideo
	case KindAudio:
		return whatsmeow.MediaAudio
	default:
		return whatsmeow.MediaDocument
	}
}

func mimeCompatible(kind MessageKind, mimeType string) bool {
	switch kind {
	case KindImage:
		return strings.HasPrefix(mimeType, "image/")
	case KindVideo, KindPTV:
		return strings.HasPrefix(mimeType, "video/")
	case KindAudio:
		return strings.HasPrefix(mimeType, "audio/") || mimeType == "application/ogg"
	case KindDocument:
		return mimeType != "" && !strings.HasPrefix(mimeType, "image/") && !strings.HasPrefix(mimeType, "video/") && !strings.HasPrefix(mimeType, "audio/")
	default:
		return false
	}
}

func fileNameForMedia(media *MediaMessage, mimeType string) string {
	if media.FileName != nil {
		return path.Base(*media.FileName)
	}
	parsed, err := url.Parse(media.Media)
	if err == nil {
		base := path.Base(parsed.Path)
		if base != "." && base != "/" && base != "" {
			return base
		}
	}
	exts, _ := mime.ExtensionsByType(mimeType)
	if len(exts) > 0 {
		return "document" + exts[0]
	}
	return "document"
}

func contextInfoFromPersisted(message dbtypes.Message) (*wae2e.ContextInfo, error) {
	content := map[string]any{}
	if len(message.Content) > 0 {
		_ = json.Unmarshal(message.Content, &content)
	}
	quoted, err := quotedProto(message.MessageType, content)
	if err != nil {
		return nil, err
	}
	info := &wae2e.ContextInfo{
		StanzaID:      proto.String(message.KeyID),
		QuotedMessage: quoted,
	}
	if message.KeyRemoteJid != nil {
		info.RemoteJID = message.KeyRemoteJid
	}
	if message.KeyParticipant != nil && strings.TrimSpace(*message.KeyParticipant) != "" {
		info.Participant = message.KeyParticipant
	}
	return info, nil
}

func contextInfoFromMap(value map[string]any) (*wae2e.ContextInfo, error) {
	keyID, ok := stringFromMap(value, "keyId")
	if !ok || keyID == "" {
		return nil, fmt.Errorf("%w: keyId", ErrQuotedMessageInvalid)
	}
	remote, ok := stringFromMap(value, "keyRemoteJid")
	if !ok || remote == "" {
		return nil, fmt.Errorf("%w: keyRemoteJid", ErrQuotedMessageInvalid)
	}
	if _, err := watypes.ParseJID(remote); err != nil {
		return nil, fmt.Errorf("%w: keyRemoteJid", ErrQuotedMessageInvalid)
	}
	messageType, _ := stringFromMap(value, "messageType")
	content, _ := value["content"].(map[string]any)
	quoted, err := quotedProto(messageType, content)
	if err != nil {
		return nil, err
	}
	info := &wae2e.ContextInfo{
		StanzaID:      proto.String(keyID),
		RemoteJID:     proto.String(remote),
		QuotedMessage: quoted,
	}
	if participant, ok := stringFromMap(value, "keyParticipant"); ok && strings.TrimSpace(participant) != "" && strings.Contains(remote, "@"+watypes.GroupServer) {
		info.Participant = proto.String(participant)
	}
	return info, nil
}

func quotedProto(messageType string, content map[string]any) (*wae2e.Message, error) {
	text, _ := stringFromMap(content, "text")
	caption, _ := stringFromMap(content, "caption")
	switch messageType {
	case "conversation":
		if text == "" {
			return nil, fmt.Errorf("%w: quoted content", ErrQuotedMessageInvalid)
		}
		return &wae2e.Message{Conversation: proto.String(text)}, nil
	case "extendedTextMessage", "":
		if text == "" {
			text, _ = stringFromMap(content, "matchedText")
		}
		if text == "" {
			return nil, fmt.Errorf("%w: quoted content", ErrQuotedMessageInvalid)
		}
		return &wae2e.Message{ExtendedTextMessage: &wae2e.ExtendedTextMessage{Text: proto.String(text)}}, nil
	case "imageMessage":
		return &wae2e.Message{ImageMessage: &wae2e.ImageMessage{Caption: optionalString(&caption)}}, nil
	case "videoMessage":
		return &wae2e.Message{VideoMessage: &wae2e.VideoMessage{Caption: optionalString(&caption)}}, nil
	case "documentMessage":
		return &wae2e.Message{DocumentMessage: &wae2e.DocumentMessage{Caption: optionalString(&caption)}}, nil
	case "audioMessage":
		return &wae2e.Message{AudioMessage: &wae2e.AudioMessage{}}, nil
	case "contactMessage":
		displayName, _ := stringFromMap(content, "displayName")
		vcard, _ := stringFromMap(content, "vcard")
		return &wae2e.Message{ContactMessage: &wae2e.ContactMessage{DisplayName: optionalString(&displayName), Vcard: optionalString(&vcard)}}, nil
	case "contactsArrayMessage":
		displayName, _ := stringFromMap(content, "displayName")
		return &wae2e.Message{ContactsArrayMessage: &wae2e.ContactsArrayMessage{DisplayName: optionalString(&displayName)}}, nil
	case "locationMessage":
		return &wae2e.Message{LocationMessage: &wae2e.LocationMessage{}}, nil
	case "reactionMessage":
		return &wae2e.Message{ReactionMessage: &wae2e.ReactionMessage{}}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported quoted type", ErrQuotedMessageInvalid)
	}
}

func stringFromMap(value map[string]any, key string) (string, bool) {
	item, ok := value[key]
	if !ok || item == nil {
		return "", false
	}
	text, ok := item.(string)
	return strings.TrimSpace(text), ok
}

func contextInfoContent(info *wae2e.ContextInfo) map[string]any {
	if info == nil {
		return map[string]any{}
	}
	raw, err := protojson.MarshalOptions{EmitUnpopulated: false, UseProtoNames: false}.Marshal(info)
	if err != nil {
		return map[string]any{}
	}
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	sanitized, _ := SanitizeMessageContent(out).(map[string]any)
	return sanitized
}

func contextInfoFromMessage(message *wae2e.Message) *wae2e.ContextInfo {
	if message == nil {
		return nil
	}
	switch {
	case message.ExtendedTextMessage != nil:
		return message.ExtendedTextMessage.ContextInfo
	case message.ImageMessage != nil:
		return message.ImageMessage.ContextInfo
	case message.DocumentMessage != nil:
		return message.DocumentMessage.ContextInfo
	case message.VideoMessage != nil:
		return message.VideoMessage.ContextInfo
	case message.PtvMessage != nil:
		return message.PtvMessage.ContextInfo
	case message.AudioMessage != nil:
		return message.AudioMessage.ContextInfo
	case message.ContactMessage != nil:
		return message.ContactMessage.ContextInfo
	case message.ContactsArrayMessage != nil:
		return message.ContactsArrayMessage.ContextInfo
	case message.LocationMessage != nil:
		return message.LocationMessage.ContextInfo
	default:
		return nil
	}
}

func externalAttributesFromOptions(options *MessageOptions) map[string]any {
	if options == nil {
		return map[string]any{}
	}
	return cloneMap(options.ExternalAttributes)
}

func participantCountFromContent(raw json.RawMessage) int {
	var content map[string]any
	if len(raw) == 0 || json.Unmarshal(raw, &content) != nil {
		return 0
	}
	contextInfo, ok := content["contextInfo"].(map[string]any)
	if !ok {
		return 0
	}
	mentions, ok := contextInfo["mentionedJID"].([]any)
	if ok {
		return len(mentions)
	}
	mentions, ok = contextInfo["mentionedJid"].([]any)
	if ok {
		return len(mentions)
	}
	return 0
}
