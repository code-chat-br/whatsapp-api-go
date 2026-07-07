package chat

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	MediaTypeImage    = "imageMessage"
	MediaTypeVideo    = "videoMessage"
	MediaTypeDocument = "documentMessage"
	MediaTypeSticker  = "stickerMessage"
	MediaTypeAudio    = "audioMessage"
)

var supportedMediaTypes = map[string]struct{}{
	MediaTypeImage:    {},
	MediaTypeVideo:    {},
	MediaTypeDocument: {},
	MediaTypeSticker:  {},
	MediaTypeAudio:    {},
}

var fallbackMIME = map[string]string{
	MediaTypeImage:    "application/octet-stream",
	MediaTypeVideo:    "application/octet-stream",
	MediaTypeDocument: "application/octet-stream",
	MediaTypeSticker:  "image/webp",
	MediaTypeAudio:    "application/octet-stream",
}

var extensionFallback = map[string]string{
	"image/jpeg":      ".jpeg",
	"image/png":       ".png",
	"image/webp":      ".webp",
	"video/mp4":       ".mp4",
	"audio/ogg":       ".ogg",
	"audio/mpeg":      ".mp3",
	"audio/mp4":       ".m4a",
	"application/pdf": ".pdf",
}

var unsafeFileNameChars = regexp.MustCompile(`[[:cntrl:]/\\]+`)

func IsSupportedMediaType(messageType string) bool {
	_, ok := supportedMediaTypes[strings.TrimSpace(messageType)]
	return ok
}

func BuildDownloadableMessage(messageType string, content json.RawMessage) (whatsmeow.DownloadableMessage, MediaMetadata, error) {
	messageType = strings.TrimSpace(messageType)
	normalized, err := normalizeMediaContentJSON(content)
	if err != nil {
		return nil, MediaMetadata{}, err
	}
	options := protojson.UnmarshalOptions{DiscardUnknown: true}

	switch messageType {
	case MediaTypeImage:
		message := &wae2e.ImageMessage{}
		if err := unmarshalMediaContent(options, normalized, message); err != nil {
			return nil, MediaMetadata{}, err
		}
		return message, metadataFromImage(message), validateDownloadable(message)
	case MediaTypeVideo:
		message := &wae2e.VideoMessage{}
		if err := unmarshalMediaContent(options, normalized, message); err != nil {
			return nil, MediaMetadata{}, err
		}
		return message, metadataFromVideo(message), validateDownloadable(message)
	case MediaTypeDocument:
		message := &wae2e.DocumentMessage{}
		if err := unmarshalMediaContent(options, normalized, message); err != nil {
			return nil, MediaMetadata{}, err
		}
		return message, metadataFromDocument(message), validateDownloadable(message)
	case MediaTypeSticker:
		message := &wae2e.StickerMessage{}
		if err := unmarshalMediaContent(options, normalized, message); err != nil {
			return nil, MediaMetadata{}, err
		}
		return message, metadataFromSticker(message), validateDownloadable(message)
	case MediaTypeAudio:
		message := &wae2e.AudioMessage{}
		if err := unmarshalMediaContent(options, normalized, message); err != nil {
			return nil, MediaMetadata{}, err
		}
		return message, metadataFromAudio(message), validateDownloadable(message)
	default:
		return nil, MediaMetadata{}, fmt.Errorf("%w: %s", ErrUnsupportedMediaType, messageType)
	}
}

func unmarshalMediaContent(options protojson.UnmarshalOptions, content json.RawMessage, message proto.Message) error {
	if err := options.Unmarshal(content, message); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidMediaContent, err)
	}
	return nil
}

func normalizeMediaContentJSON(content json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("%w: content is required", ErrInvalidMediaContent)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || len(object) == 0 {
		return nil, fmt.Errorf("%w: content must be a non-empty object", ErrInvalidMediaContent)
	}

	aliases := map[string]string{
		"fileSha256":    "fileSHA256",
		"fileEncSha256": "fileEncSHA256",
	}
	for from, to := range aliases {
		if value, ok := object[from]; ok {
			if _, exists := object[to]; !exists {
				object[to] = value
			}
			delete(object, from)
		}
	}
	normalized, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidMediaContent, err)
	}
	return normalized, nil
}

func validateDownloadable(message whatsmeow.DownloadableMessage) error {
	if message == nil {
		return fmt.Errorf("%w: message is empty", ErrInvalidMediaContent)
	}
	if directPath := strings.TrimSpace(message.GetDirectPath()); directPath == "" || !strings.HasPrefix(directPath, "/") {
		return fmt.Errorf("%w: directPath is required", ErrInvalidMediaContent)
	}
	if len(message.GetMediaKey()) == 0 {
		return fmt.Errorf("%w: mediaKey is required", ErrInvalidMediaContent)
	}
	if len(message.GetFileSHA256()) == 0 {
		return fmt.Errorf("%w: fileSha256 is required", ErrInvalidMediaContent)
	}
	if len(message.GetFileEncSHA256()) == 0 {
		return fmt.Errorf("%w: fileEncSha256 is required", ErrInvalidMediaContent)
	}
	return nil
}

func metadataFromImage(message *wae2e.ImageMessage) MediaMetadata {
	return MediaMetadata{
		MediaType: MediaTypeImage,
		MIMEType:  safeMIME(MediaTypeImage, message.GetMimetype()),
		Size: map[string]any{
			"fileLength": strconv.FormatUint(message.GetFileLength(), 10),
			"height":     message.GetHeight(),
			"width":      message.GetWidth(),
		},
	}
}

func metadataFromVideo(message *wae2e.VideoMessage) MediaMetadata {
	return MediaMetadata{
		MediaType: MediaTypeVideo,
		MIMEType:  safeMIME(MediaTypeVideo, message.GetMimetype()),
		Size: map[string]any{
			"fileLength": strconv.FormatUint(message.GetFileLength(), 10),
			"height":     message.GetHeight(),
			"width":      message.GetWidth(),
			"seconds":    message.GetSeconds(),
		},
	}
}

func metadataFromDocument(message *wae2e.DocumentMessage) MediaMetadata {
	metadata := MediaMetadata{
		MediaType: MediaTypeDocument,
		MIMEType:  safeMIME(MediaTypeDocument, message.GetMimetype()),
		Size: map[string]any{
			"fileLength": strconv.FormatUint(message.GetFileLength(), 10),
			"pageCount":  message.GetPageCount(),
		},
	}
	if name := strings.TrimSpace(message.GetFileName()); name != "" {
		metadata.FileName = SanitizeFileName(name)
	}
	return metadata
}

func metadataFromSticker(message *wae2e.StickerMessage) MediaMetadata {
	return MediaMetadata{
		MediaType: MediaTypeSticker,
		MIMEType:  safeMIME(MediaTypeSticker, message.GetMimetype()),
		Size: map[string]any{
			"fileLength": strconv.FormatUint(message.GetFileLength(), 10),
			"height":     message.GetHeight(),
			"width":      message.GetWidth(),
			"isAnimated": message.GetIsAnimated(),
		},
	}
}

func metadataFromAudio(message *wae2e.AudioMessage) MediaMetadata {
	return MediaMetadata{
		MediaType: MediaTypeAudio,
		MIMEType:  safeMIME(MediaTypeAudio, message.GetMimetype()),
		Size: map[string]any{
			"fileLength": strconv.FormatUint(message.GetFileLength(), 10),
			"seconds":    message.GetSeconds(),
			"ptt":        message.GetPTT(),
		},
	}
}

func safeMIME(messageType string, value string) string {
	base, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil || strings.TrimSpace(base) == "" || strings.ContainsAny(base, "\r\n\x00") {
		return fallbackMIME[messageType]
	}
	return strings.TrimSpace(base)
}

func CompleteMediaFileName(metadata MediaMetadata, keyID string) string {
	ext := extensionForMIME(metadata.MIMEType)
	if metadata.MediaType == MediaTypeSticker && ext == "" {
		ext = ".webp"
	}
	if metadata.FileName != "" {
		return ensureFileExtension(SanitizeFileName(metadata.FileName), ext)
	}
	base := strings.TrimSpace(keyID)
	if base == "" {
		sum := sha256.Sum256([]byte(metadata.MediaType + "|" + metadata.MIMEType))
		base = "media_" + hex.EncodeToString(sum[:4])
	}
	return ensureFileExtension(SanitizeFileName(base), ext)
}

func extensionForMIME(mimeType string) string {
	if ext, ok := extensionFallback[mimeType]; ok {
		return ext
	}
	extensions, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(extensions) == 0 {
		return ""
	}
	return extensions[0]
}

func ensureFileExtension(fileName string, ext string) string {
	fileName = SanitizeFileName(fileName)
	if ext == "" {
		return fileName
	}
	if strings.EqualFold(filepath.Ext(fileName), ext) {
		return fileName
	}
	return strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ext
}

func SanitizeFileName(value string) string {
	value = strings.TrimSpace(filepath.Base(value))
	value = unsafeFileNameChars.ReplaceAllString(value, "_")
	value = strings.ReplaceAll(value, "..", "_")
	value = strings.Trim(value, ". _")
	if value == "" {
		value = "media"
	}
	if len(value) > 180 {
		ext := filepath.Ext(value)
		base := strings.TrimSuffix(value, ext)
		maxBase := 180 - len(ext)
		if maxBase < 1 {
			maxBase = 180
			ext = ""
		}
		if len(base) > maxBase {
			base = base[:maxBase]
		}
		value = base + ext
	}
	return value
}
