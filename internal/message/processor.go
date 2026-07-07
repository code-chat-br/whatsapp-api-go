package message

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	watypes "go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	dbtypes "whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/whatsapp"
)

const (
	defaultMessageProcessingWorkers   = 4
	defaultMessageProcessingQueueSize = 100
	defaultMessageProcessingTimeout   = 60 * time.Second
	defaultMessageGroupInfoTimeout    = 30 * time.Second
	defaultMessageSendTimeout         = 30 * time.Second
)

type ProcessingConfig struct {
	Workers           int
	QueueSize         int
	ProcessingTimeout time.Duration
	GroupInfoTimeout  time.Duration
	SendTimeout       time.Duration
}

func DefaultProcessingConfig() ProcessingConfig {
	return ProcessingConfig{
		Workers:           defaultMessageProcessingWorkers,
		QueueSize:         defaultMessageProcessingQueueSize,
		ProcessingTimeout: defaultMessageProcessingTimeout,
		GroupInfoTimeout:  defaultMessageGroupInfoTimeout,
		SendTimeout:       defaultMessageSendTimeout,
	}
}

type MessageProcessingManager struct {
	service *MessageService
	config  ProcessingConfig
	queue   chan MessageProcessingJob
	logger  zerolog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
	closed bool
}

type MessageProcessingJob struct {
	ProcessID          string
	Instance           dbtypes.Instance
	InstanceName       string
	RemoteJID          watypes.JID
	Request            outboundRequest
	Quoted             *wae2e.ContextInfo
	Presence           *string
	Delay              time.Duration
	ExternalAttributes map[string]any
	CreatedAt          time.Time
}

func NewMessageProcessingManager(parent context.Context, service *MessageService, cfg ProcessingConfig, logger zerolog.Logger) (*MessageProcessingManager, error) {
	if service == nil {
		return nil, fmt.Errorf("%w: message service is required", ErrMessageProcessorStopped)
	}
	if parent == nil {
		parent = context.Background()
	}
	if cfg.Workers <= 0 {
		cfg.Workers = defaultMessageProcessingWorkers
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultMessageProcessingQueueSize
	}
	if cfg.ProcessingTimeout <= 0 {
		cfg.ProcessingTimeout = defaultMessageProcessingTimeout
	}
	if cfg.GroupInfoTimeout <= 0 {
		cfg.GroupInfoTimeout = defaultMessageGroupInfoTimeout
	}
	if cfg.SendTimeout <= 0 {
		cfg.SendTimeout = defaultMessageSendTimeout
	}
	ctx, cancel := context.WithCancel(parent)
	return &MessageProcessingManager{
		service: service,
		config:  cfg,
		queue:   make(chan MessageProcessingJob, cfg.QueueSize),
		logger:  logger.With().Str("component", "message_processor").Logger(),
		ctx:     ctx,
		cancel:  cancel,
	}, nil
}

func (m *MessageProcessingManager) Start() {
	if m == nil {
		return
	}
	for i := 0; i < m.config.Workers; i++ {
		m.wg.Add(1)
		go m.worker(i + 1)
	}
}

func (m *MessageProcessingManager) Submit(job MessageProcessingJob) error {
	if m == nil {
		return ErrMessageProcessorStopped
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return ErrMessageProcessorStopped
	}
	select {
	case m.queue <- job:
		m.logger.Info().
			Str("processId", job.ProcessID).
			Str("instanceName", job.InstanceName).
			Str("remoteJid", job.RemoteJID.String()).
			Bool("mentionAll", true).
			Msg("message processing job queued")
		return nil
	default:
		return ErrMessageQueueFull
	}
}

func (m *MessageProcessingManager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
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
		m.cancel()
		return nil
	case <-ctx.Done():
		m.cancel()
		return ctx.Err()
	}
}

func (m *MessageProcessingManager) worker(workerID int) {
	defer m.wg.Done()
	for job := range m.queue {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					m.logger.Error().
						Interface("panic", recovered).
						Int("workerId", workerID).
						Str("processId", job.ProcessID).
						Str("instanceName", job.InstanceName).
						Str("remoteJid", job.RemoteJID.String()).
						Bool("mentionAll", true).
						Msg("message processing worker recovered from panic")
					m.service.dispatchMentionAllFailure(m.ctx, job.Instance, job.ProcessID, "GROUP_MENTION_PROCESSING_FAILED", "Nao foi possivel concluir o envio da mensagem para o grupo.", job.ExternalAttributes)
				}
			}()
			m.process(job)
		}()
	}
}

func (m *MessageProcessingManager) process(job MessageProcessingJob) {
	startedAt := time.Now()
	jobCtx, cancel := context.WithTimeout(m.ctx, m.config.ProcessingTimeout)
	defer cancel()

	logger := m.logger.With().
		Str("processId", job.ProcessID).
		Str("instanceName", job.InstanceName).
		Str("remoteJid", job.RemoteJID.String()).
		Bool("mentionAll", true).
		Logger()

	message, err := m.service.processMentionAllJob(jobCtx, job, m.config, logger)
	status := "sent"
	if err != nil {
		status = "failed"
		logger.Error().
			Err(err).
			Dur("duration", time.Since(startedAt)).
			Str("status", status).
			Msg("message processing job failed")
		m.service.dispatchMentionAllFailure(jobCtx, job.Instance, job.ProcessID, errorCodeForProcessing(err), safeProcessingError(err), job.ExternalAttributes)
		return
	}

	logger.Info().
		Dur("duration", time.Since(startedAt)).
		Str("status", status).
		Msg("message processing job completed")
	m.service.dispatchMentionAllSuccess(jobCtx, job.Instance, job.ProcessID, message, job.ExternalAttributes)
}

func newProcessID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

func errorCodeForProcessing(err error) string {
	switch {
	case errors.Is(err, ErrGroupInfoFetchFailed):
		return "GROUP_INFO_FETCH_FAILED"
	case errors.Is(err, ErrGroupHasNoParticipants):
		return "GROUP_HAS_NO_PARTICIPANTS"
	case errors.Is(err, whatsapp.ErrClientNotConnected):
		return "INSTANCE_NOT_CONNECTED"
	case errors.Is(err, ErrSendFailed):
		return "MESSAGE_SEND_FAILED"
	default:
		return "GROUP_MENTION_PROCESSING_FAILED"
	}
}

func safeProcessingError(err error) string {
	switch {
	case errors.Is(err, ErrGroupInfoFetchFailed):
		return "Nao foi possivel consultar os participantes do grupo."
	case errors.Is(err, ErrGroupHasNoParticipants):
		return "O grupo nao possui participantes validos para mencao."
	case errors.Is(err, whatsapp.ErrClientNotConnected):
		return "A instancia nao esta conectada."
	case errors.Is(err, ErrSendFailed):
		return "Nao foi possivel enviar a mensagem pelo WhatsApp."
	default:
		return "Nao foi possivel concluir o envio da mensagem para o grupo."
	}
}

func quotedClone(info *wae2e.ContextInfo) *wae2e.ContextInfo {
	if info == nil {
		return nil
	}
	cloned, ok := proto.Clone(info).(*wae2e.ContextInfo)
	if !ok {
		return nil
	}
	return cloned
}
