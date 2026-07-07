package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
)

const DefaultMaxMediaBytes int64 = 50 * 1024 * 1024

func (s *ChatService) MediaData(ctx context.Context, instanceName string, bearerToken string, input MediaDataRequest) (MediaDownloadResult, error) {
	started := time.Now()
	mode, err := input.Validate()
	if err != nil {
		return MediaDownloadResult{}, err
	}
	instance, client, err := s.authorizedClient(ctx, instanceName, bearerToken)
	if err != nil {
		return MediaDownloadResult{}, err
	}

	messageType, content, keyID, messageID, err := s.resolveMediaDataRequest(ctx, instance.ID, mode, input)
	if err != nil {
		return MediaDownloadResult{}, err
	}

	downloadable, metadata, err := BuildDownloadableMessage(messageType, content)
	if err != nil {
		return MediaDownloadResult{}, err
	}
	metadata.FileName = CompleteMediaFileName(metadata, keyID)

	declaredSize := declaredFileLength(metadata)
	if declaredSize > s.mediaBytesLimit() {
		return MediaDownloadResult{}, ErrMediaTooLarge
	}

	data, err := client.Download(ctx, downloadable)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return MediaDownloadResult{}, err
		}
		s.logger.Error().
			Err(err).
			Int32("instance_id", instance.ID).
			Str("instance_name", instance.Name).
			Str("lookup_mode", string(mode)).
			Int32("message_id", messageID).
			Str("key_id", maskMediaKeyID(keyID)).
			Str("message_type", messageType).
			Msg("media download failed")
		return MediaDownloadResult{}, fmt.Errorf("%w: %w", ErrMediaDownloadFailed, err)
	}
	if int64(len(data)) > s.mediaBytesLimit() {
		return MediaDownloadResult{}, ErrMediaTooLarge
	}
	if declaredSize > 0 && declaredSize != int64(len(data)) {
		s.logger.Debug().
			Int32("instance_id", instance.ID).
			Str("instance_name", instance.Name).
			Int64("declared_size", declaredSize).
			Int("downloaded_size", len(data)).
			Msg("media size differs from declared metadata")
	}

	s.logger.Debug().
		Int32("instance_id", instance.ID).
		Str("instance_name", instance.Name).
		Str("lookup_mode", string(mode)).
		Int32("message_id", messageID).
		Str("key_id", maskMediaKeyID(keyID)).
		Str("message_type", messageType).
		Str("mime_type", metadata.MIMEType).
		Int64("declared_size", declaredSize).
		Int("downloaded_size", len(data)).
		Dur("duration", time.Since(started)).
		Msg("media downloaded")

	return MediaDownloadResult{Data: data, MediaMetadata: metadata}, nil
}

func (s *ChatService) resolveMediaDataRequest(ctx context.Context, instanceID int32, mode MediaDataMode, input MediaDataRequest) (string, json.RawMessage, string, int32, error) {
	switch mode {
	case MediaDataModeID:
		message, err := s.messages.FindByIDForInstance(ctx, instanceID, int32(*input.ID))
		if err != nil {
			return "", nil, "", 0, mediaLookupError(err)
		}
		messageType, content, keyID, err := MediaRequestFromStoredMessage(message)
		return messageType, content, keyID, message.ID, err
	case MediaDataModeKeyID:
		message, err := s.messages.FindByKeyIDForInstance(ctx, instanceID, strings.TrimSpace(*input.KeyID))
		if err != nil {
			return "", nil, "", 0, mediaLookupError(err)
		}
		messageType, content, keyID, err := MediaRequestFromStoredMessage(message)
		return messageType, content, keyID, message.ID, err
	case MediaDataModePayload:
		keyID := ""
		if input.KeyID != nil {
			keyID = strings.TrimSpace(*input.KeyID)
		}
		return strings.TrimSpace(input.MessageType), input.Content, keyID, 0, nil
	default:
		return "", nil, "", 0, ErrInvalidMediaRequest
	}
}

func MediaRequestFromStoredMessage(message dbtypes.Message) (string, json.RawMessage, string, error) {
	messageType := strings.TrimSpace(message.MessageType)
	if !IsSupportedMediaType(messageType) {
		return "", nil, "", ErrMessageIsNotMedia
	}
	content := json.RawMessage(strings.TrimSpace(string(message.Content)))
	if len(content) == 0 || string(content) == "null" {
		return "", nil, "", fmt.Errorf("%w: content is required", ErrInvalidMediaContent)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(content, &object); err != nil || len(object) == 0 {
		return "", nil, "", fmt.Errorf("%w: content must be a non-empty object", ErrInvalidMediaContent)
	}
	return messageType, content, strings.TrimSpace(message.KeyID), nil
}

func mediaLookupError(err error) error {
	if errors.Is(err, repository.ErrMessageNotFound) {
		return ErrMediaMessageNotFound
	}
	return ErrDatabaseOperation
}

func (s *ChatService) mediaBytesLimit() int64 {
	if s.maxMediaBytes <= 0 {
		return DefaultMaxMediaBytes
	}
	return s.maxMediaBytes
}

func declaredFileLength(metadata MediaMetadata) int64 {
	if metadata.Size == nil {
		return 0
	}
	value, ok := metadata.Size["fileLength"]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil || parsed < 0 {
			return 0
		}
		return parsed
	case uint64:
		if typed > uint64(math.MaxInt64) {
			return math.MaxInt64
		}
		return int64(typed)
	case int64:
		if typed < 0 {
			return 0
		}
		return typed
	case int:
		if typed < 0 {
			return 0
		}
		return int64(typed)
	default:
		return 0
	}
}

func maskMediaKeyID(keyID string) string {
	keyID = strings.TrimSpace(keyID)
	if len(keyID) <= 10 {
		return keyID
	}
	return keyID[:6] + "**********" + keyID[len(keyID)-6:]
}
