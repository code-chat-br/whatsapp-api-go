package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/types"
)

const (
	defaultWebhookWorkers   = 10
	defaultWebhookQueueSize = 1000
	defaultHTTPTimeout      = 15 * time.Second
	maxLoggedResponseBody   = 2 * 1024
	webhookUserAgent        = "CodeChat-Webhook/1.0"
)

var (
	ErrWebhookQueueFull  = errors.New("webhook queue is full")
	ErrInvalidWebhookURL = errors.New("invalid webhook URL")
	ErrUnsupportedEvent  = errors.New("unsupported webhook event")
)

type WebhookManager interface {
	Dispatch(ctx context.Context, instance WebhookInstance, event types.WebhookEvent, data any) error
	Shutdown(ctx context.Context) error
}

type ManagerConfig struct {
	GlobalURL     string
	GlobalEnabled bool
	Workers       int
	QueueSize     int
	HTTPClient    *http.Client
}

type WebhookJob struct {
	RequestID    string
	URL          string
	Body         []byte
	Headers      map[string]string
	Event        types.WebhookEvent
	InstanceID   int64
	InstanceName string
	Target       string
}

type Manager struct {
	cache         WebhookCache
	globalURL     string
	globalEnabled bool
	queue         chan WebhookJob
	client        *http.Client
	logger        zerolog.Logger
	wg            sync.WaitGroup
	mu            sync.RWMutex
	closed        bool
}

func NewManager(cache WebhookCache, cfg ManagerConfig, logger zerolog.Logger) (*Manager, error) {
	if cache == nil {
		cache = NewMemoryWebhookCache()
	}
	workers := cfg.Workers
	if workers <= 0 {
		workers = defaultWebhookWorkers
	}
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = defaultWebhookQueueSize
	}

	globalURL := strings.TrimSpace(cfg.GlobalURL)
	if globalURL != "" {
		normalized, err := NormalizeURL(globalURL)
		if err != nil {
			return nil, fmt.Errorf("%w: global webhook URL", ErrInvalidWebhookURL)
		}
		globalURL = normalized
	}
	if cfg.GlobalEnabled && globalURL == "" {
		return nil, fmt.Errorf("%w: global webhook enabled without URL", ErrInvalidWebhookURL)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	manager := &Manager{
		cache:         cache,
		globalURL:     globalURL,
		globalEnabled: cfg.GlobalEnabled,
		queue:         make(chan WebhookJob, queueSize),
		client:        client,
		logger:        logger.With().Str("component", "webhook_manager").Logger(),
	}
	for i := 0; i < workers; i++ {
		manager.wg.Add(1)
		go manager.worker()
	}
	return manager, nil
}

func (m *Manager) Dispatch(ctx context.Context, instance WebhookInstance, event types.WebhookEvent, data any) error {
	if !event.IsSupported() {
		return fmt.Errorf("%w: %s", ErrUnsupportedEvent, event)
	}

	requestID := RequestIDFromContext(ctx)
	if requestID == "" {
		requestID = uuid.NewString()
	}

	payload := WebhookPayload{
		Event:     event,
		Instance:  instance,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		m.logger.Warn().
			Err(err).
			Str("requestId", requestID).
			Str("event", string(event)).
			Int64("instanceId", instance.ID).
			Str("instanceName", instance.Name).
			Msg("webhook payload serialization failed")
		return err
	}
	headers := webhookHeaders(requestID, instance, event)

	var result error
	if cached, ok := m.cache.Get(instance.ID, instance.Name); ok && cached.Enabled {
		if !cached.Events.IsEnabled(event) {
			m.logger.Debug().
				Str("requestId", requestID).
				Str("event", string(event)).
				Int64("instanceId", instance.ID).
				Str("instanceName", instance.Name).
				Str("target", "instance").
				Msg("webhook event disabled")
		} else {
			if err := m.enqueue(WebhookJob{
				RequestID:    requestID,
				URL:          cached.URL,
				Body:         body,
				Headers:      headers,
				Event:        event,
				InstanceID:   instance.ID,
				InstanceName: instance.Name,
				Target:       "instance",
			}); err != nil {
				result = errors.Join(result, err)
			}
		}
	}

	if m.globalEnabled {
		if err := m.enqueue(WebhookJob{
			RequestID:    requestID,
			URL:          m.globalURL,
			Body:         body,
			Headers:      headers,
			Event:        event,
			InstanceID:   instance.ID,
			InstanceName: instance.Name,
			Target:       "global",
		}); err != nil {
			result = errors.Join(result, err)
		}
	}

	return result
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	close(m.queue)
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) DeleteCachedWebhook(instanceID int64, instanceName string) {
	if m.cache == nil {
		return
	}
	m.cache.Delete(instanceID, instanceName)
}

func (m *Manager) enqueue(job WebhookJob) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil
	}

	select {
	case m.queue <- job:
		m.logger.Info().
			Str("requestId", job.RequestID).
			Str("event", string(job.Event)).
			Int64("instanceId", job.InstanceID).
			Str("instanceName", job.InstanceName).
			Str("target", job.Target).
			Msg("webhook queued")
		return nil
	default:
		m.logger.Warn().
			Str("requestId", job.RequestID).
			Str("event", string(job.Event)).
			Int64("instanceId", job.InstanceID).
			Str("instanceName", job.InstanceName).
			Str("target", job.Target).
			Msg("webhook queue full")
		return ErrWebhookQueueFull
	}
}

func (m *Manager) worker() {
	defer m.wg.Done()
	for job := range m.queue {
		m.deliver(job)
	}
}

func (m *Manager) deliver(job WebhookJob) {
	start := time.Now()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, job.URL, bytes.NewReader(job.Body))
	if err != nil {
		m.logDeliveryFailure(job, 0, time.Since(start), err, "")
		return
	}
	for key, value := range job.Headers {
		req.Header.Set(key, value)
	}

	resp, err := m.client.Do(req)
	duration := time.Since(start)
	if err != nil {
		m.logDeliveryFailure(job, 0, duration, err, "")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody := readResponseBody(resp.Body)
		m.logDeliveryFailure(job, resp.StatusCode, duration, fmt.Errorf("webhook returned non-2xx status"), responseBody)
		return
	}

	m.logger.Info().
		Str("requestId", job.RequestID).
		Str("event", string(job.Event)).
		Int64("instanceId", job.InstanceID).
		Str("instanceName", job.InstanceName).
		Str("target", job.Target).
		Int("statusCode", resp.StatusCode).
		Int64("durationMs", duration.Milliseconds()).
		Str("url", safeWebhookURL(job.URL)).
		Msg("webhook delivered")
}

func (m *Manager) logDeliveryFailure(job WebhookJob, statusCode int, duration time.Duration, err error, responseBody string) {
	event := m.logger.Error().
		Err(err).
		Str("requestId", job.RequestID).
		Str("event", string(job.Event)).
		Int64("instanceId", job.InstanceID).
		Str("instanceName", job.InstanceName).
		Str("target", job.Target).
		Int("statusCode", statusCode).
		Int64("durationMs", duration.Milliseconds()).
		Str("url", safeWebhookURL(job.URL))
	if responseBody != "" {
		event.Str("responseBody", responseBody)
	}
	event.Msg("webhook delivery failed")
}

func NormalizeURL(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "", ErrInvalidWebhookURL
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", ErrInvalidWebhookURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidWebhookURL
	}
	return normalized, nil
}

func webhookHeaders(requestID string, instance WebhookInstance, event types.WebhookEvent) map[string]string {
	ownerJID := ""
	if instance.OwnerJID != nil {
		ownerJID = *instance.OwnerJID
	}
	return map[string]string{
		"Content-Type":    "application/json",
		"User-Agent":      webhookUserAgent,
		"x-request-id":    requestID,
		"x-owner-jid":     ownerJID,
		"x-instance-name": instance.Name,
		"x-instance-id":   strconv.FormatInt(instance.ID, 10),
		"x-webhook-event": string(event),
	}
}

func readResponseBody(reader io.Reader) string {
	body, err := io.ReadAll(io.LimitReader(reader, maxLoggedResponseBody+1))
	if err != nil {
		return ""
	}
	if len(body) > maxLoggedResponseBody {
		body = body[:maxLoggedResponseBody]
	}
	return string(body)
}

func safeWebhookURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

type requestIDContextKey struct{}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, strings.TrimSpace(requestID))
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if value, ok := ctx.Value(requestIDContextKey{}).(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	for _, key := range []string{"requestId", "request_id", "x-request-id"} {
		if value, ok := ctx.Value(key).(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
