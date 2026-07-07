package message

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	dbtypes "whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/whatsapp"
	"whatsapp-go-api/internal/whatsapp/address"
)

const (
	audioOutputMIME     = "audio/ogg; codecs=opus"
	audioOutputFilename = "audio.ogg"
	envFFmpegPath       = "FFMPEG_PATH"
	envFFprobePath      = "FFPROBE_PATH"
)

type AudioConfig struct {
	MaxInputBytes      int64
	MaxDurationSeconds uint32
	DownloadTimeout    time.Duration
	ProcessingTimeout  time.Duration
	OpusBitrate        string
	SampleRate         int
	Channels           int
}

func DefaultAudioConfig() AudioConfig {
	return AudioConfig{
		MaxInputBytes:      50 * 1024 * 1024,
		MaxDurationSeconds: 60 * 60,
		DownloadTimeout:    30 * time.Second,
		ProcessingTimeout:  60 * time.Second,
		OpusBitrate:        "32k",
		SampleRate:         48000,
		Channels:           1,
	}
}

type PreparedAudio struct {
	Data            []byte
	MIMEType        string
	Filename        string
	DurationSeconds uint32
	Codec           string
	Container       string
}

type AudioProcessor interface {
	PreparePTT(ctx context.Context, input []byte, filename string, mimeType string) (PreparedAudio, error)
}

type FFmpegAudioProcessor struct {
	config AudioConfig
	logger zerolog.Logger
}

func NewFFmpegAudioProcessor(config AudioConfig, logger zerolog.Logger) *FFmpegAudioProcessor {
	return &FFmpegAudioProcessor{
		config: config,
		logger: logger.With().Str("component", "audio_processor").Logger(),
	}
}

func (p *FFmpegAudioProcessor) PreparePTT(ctx context.Context, input []byte, filename string, mimeType string) (PreparedAudio, error) {
	if len(input) == 0 {
		return PreparedAudio{}, fmt.Errorf("%w: empty audio", ErrInvalidRequest)
	}
	if int64(len(input)) > p.config.MaxInputBytes {
		return PreparedAudio{}, ErrPayloadTooLarge
	}
	if !looksLikeSupportedAudio(mimeType, filename) {
		return PreparedAudio{}, ErrUnsupportedMediaType
	}
	if err := validateAudioDependencies(); err != nil {
		return PreparedAudio{}, fmt.Errorf("%w: %w", ErrAudioProcessing, err)
	}

	processCtx, cancel := context.WithTimeout(ctx, p.config.ProcessingTimeout)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "codechat-audio-*")
	if err != nil {
		return PreparedAudio{}, fmt.Errorf("%w: temp dir: %w", ErrAudioProcessing, err)
	}
	defer os.RemoveAll(tempDir)

	inputName := safeFilename(filename)
	if inputName == "" {
		inputName = "input"
	}
	inputPath := filepath.Join(tempDir, "input-"+inputName)
	outputPath := filepath.Join(tempDir, audioOutputFilename)
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		return PreparedAudio{}, fmt.Errorf("%w: write input: %w", ErrAudioProcessing, err)
	}

	probe, err := probeAudio(processCtx, inputPath)
	if err != nil {
		return PreparedAudio{}, fmt.Errorf("%w: probe input: %w", ErrUnsupportedMediaType, err)
	}
	if !probe.isAudio() {
		return PreparedAudio{}, ErrUnsupportedMediaType
	}

	var output []byte
	var finalProbe audioProbe
	if probe.isOggOpus() {
		output = input
		finalProbe = probe
	} else {
		if err := p.convertToOggOpus(processCtx, inputPath, outputPath); err != nil {
			return PreparedAudio{}, fmt.Errorf("%w: ffmpeg: %w", ErrAudioProcessing, err)
		}
		output, err = os.ReadFile(outputPath)
		if err != nil {
			return PreparedAudio{}, fmt.Errorf("%w: read output: %w", ErrAudioProcessing, err)
		}
		finalProbe, err = probeAudio(processCtx, outputPath)
		if err != nil {
			return PreparedAudio{}, fmt.Errorf("%w: probe output: %w", ErrAudioProcessing, err)
		}
	}
	seconds, err := normalizeDuration(finalProbe.Duration, p.config.MaxDurationSeconds)
	if err != nil {
		return PreparedAudio{}, err
	}
	if int64(len(output)) > p.config.MaxInputBytes {
		return PreparedAudio{}, ErrPayloadTooLarge
	}
	return PreparedAudio{
		Data:            output,
		MIMEType:        audioOutputMIME,
		Filename:        audioOutputFilename,
		DurationSeconds: seconds,
		Codec:           "opus",
		Container:       "ogg",
	}, nil
}

func (p *FFmpegAudioProcessor) convertToOggOpus(ctx context.Context, inputPath string, outputPath string) error {
	args := []string{
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", strconv.Itoa(p.config.Channels),
		"-ar", strconv.Itoa(p.config.SampleRate),
		"-c:a", "libopus",
		"-b:a", p.config.OpusBitrate,
		"-vbr", "on",
		"-application", "voip",
		"-f", "ogg",
		outputPath,
	}
	output, err := exec.CommandContext(ctx, ffmpegPath(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, sanitizeCommandOutput(output))
	}
	return nil
}

func validateAudioDependencies() error {
	if err := validateExecutable(ffmpegPath()); err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}
	if err := validateExecutable(ffprobePath()); err != nil {
		return fmt.Errorf("ffprobe not found: %w", err)
	}
	return nil
}

func ffmpegPath() string {
	return executablePathFromEnv(envFFmpegPath, "ffmpeg")
}

func ffprobePath() string {
	return executablePathFromEnv(envFFprobePath, "ffprobe")
}

func executablePathFromEnv(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func validateExecutable(path string) error {
	if filepath.IsAbs(path) {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory", path)
		}
		return nil
	}
	_, err := exec.LookPath(path)
	return err
}

type audioProbe struct {
	Duration float64
	Codec    string
	Format   string
}

func (p audioProbe) isAudio() bool {
	return p.Codec != ""
}

func (p audioProbe) isOggOpus() bool {
	return strings.EqualFold(p.Codec, "opus") && strings.Contains(strings.ToLower(p.Format), "ogg")
}

func probeAudio(ctx context.Context, path string) (audioProbe, error) {
	formatOutput, err := exec.CommandContext(ctx, ffprobePath(), "-v", "error", "-show_entries", "format=duration,format_name", "-of", "json", path).Output()
	if err != nil {
		return audioProbe{}, err
	}
	streamOutput, err := exec.CommandContext(ctx, ffprobePath(), "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_name,duration", "-of", "json", path).Output()
	if err != nil {
		return audioProbe{}, err
	}
	var format struct {
		Format struct {
			Duration   string `json:"duration"`
			FormatName string `json:"format_name"`
		} `json:"format"`
	}
	var stream struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			Duration  string `json:"duration"`
		} `json:"streams"`
	}
	_ = json.Unmarshal(formatOutput, &format)
	_ = json.Unmarshal(streamOutput, &stream)
	probe := audioProbe{Format: format.Format.FormatName}
	if len(stream.Streams) > 0 {
		probe.Codec = stream.Streams[0].CodecName
		probe.Duration, _ = strconv.ParseFloat(stream.Streams[0].Duration, 64)
	}
	if probe.Duration <= 0 {
		probe.Duration, _ = strconv.ParseFloat(format.Format.Duration, 64)
	}
	if probe.Codec == "" {
		return audioProbe{}, ErrUnsupportedMediaType
	}
	return probe, nil
}

func normalizeDuration(value float64, maxSeconds uint32) (uint32, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return 0, ErrInvalidAudioDuration
	}
	rounded := math.Ceil(value)
	if rounded > math.MaxUint32 {
		return 0, ErrInvalidAudioDuration
	}
	if maxSeconds > 0 && rounded > float64(maxSeconds) {
		return 0, ErrInvalidAudioDuration
	}
	return uint32(rounded), nil
}

var supportedAudioMIMEs = map[string]bool{
	"audio/mpeg":  true,
	"audio/mp3":   true,
	"audio/mp4":   true,
	"audio/aac":   true,
	"audio/x-m4a": true,
	"audio/ogg":   true,
	"audio/opus":  true,
	"audio/wav":   true,
	"audio/x-wav": true,
	"audio/flac":  true,
}

var supportedAudioExtensions = map[string]bool{
	".mp3":  true,
	".mp4":  true,
	".m4a":  true,
	".aac":  true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
	".flac": true,
}

func looksLikeSupportedAudio(mimeType string, filename string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
	if supportedAudioMIMEs[normalized] {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	return supportedAudioExtensions[ext]
}

func safeFilename(original string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(original), "\\", "/")
	base := filepath.Base(normalized)
	if base == "." || base == "/" {
		return ""
	}
	var builder strings.Builder
	for _, r := range base {
		if unicode.IsControl(r) || r == '/' || r == '\\' {
			continue
		}
		builder.WriteRune(r)
	}
	name := strings.TrimSpace(builder.String())
	if name == "" || name == "." || name == ".." {
		return ""
	}
	return name
}

func sanitizeCommandOutput(output []byte) string {
	text := strings.TrimSpace(string(output))
	if len(text) > 512 {
		return text[:512]
	}
	return text
}

type audioSource struct {
	Source       string
	Data         []byte
	MIMEType     string
	Filename     string
	OriginalURL  string
	OriginalSize int
}

func (s *MessageService) SendWhatsAppAudio(ctx context.Context, instanceName string, bearerToken string, input SendWhatsAppAudioRequest) (SendResult, error) {
	if strings.TrimSpace(input.Number) == "" {
		return SendResult{}, fmt.Errorf("%w: number is required", ErrInvalidRequest)
	}
	if input.AudioMessage == nil || strings.TrimSpace(input.AudioMessage.Audio) == "" {
		return SendResult{}, fmt.Errorf("%w: audioMessage.audio is required", ErrInvalidRequest)
	}
	audioURL, err := validateHTTPURL(input.AudioMessage.Audio)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: audioMessage.audio", ErrInvalidRequest)
	}
	data, mimeType, filename, err := s.downloadAudio(ctx, audioURL)
	if err != nil {
		return SendResult{}, err
	}
	return s.sendWhatsAppAudio(ctx, instanceName, bearerToken, input.Number, audioSource{
		Source:       "url",
		Data:         data,
		MIMEType:     mimeType,
		Filename:     filename,
		OriginalURL:  audioURL,
		OriginalSize: len(data),
	}, input.Options)
}

func (s *MessageService) SendWhatsAppAudioFile(ctx context.Context, instanceName string, bearerToken string, number string, file multipart.File, header *multipart.FileHeader, options *MessageOptions) (SendResult, error) {
	if strings.TrimSpace(number) == "" {
		return SendResult{}, fmt.Errorf("%w: number is required", ErrInvalidRequest)
	}
	if file == nil || header == nil {
		return SendResult{}, fmt.Errorf("%w: attachment is required", ErrInvalidRequest)
	}
	filename := safeFilename(header.Filename)
	if filename == "" {
		filename = "audio"
	}
	maxBytes := DefaultAudioConfig().MaxInputBytes
	if header.Size > maxBytes {
		return SendResult{}, ErrPayloadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: read attachment", ErrInvalidRequest)
	}
	if int64(len(data)) > maxBytes {
		return SendResult{}, ErrPayloadTooLarge
	}
	if len(data) == 0 {
		return SendResult{}, fmt.Errorf("%w: empty attachment", ErrInvalidRequest)
	}
	mimeType := detectAudioMIME(data, header.Header.Get("Content-Type"))
	return s.sendWhatsAppAudio(ctx, instanceName, bearerToken, number, audioSource{
		Source:       "upload",
		Data:         data,
		MIMEType:     mimeType,
		Filename:     filename,
		OriginalSize: len(data),
	}, options)
}

func (s *MessageService) sendWhatsAppAudio(ctx context.Context, instanceName string, bearerToken string, number string, source audioSource, options *MessageOptions) (SendResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return SendResult{}, err
	}
	presence, delay, err := validateAudioOptions(options)
	if err != nil {
		return SendResult{}, err
	}
	quoted, err := s.resolveAudioQuoted(ctx, instance.Instance.ID, options)
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
		Address:    number,
	})
	if err != nil {
		return SendResult{}, err
	}
	recipient := resolved.CanonicalJID
	if mentionAllEnabled(options) {
		sourceCopy := audioSource{
			Source:       source.Source,
			Data:         append([]byte(nil), source.Data...),
			MIMEType:     source.MIMEType,
			Filename:     source.Filename,
			OriginalURL:  source.OriginalURL,
			OriginalSize: source.OriginalSize,
		}
		return s.enqueueMentionAll(ctx, instance.Instance, recipient, outboundRequest{
			Recipient: recipientInput(&number, nil, nil),
			Options:   options,
			Kind:      KindAudio,
			Build: func(ctx context.Context, client *whatsmeow.Client, quoted *wae2e.ContextInfo) (*wae2e.Message, string, map[string]any, error) {
				processor := s.audio
				if processor == nil {
					processor = NewFFmpegAudioProcessor(DefaultAudioConfig(), s.logger)
				}
				prepared, err := processor.PreparePTT(ctx, sourceCopy.Data, sourceCopy.Filename, sourceCopy.MIMEType)
				if err != nil {
					return nil, "", nil, err
				}
				upload, err := client.Upload(ctx, prepared.Data, whatsmeow.MediaAudio)
				if err != nil {
					return nil, "", nil, fmt.Errorf("%w: %w", ErrUploadFailed, err)
				}
				message, content := buildPTTAudioMessage(upload, prepared, quoted, sourceCopy)
				return message, "audioMessage", content, nil
			},
		}, quoted, presence, delay)
	}
	processor := s.audio
	if processor == nil {
		processor = NewFFmpegAudioProcessor(DefaultAudioConfig(), s.logger)
	}
	prepared, err := processor.PreparePTT(ctx, source.Data, source.Filename, source.MIMEType)
	if err != nil {
		return SendResult{}, err
	}
	upload, err := managed.Client.Upload(ctx, prepared.Data, whatsmeow.MediaAudio)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: %w", ErrUploadFailed, err)
	}
	protoMessage, content := buildPTTAudioMessage(upload, prepared, quoted, source)

	s.logger.Info().
		Str("operation", "message.audio.send").
		Int32("instanceId", instance.Instance.ID).
		Str("instanceName", instance.Instance.Name).
		Str("remoteJid", address.MaskAddress(recipient.String())).
		Str("source", source.Source).
		Str("originalMimeType", source.MIMEType).
		Str("outputMimeType", prepared.MIMEType).
		Int("originalSize", source.OriginalSize).
		Int("outputSize", len(prepared.Data)).
		Uint32("durationSeconds", prepared.DurationSeconds).
		Str("codec", prepared.Codec).
		Str("container", prepared.Container).
		Int64("delayMs", delay.Milliseconds()).
		Msg("sending WhatsApp PTT audio")

	if err := applyAudioPresenceAndDelay(ctx, managed.Client, recipient, presence, delay); err != nil {
		return SendResult{}, err
	}
	sendResp, err := managed.Client.SendMessage(ctx, recipient, protoMessage)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}
	content = SanitizeMessageContent(content).(map[string]any)
	raw, err := json.Marshal(content)
	if err != nil {
		return SendResult{}, fmt.Errorf("%w: marshal audio content: %w", ErrInvalidRequest, err)
	}
	remote := recipient.String()
	isGroup := recipient.Server == watypes.GroupServer
	timestamp := int32(sendResp.Timestamp.Unix())
	if timestamp <= 0 {
		timestamp = int32(time.Now().Unix())
	}
	persisted, err := s.messages.Create(ctx, dbtypes.CreateMessageInput{
		KeyID:            string(sendResp.ID),
		KeyRemoteJid:     &remote,
		KeyFromMe:        true,
		MessageType:      "audioMessage",
		Content:          raw,
		MessageTimestamp: timestamp,
		Device:           dbtypes.DeviceMessageWeb,
		IsGroup:          &isGroup,
		InstanceID:       instance.Instance.ID,
	})
	if err != nil {
		s.logger.Error().
			Err(err).
			Str("keyId", string(sendResp.ID)).
			Int32("instanceId", instance.Instance.ID).
			Str("keyRemoteJid", address.MaskAddress(remote)).
			Msg("PTT audio sent but persistence failed")
		return SendResult{}, ErrPersistenceFailed
	}
	s.dispatchSendMessageWebhook(ctx, instance.Instance, persisted)
	return SendResult{Message: persisted}, nil
}

func buildPTTAudioMessage(upload whatsmeow.UploadResponse, prepared PreparedAudio, quoted *wae2e.ContextInfo, source audioSource) (*wae2e.Message, map[string]any) {
	now := time.Now().Unix()
	audio := &wae2e.AudioMessage{
		URL:               proto.String(upload.URL),
		DirectPath:        proto.String(upload.DirectPath),
		MediaKey:          upload.MediaKey,
		Mimetype:          proto.String(prepared.MIMEType),
		FileEncSHA256:     upload.FileEncSHA256,
		FileSHA256:        upload.FileSHA256,
		FileLength:        proto.Uint64(uint64(len(prepared.Data))),
		Seconds:           proto.Uint32(prepared.DurationSeconds),
		PTT:               proto.Bool(true),
		ContextInfo:       quoted,
		MediaKeyTimestamp: proto.Int64(now),
	}
	content := map[string]any{
		"url":               upload.URL,
		"mimetype":          prepared.MIMEType,
		"fileLength":        strconv.FormatUint(uint64(len(prepared.Data)), 10),
		"seconds":           prepared.DurationSeconds,
		"ptt":               true,
		"fileSha256":        base64.StdEncoding.EncodeToString(upload.FileSHA256),
		"fileEncSha256":     base64.StdEncoding.EncodeToString(upload.FileEncSHA256),
		"mediaKey":          base64.StdEncoding.EncodeToString(upload.MediaKey),
		"directPath":        upload.DirectPath,
		"mediaKeyTimestamp": strconv.FormatInt(now, 10),
		"metadata": map[string]any{
			"source":           source.Source,
			"originalFilename": source.Filename,
			"outputFilename":   prepared.Filename,
			"originalMimeType": source.MIMEType,
			"codec":            prepared.Codec,
			"container":        prepared.Container,
		},
	}
	if quoted != nil {
		content["contextInfo"] = contextInfoContent(quoted)
	}
	return &wae2e.Message{AudioMessage: audio}, content
}

const audioMaxDelayMilliseconds int64 = 300000

func validateAudioOptions(options *MessageOptions) (*string, time.Duration, error) {
	defaultPresence := "recording"
	if options == nil {
		return &defaultPresence, 0, nil
	}
	var delay time.Duration
	if options.Delay != nil {
		if *options.Delay < 0 {
			return nil, 0, fmt.Errorf("%w: negative delay", ErrDelayInvalid)
		}
		if *options.Delay > audioMaxDelayMilliseconds {
			return nil, 0, fmt.Errorf("%w: delay too high", ErrDelayInvalid)
		}
		delay = time.Duration(*options.Delay) * time.Millisecond
	}
	presence := &defaultPresence
	if options.Presence != nil {
		normalized := strings.ToLower(strings.TrimSpace(*options.Presence))
		if normalized != "recording" && normalized != "paused" {
			return nil, 0, fmt.Errorf("%w: unsupported audio presence", ErrPresenceInvalid)
		}
		presence = &normalized
	}
	return presence, delay, nil
}

func applyAudioPresenceAndDelay(ctx context.Context, client *whatsmeow.Client, to watypes.JID, presence *string, delay time.Duration) error {
	if presence != nil && *presence == "recording" {
		if err := client.SendChatPresence(ctx, to, watypes.ChatPresenceComposing, watypes.ChatPresenceMediaAudio); err != nil {
			return fmt.Errorf("%w: set recording presence: %w", ErrSendFailed, err)
		}
		defer func() {
			_ = client.SendChatPresence(context.Background(), to, watypes.ChatPresencePaused, watypes.ChatPresenceMediaText)
		}()
	} else if presence != nil && *presence == "paused" {
		if err := client.SendChatPresence(ctx, to, watypes.ChatPresencePaused, watypes.ChatPresenceMediaText); err != nil {
			return fmt.Errorf("%w: set paused presence: %w", ErrSendFailed, err)
		}
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *MessageService) resolveAudioQuoted(ctx context.Context, instanceID int32, options *MessageOptions) (*wae2e.ContextInfo, error) {
	if options == nil {
		return nil, nil
	}
	if options.QuotedMessageID != nil {
		return s.resolveQuoted(ctx, instanceID, options)
	}
	if options.QuotedMessage != nil {
		if len(options.QuotedMessage) == 0 {
			return nil, nil
		}
		return contextInfoFromMap(options.QuotedMessage)
	}
	return nil, nil
}

func detectAudioMIME(data []byte, header string) string {
	headerType := strings.TrimSpace(strings.Split(header, ";")[0])
	if supportedAudioMIMEs[strings.ToLower(headerType)] {
		return headerType
	}
	detected := http.DetectContentType(data)
	if supportedAudioMIMEs[strings.ToLower(detected)] {
		return detected
	}
	return headerType
}

func (s *MessageService) downloadAudio(ctx context.Context, rawURL string) ([]byte, string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: audio url", ErrInvalidRequest)
	}
	if err := validatePublicHTTPHost(ctx, parsed.Hostname()); err != nil {
		return nil, "", "", err
	}
	client := &http.Client{
		Timeout: DefaultAudioConfig().DownloadTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return ErrInvalidRequest
			}
			return validatePublicHTTPHost(req.Context(), req.URL.Hostname())
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: request", ErrDownloadFailed)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", "", fmt.Errorf("%w: status %d", ErrDownloadFailed, resp.StatusCode)
	}
	maxBytes := DefaultAudioConfig().MaxInputBytes
	if resp.ContentLength > maxBytes {
		return nil, "", "", ErrPayloadTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", "", fmt.Errorf("%w: read", ErrDownloadFailed)
	}
	if int64(len(data)) > maxBytes {
		return nil, "", "", ErrPayloadTooLarge
	}
	if len(data) == 0 {
		return nil, "", "", fmt.Errorf("%w: empty audio", ErrInvalidRequest)
	}
	filename := safeFilename(pathBaseFromURL(resp.Request.URL))
	if filename == "" {
		filename = "audio"
	}
	return data, detectAudioMIME(data, resp.Header.Get("Content-Type")), filename, nil
}

func pathBaseFromURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	base := filepath.Base(value.Path)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

func validatePublicHTTPHost(ctx context.Context, host string) error {
	normalized := strings.ToLower(strings.TrimSpace(host))
	if normalized == "" || normalized == "localhost" {
		return fmt.Errorf("%w: blocked host", ErrInvalidRequest)
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", normalized)
	if err != nil {
		return fmt.Errorf("%w: resolve host", ErrDownloadFailed)
	}
	if len(ips) == 0 {
		return fmt.Errorf("%w: resolve host", ErrDownloadFailed)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return fmt.Errorf("%w: blocked private host", ErrInvalidRequest)
		}
	}
	return nil
}

func isPublicIP(ip net.IP) bool {
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return false
	}
	return true
}

func ParseMultipartAudioOptions(delayRaw string, presenceRaw string, quotedIDRaw string, quotedRaw string, mentionAllRaw string) (*MessageOptions, error) {
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

func extensionFromMIME(mimeType string) string {
	exts, _ := mime.ExtensionsByType(strings.Split(mimeType, ";")[0])
	if len(exts) == 0 {
		return ""
	}
	return exts[0]
}
