package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow/store/sqlstore"

	authjwt "whatsapp-go-api/internal/authentication/jwt"
	"whatsapp-go-api/internal/chat"
	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/database/migrations"
	"whatsapp-go-api/internal/database/postgres"
	"whatsapp-go-api/internal/database/repository"
	"whatsapp-go-api/internal/group"
	apphttp "whatsapp-go-api/internal/http"
	"whatsapp-go-api/internal/instance"
	"whatsapp-go-api/internal/message"
	webhooksvc "whatsapp-go-api/internal/webhook"
	"whatsapp-go-api/internal/whatsapp"
	"whatsapp-go-api/internal/whatsapp/address"
)

type Application struct {
	config           *config.Config
	logger           zerolog.Logger
	database         *pgxpool.Pool
	sqlstore         *sqlstore.Container
	clientHub        whatsapp.ClientHub
	whatsApp         whatsapp.ConnectionService
	webhooks         webhooksvc.WebhookManager
	messageProcessor *message.MessageProcessingManager
	httpApp          *fiber.App
	readiness        *ReadinessState
}

func New(ctx context.Context, cfg *config.Config, logger zerolog.Logger) (*Application, error) {
	readiness := &ReadinessState{}

	if err := whatsapp.ConfigureSessionDevice(whatsapp.SessionDeviceConfig{
		Client: cfg.WhatsApp.SessionPhoneClient,
		Name:   cfg.WhatsApp.SessionPhoneName,
	}, logger); err != nil {
		return nil, fmt.Errorf("configure WhatsApp session device: %w", err)
	}

	database, err := postgres.NewPostgresPool(ctx, cfg.Database.URL, logger)
	if err != nil {
		return nil, err
	}
	readiness.MarkDatabaseReady()
	logger.Info().Msg("database_connected")

	if err := migrations.Run(ctx, database); err != nil {
		database.Close()
		return nil, err
	}
	logger.Info().Msg("database_migrations_completed")

	waLogger := whatsapp.NewWhatsmeowLogger(logger)
	clientFactory, err := whatsapp.NewSQLStoreClientFactory(ctx, cfg.WhatsAppSession, cfg.Database.URL, waLogger)
	if err != nil {
		database.Close()
		return nil, err
	}
	whatsmeowStore := clientFactory.Store()
	readiness.MarkWhatsmeowStoreReady()
	logWhatsAppSessionStoreInitialized(logger, cfg.WhatsAppSession)
	logger.Info().Msg("whatsmeow_migrations_completed")

	instanceRepository := repository.NewInstanceRepository(database, logger)
	authRepository := repository.NewAuthRepository(database, logger)
	webhookRepository := repository.NewWebhookRepository(database, logger)
	messageRepository := repository.NewMessageRepository(database, logger)
	messageUpdateRepository := repository.NewMessageUpdateRepository(database, logger)
	contactRepository := repository.NewContactRepository(database, logger)
	addressRepository := repository.NewAddressMappingRepository(database, logger)
	logger.Info().Msg("repositories_initialized")

	webhookCache := webhooksvc.NewMemoryWebhookCache()
	if err := webhooksvc.LoadCache(ctx, webhookRepository, webhookCache, logger); err != nil {
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}
	webhookManager, err := webhooksvc.NewManager(webhookCache, webhooksvc.ManagerConfig{
		GlobalURL:     cfg.Webhook.GlobalURL,
		GlobalEnabled: cfg.Webhook.GlobalEnabled,
	}, logger)
	if err != nil {
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}

	tokenGenerator, err := authjwt.NewJWTGenerator(cfg.Authentication, logger)
	if err != nil {
		_ = webhookManager.Shutdown(context.Background())
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}
	tokenValidator, err := authjwt.NewJWTValidator(cfg.Authentication, logger)
	if err != nil {
		_ = webhookManager.Shutdown(context.Background())
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}

	instanceService := instance.NewService(database, instanceRepository, authRepository, tokenGenerator, tokenValidator, logger)
	webhookService := webhooksvc.NewService(database, instanceRepository, webhookRepository, webhookCache, logger)
	clientHub := whatsapp.NewClientHub()
	readiness.MarkClientHubReady()
	logger.Info().Msg("whatsapp_hub_initialized")

	connectionLock := whatsapp.NewPostgresInstanceConnectionLock(instanceRepository)
	eventPersistence := whatsapp.NewEventPersistenceService(whatsapp.EventPersistenceConfig{
		SaveDataNewMessage:       cfg.Database.SaveDataNewMessage,
		SaveMessageUpdate:        cfg.Database.SaveMessageUpdate,
		SaveDataContacts:         cfg.Database.SaveDataContacts,
		InitialContactSyncDelay:  30 * time.Second,
		ContactProfileWorkers:    5,
		ProfilePictureTimeout:    cfg.WhatsApp.ProfilePictureTimeout,
		ReceiptRetryAttempts:     3,
		ReceiptRetryInitialDelay: 100 * time.Millisecond,
	}, messageRepository, messageUpdateRepository, contactRepository, logger)
	eventPersistence.SetWebhookDispatcher(instanceRepository, webhookManager)
	whatsAppService, err := whatsapp.NewService(cfg.WhatsApp, instanceRepository, clientFactory, clientHub, connectionLock, eventPersistence, webhookManager, logger)
	if err != nil {
		_ = webhookManager.Shutdown(context.Background())
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}
	addressResolver := address.NewResolver(addressRepository, cfg.WhatsApp.AddressCacheTTL, logger)
	messageService := message.NewService(instanceRepository, messageRepository, whatsAppService, addressResolver, webhookManager, logger)
	messageProcessor, err := message.NewMessageProcessingManager(ctx, messageService, message.ProcessingConfig{
		Workers:           cfg.MessageProcessing.Workers,
		QueueSize:         cfg.MessageProcessing.QueueSize,
		ProcessingTimeout: cfg.MessageProcessing.ProcessingTimeout,
		GroupInfoTimeout:  cfg.MessageProcessing.GroupInfoTimeout,
		SendTimeout:       cfg.MessageProcessing.SendTimeout,
	}, logger)
	if err != nil {
		_ = whatsAppService.Shutdown(context.Background())
		_ = webhookManager.Shutdown(context.Background())
		_ = whatsmeowStore.Close()
		database.Close()
		return nil, err
	}
	messageService.SetProcessor(messageProcessor)
	messageProcessor.Start()
	chatService := chat.NewService(instanceRepository, messageRepository, whatsAppService, addressResolver, logger)
	groupService := group.NewService(instanceRepository, whatsAppService, logger)

	httpApp := apphttp.NewServer(logger, *cfg, instanceService, webhookService, whatsAppService, messageService, chatService, groupService, readiness)
	logger.Info().Msg("routes_registered")

	return &Application{
		config:           cfg,
		logger:           logger,
		database:         database,
		sqlstore:         whatsmeowStore,
		clientHub:        clientHub,
		whatsApp:         whatsAppService,
		webhooks:         webhookManager,
		messageProcessor: messageProcessor,
		httpApp:          httpApp,
		readiness:        readiness,
	}, nil
}

func logWhatsAppSessionStoreInitialized(logger zerolog.Logger, cfg config.WhatsAppSessionConfig) {
	event := logger.Info().Str("store", string(cfg.Store))
	switch cfg.Store {
	case config.WhatsAppSessionStorePostgres:
		event.Bool("dedicated_database", cfg.PostgresURL != "")
	case config.WhatsAppSessionStoreSQLite:
		event.Str("database_file", sqliteDatabaseFileForLog(cfg.SQLiteDSN))
	}
	event.Msg("whatsmeow session store initialized")
}

func sqliteDatabaseFileForLog(dsn string) string {
	beforeQuery, _, _ := strings.Cut(strings.TrimSpace(dsn), "?")
	return strings.TrimPrefix(beforeQuery, "file:")
}

func (a *Application) Run(ctx context.Context) error {
	if a.config.WhatsApp.AutoReconnect {
		a.readiness.MarkRestorationStarted()
		a.logger.Info().Msg("connection_restoration_started")
		go func() {
			if err := a.whatsApp.Restore(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.logger.Error().Err(err).Msg("failed to restore WhatsApp connections")
			}
		}()
	} else {
		a.readiness.MarkRestorationStarted()
	}

	serverErrors := make(chan error, 1)
	go func() {
		address := ":" + a.config.Server.Port
		a.logger.Info().Str("address", address).Msg("http_server_listening")
		serverErrors <- a.httpApp.Listen(address)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return a.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		return fmt.Errorf("HTTP server failed: %w", err)
	}
}

func (a *Application) Shutdown(ctx context.Context) error {
	a.logger.Info().Msg("shutdown_started")

	var result error
	if a.httpApp != nil {
		if err := a.httpApp.ShutdownWithContext(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown fiber: %w", err))
		}
	}
	if a.messageProcessor != nil {
		if err := a.messageProcessor.Shutdown(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown message processor: %w", err))
		}
	}
	if a.whatsApp != nil {
		if err := a.whatsApp.Shutdown(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown whatsapp: %w", err))
		}
	}
	if a.webhooks != nil {
		if err := a.webhooks.Shutdown(ctx); err != nil {
			result = errors.Join(result, fmt.Errorf("shutdown webhooks: %w", err))
		}
	}
	if a.sqlstore != nil {
		if err := a.sqlstore.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("close whatsmeow sqlstore: %w", err))
		}
	}
	if a.database != nil {
		a.database.Close()
	}

	if result != nil {
		return result
	}
	a.logger.Info().Msg("shutdown_completed")
	return nil
}
