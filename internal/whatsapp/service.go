package whatsapp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"whatsapp-go-api/internal/config"
	"whatsapp-go-api/internal/database/repository"
	"whatsapp-go-api/internal/database/types"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

type QRCodeConnectionResult struct {
	Count             int
	Code              string
	Base64            string
	InstanceName      string
	ConnectionStatus  string
	AlreadyConnected  bool
	AlreadyConnecting bool
	OwnerJid          *string
}

type PhonePairingResult struct {
	Code string
}

type ConnectionStateResult struct {
	State            string
	StatusReason     int
	InstanceName     string
	ConnectionStatus string
	Connected        bool
	LoggedIn         bool
	OwnerJid         *string
}

type LogoutResult struct {
	InstanceName     string
	State            string
	ConnectionStatus string
	Message          string
}

type DeleteResult struct {
	InstanceName string
	Deleted      bool
	Forced       bool
	Message      string
}

type ConnectionService interface {
	ConnectQRCode(ctx context.Context, instanceName string, bearerToken string) (QRCodeConnectionResult, error)
	ConnectPhone(ctx context.Context, instanceName string, bearerToken string, phoneNumber string) (PhonePairingResult, error)
	RequestPasskeyChallenge(ctx context.Context, instanceName string, bearerToken string) (PasskeyChallengeResult, error)
	SubmitPasskeyAssertion(ctx context.Context, instanceName string, bearerToken string, request SubmitPasskeyAssertionRequest) (PasskeyAssertionResult, error)
	ConnectionState(ctx context.Context, instanceName string, bearerToken string) (ConnectionStateResult, error)
	Logout(ctx context.Context, instanceName string, bearerToken string) (LogoutResult, error)
	DeleteInstance(ctx context.Context, instanceName string, bearerToken string, force bool) (DeleteResult, error)
	ResolveConnectedClient(ctx context.Context, instanceName string) (*ManagedWhatsAppClient, error)
	Restore(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Service struct {
	config             config.WhatsAppConfig
	instances          repository.InstanceRepository
	factory            WhatsAppClientFactory
	hub                ClientHub
	lock               InstanceConnectionLock
	events             *EventPersistenceService
	webhooks           webhooksvc.WebhookManager
	qr                 QRGenerator
	logger             zerolog.Logger
	passkeyClient      func(*ManagedWhatsAppClient) PasskeyClient
	appCtx             context.Context
	appCancel          context.CancelFunc
	pairings           *pairingManager
	contactSyncMu      sync.Mutex
	contactSyncCancels map[string]*contactSyncHandle
}

type contactSyncHandle struct {
	cancel context.CancelFunc
}

func NewService(
	cfg config.WhatsAppConfig,
	instances repository.InstanceRepository,
	factory WhatsAppClientFactory,
	hub ClientHub,
	lock InstanceConnectionLock,
	events *EventPersistenceService,
	webhooks webhooksvc.WebhookManager,
	logger zerolog.Logger,
) (*Service, error) {
	qr, err := NewQRGenerator(cfg.QRCodeLightColor, cfg.QRCodeDarkColor)
	if err != nil {
		return nil, err
	}
	appCtx, appCancel := context.WithCancel(context.Background())
	return &Service{
		config:             cfg,
		instances:          instances,
		factory:            factory,
		hub:                hub,
		lock:               lock,
		events:             events,
		webhooks:           webhooks,
		qr:                 qr,
		logger:             logger.With().Str("component", "whatsapp_service").Logger(),
		passkeyClient:      newWhatsmeowPasskeyClient,
		appCtx:             appCtx,
		appCancel:          appCancel,
		pairings:           newPairingManager(),
		contactSyncCancels: make(map[string]*contactSyncHandle),
	}, nil
}

func (s *Service) ConnectQRCode(ctx context.Context, instanceName string, bearerToken string) (QRCodeConnectionResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return QRCodeConnectionResult{}, err
	}
	instanceID := strconv.Itoa(int(instance.Instance.ID))
	locked, err := s.lock.TryAcquire(ctx, instanceID)
	if err != nil {
		return QRCodeConnectionResult{}, err
	}
	if !locked {
		return QRCodeConnectionResult{
			InstanceName:      instance.Instance.Name,
			ConnectionStatus:  string(types.InstanceConnectionStatusConnecting),
			AlreadyConnecting: true,
		}, nil
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_ = s.lock.Release(context.Background(), instanceID)
		}
	}()

	if result, handled, err := s.connectExistingSession(ctx, instance.Instance, "connect_existing_session"); handled || err != nil {
		if err == nil {
			if result.AlreadyConnected {
				_ = s.lock.Release(context.Background(), instanceID)
				lockHeld = false
			} else {
				lockHeld = false
			}
		}
		return result, err
	}

	if s.pairings.exists(instanceID) {
		return QRCodeConnectionResult{
			InstanceName:      instance.Instance.Name,
			ConnectionStatus:  string(types.InstanceConnectionStatusConnecting),
			AlreadyConnecting: true,
		}, nil
	}

	if err := s.hub.Reserve(instanceID, instance.Instance.Name); err != nil {
		return QRCodeConnectionResult{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			s.hub.Remove(instanceID)
		}
	}()

	now := time.Now().UTC()
	connecting := types.InstanceConnectionStatusConnecting
	event := "connect_qr_start"
	if err := s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:              instance.Instance.ID,
		ConnectionStatus:        &connecting,
		LastConnectionAttemptAt: &now,
		LastConnectionEvent:     types.OptionalField[string]{Set: true, Value: &event},
		IncrementAttempts:       true,
	}); err != nil {
		return QRCodeConnectionResult{}, err
	}

	client, err := s.newClientForManualConnect(ctx, instance.Instance)
	if err != nil {
		return QRCodeConnectionResult{}, err
	}
	if client.Store.ID != nil {
		s.hub.Remove(instanceID)
		reserved = false
		managed := s.newManagedClient(instance.Instance, client, context.Background(), func() {})
		if err := s.validateManagedDevice(instance.Instance, managed); err != nil {
			return QRCodeConnectionResult{}, err
		}
		if result, err := s.registerAndConnectExistingSession(ctx, instance.Instance, managed, "connect_existing_session"); err != nil {
			return QRCodeConnectionResult{}, err
		} else {
			lockHeld = false
			return result, nil
		}
	}

	pairingCtx, pairingCancel := context.WithTimeout(s.appCtx, s.config.MaximumPairingTime())
	session := &pairingSession{
		cancel:    pairingCancel,
		ctx:       pairingCtx,
		startedAt: now,
	}
	if !s.pairings.add(instanceID, session) {
		pairingCancel()
		return QRCodeConnectionResult{}, ErrConnectionInProgress
	}
	sessionRegistered := true
	defer func() {
		if sessionRegistered {
			s.pairings.remove(instanceID, session)
		}
	}()

	managed := s.newManagedClient(instance.Instance, client, pairingCtx, pairingCancel)
	s.registerEventHandlers(managed, pairingCancel)

	qrChannel, err := client.GetQRChannel(pairingCtx)
	if err != nil {
		pairingCancel()
		s.markConnectionError(ctx, instance.Instance.ID, types.InstanceConnectionStatusConnectionError, "qr_channel_error")
		if errors.Is(err, whatsmeow.ErrQRStoreContainsID) || errors.Is(err, whatsmeow.ErrQRAlreadyConnected) {
			return QRCodeConnectionResult{}, fmt.Errorf("%w: %w", ErrInstanceConnected, err)
		}
		return QRCodeConnectionResult{}, fmt.Errorf("%w: get QR channel: %w", ErrWhatsAppUnavailable, err)
	}
	if err := s.hub.Register(managed); err != nil {
		pairingCancel()
		return QRCodeConnectionResult{}, err
	}
	reserved = false
	lockHeld = false

	firstQRCode := make(chan QRCodeConnectionResult, 1)
	firstError := make(chan error, 1)
	go s.consumeQRCodeChannel(pairingCtx, pairingCancel, session, managed, qrChannel, firstQRCode, firstError)

	if err := client.Connect(); err != nil {
		pairingCancel()
		s.markConnectionError(ctx, instance.Instance.ID, types.InstanceConnectionStatusConnectionError, "connect_error")
		return QRCodeConnectionResult{}, fmt.Errorf("%w: connect: %w", ErrWhatsAppUnavailable, err)
	}

	select {
	case result := <-firstQRCode:
		sessionRegistered = false
		return result, nil
	case err := <-firstError:
		return QRCodeConnectionResult{}, err
	case <-ctx.Done():
		s.logger.Debug().Str("instance_id", instanceID).Str("instance_name", instance.Instance.Name).Err(ctx.Err()).Msg("HTTP QR request context finished before first QR")
		sessionRegistered = false
		return QRCodeConnectionResult{}, ctx.Err()
	}
}

func (s *Service) ConnectPhone(ctx context.Context, instanceName string, bearerToken string, phoneNumber string) (PhonePairingResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return PhonePairingResult{}, err
	}
	phone, err := NormalizePhoneNumber(phoneNumber)
	if err != nil {
		return PhonePairingResult{}, err
	}
	instanceID := strconv.Itoa(int(instance.Instance.ID))
	if s.hub.Exists(instanceID) {
		return PhonePairingResult{}, ErrConnectionInProgress
	}
	locked, err := s.lock.TryAcquire(ctx, instanceID)
	if err != nil {
		return PhonePairingResult{}, err
	}
	if !locked {
		return PhonePairingResult{}, ErrConnectionInProgress
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_ = s.lock.Release(context.Background(), instanceID)
		}
	}()
	if err := s.hub.Reserve(instanceID, instance.Instance.Name); err != nil {
		return PhonePairingResult{}, err
	}
	reserved := true
	defer func() {
		if reserved {
			s.hub.Remove(instanceID)
		}
	}()

	client, err := s.newClientForManualConnect(ctx, instance.Instance)
	if err != nil {
		return PhonePairingResult{}, err
	}
	instanceCtx, instanceCancel := context.WithCancel(context.Background())
	pairingCtx, pairingCancel := context.WithTimeout(instanceCtx, s.config.MaximumPairingTime())
	managed := s.newManagedClient(instance.Instance, client, instanceCtx, instanceCancel)
	s.registerEventHandlers(managed, pairingCancel)

	now := time.Now().UTC()
	connecting := types.InstanceConnectionStatusConnecting
	event := "connect_phone_start"
	if err := s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:              instance.Instance.ID,
		ConnectionStatus:        &connecting,
		LastConnectionAttemptAt: &now,
		LastConnectionEvent:     types.OptionalField[string]{Set: true, Value: &event},
		IncrementAttempts:       true,
	}); err != nil {
		pairingCancel()
		return PhonePairingResult{}, err
	}
	if err := client.Connect(); err != nil {
		pairingCancel()
		s.markConnectionError(ctx, instance.Instance.ID, types.InstanceConnectionStatusConnectionError, "connect_error")
		return PhonePairingResult{}, fmt.Errorf("%w: connect: %w", ErrWhatsAppUnavailable, err)
	}
	code, err := client.PairPhone(pairingCtx, phone, false, whatsmeow.PairClientMacOS, "Chrome (Linux)")
	if err != nil {
		pairingCancel()
		client.Disconnect()
		s.markConnectionError(ctx, instance.Instance.ID, types.InstanceConnectionStatusConnectionError, "pair_phone_error")
		return PhonePairingResult{}, fmt.Errorf("%w: pair phone: %w", ErrWhatsAppUnavailable, err)
	}
	status := types.InstanceConnectionStatusPairingCode
	event = "pairing_code"
	_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:          instance.Instance.ID,
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	})
	if err := s.hub.Register(managed); err != nil {
		pairingCancel()
		client.Disconnect()
		return PhonePairingResult{}, err
	}
	reserved = false
	lockHeld = false
	s.logger.Info().Str("instance_id", instanceID).Str("instance_name", instance.Instance.Name).Str("connection_mode", "phone").Str("phone_masked", MaskPhone(phone)).Msg("phone pairing code generated")
	return PhonePairingResult{Code: code}, nil
}

func (s *Service) ResolveConnectedClient(ctx context.Context, instanceName string) (*ManagedWhatsAppClient, error) {
	instance, err := s.instances.FindByName(ctx, strings.TrimSpace(instanceName))
	if err != nil {
		return nil, err
	}
	client, ok := s.hub.GetByInstanceID(strconv.Itoa(int(instance.Instance.ID)))
	if !ok || client == nil || !client.IsReady() {
		return nil, ErrClientNotConnected
	}
	return client, nil
}

func (s *Service) ConnectionState(ctx context.Context, instanceName string, bearerToken string) (ConnectionStateResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return ConnectionStateResult{}, err
	}

	instanceID := strconv.Itoa(int(instance.Instance.ID))
	client, ok := s.hub.GetByInstanceID(instanceID)
	if !ok || client == nil || client.Client == nil {
		return ConnectionStateResult{
			State:            "close",
			StatusReason:     503,
			InstanceName:     instance.Instance.Name,
			ConnectionStatus: string(instance.Instance.ConnectionStatus),
			Connected:        false,
			LoggedIn:         false,
			OwnerJid:         nil,
		}, nil
	}

	connected := client.Client.IsConnected()
	loggedIn := client.Client.IsLoggedIn()
	ownerJID := ownerJIDFromClient(client)
	state := apiState(instance.Instance.ConnectionStatus, connected, loggedIn, client.Client.Store.ID != nil)
	statusReason := 200
	if state == "close" {
		statusReason = 503
	}
	return ConnectionStateResult{
		State:            state,
		StatusReason:     statusReason,
		InstanceName:     instance.Instance.Name,
		ConnectionStatus: string(instance.Instance.ConnectionStatus),
		Connected:        connected,
		LoggedIn:         loggedIn,
		OwnerJid:         ownerJID,
	}, nil
}

func (s *Service) Logout(ctx context.Context, instanceName string, bearerToken string) (LogoutResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return LogoutResult{}, err
	}
	if err := s.logoutInstance(ctx, instance.Instance); err != nil {
		return LogoutResult{}, err
	}
	s.logger.Info().
		Str("operation", "instance.logout").
		Str("instanceName", instance.Instance.Name).
		Int32("instanceId", instance.Instance.ID).
		Msg("instance logged out")
	return LogoutResult{
		InstanceName:     instance.Instance.Name,
		State:            "logged_out",
		ConnectionStatus: "LOGGED_OUT",
		Message:          "Instance logged out successfully",
	}, nil
}

func (s *Service) DeleteInstance(ctx context.Context, instanceName string, bearerToken string, force bool) (DeleteResult, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return DeleteResult{}, err
	}
	if !force {
		if err := s.instances.EnsureDeletable(ctx, instance.Instance.ID); err != nil {
			return DeleteResult{}, err
		}
	}
	if err := s.cleanupWhatsAppClient(ctx, instance.Instance); err != nil {
		return DeleteResult{}, err
	}
	if err := s.instances.Delete(ctx, instance.Instance.ID, force); err != nil {
		return DeleteResult{}, err
	}
	s.deleteCachedWebhook(instance.Instance.ID, instance.Instance.Name)
	message := "Instance deleted successfully"
	if force {
		message = "Instance and related data deleted successfully"
	}
	s.logger.Info().
		Str("operation", "instance.delete").
		Str("instanceName", instance.Instance.Name).
		Int32("instanceId", instance.Instance.ID).
		Bool("forced", force).
		Msg("instance deleted")
	return DeleteResult{
		InstanceName: instance.Instance.Name,
		Deleted:      true,
		Forced:       force,
		Message:      message,
	}, nil
}

func (s *Service) deleteCachedWebhook(instanceID int32, instanceName string) {
	type cacheInvalidator interface {
		DeleteCachedWebhook(instanceID int64, instanceName string)
	}
	if invalidator, ok := s.webhooks.(cacheInvalidator); ok {
		invalidator.DeleteCachedWebhook(int64(instanceID), instanceName)
	}
}

func (s *Service) Restore(ctx context.Context) error {
	if !s.config.AutoReconnect {
		return nil
	}
	instances, err := s.instances.FindAutoConnectInstances(ctx)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, s.config.StartupReconnectConcurrency)
	done := make(chan struct{}, len(instances))
	for _, item := range instances {
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
				done <- struct{}{}
			}()
			if err := s.restoreOne(ctx, item); err != nil {
				s.logger.Error().Err(err).Int32("instance_id", item.ID).Str("instance_name", item.Name).Msg("failed to restore WhatsApp client")
			}
		}()
	}
	for range instances {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.appCancel != nil {
		s.appCancel()
	}
	if s.pairings != nil {
		s.pairings.cancelAll()
	}
	s.cancelAllContactSyncs()
	for _, client := range s.hub.List() {
		_ = s.lock.Release(ctx, client.InstanceID)
	}
	return s.hub.Shutdown(ctx)
}

func (s *Service) restoreOne(ctx context.Context, item types.Instance) error {
	if item.WhatsAppDeviceJid == nil {
		status := types.InstanceConnectionStatusSessionMissing
		event := "session_missing"
		return s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
			InstanceID:          item.ID,
			ConnectionStatus:    &status,
			LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		})
	}
	instanceID := strconv.Itoa(int(item.ID))
	locked, err := s.lock.TryAcquire(ctx, instanceID)
	if err != nil || !locked {
		return err
	}
	if err := s.hub.Reserve(instanceID, item.Name); err != nil {
		_ = s.lock.Release(ctx, instanceID)
		if existing, ok := s.hub.GetByInstanceID(instanceID); ok && existing != nil {
			return s.reconcileInstanceState(ctx, item, existing)
		}
		if errors.Is(err, ErrConnectionInProgress) {
			return nil
		}
		return err
	}
	client, err := s.factory.ClientForDevice(ctx, *item.WhatsAppDeviceJid)
	if err != nil {
		s.hub.Remove(instanceID)
		_ = s.lock.Release(ctx, instanceID)
		status := types.InstanceConnectionStatusSessionMissing
		event := "session_missing"
		_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
			InstanceID:          item.ID,
			ConnectionStatus:    &status,
			LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		})
		return err
	}
	instanceCtx, instanceCancel := context.WithCancel(context.Background())
	connectCtx, connectCancel := context.WithTimeout(instanceCtx, s.config.ConnectTimeout)
	defer connectCancel()
	managed := s.newManagedClient(item, client, instanceCtx, instanceCancel)
	if err := s.validateManagedDevice(item, managed); err != nil {
		s.hub.Remove(instanceID)
		_ = s.lock.Release(ctx, instanceID)
		status := types.InstanceConnectionStatusConnectionError
		event := "device_mismatch"
		_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
			InstanceID:          item.ID,
			ConnectionStatus:    &status,
			LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		})
		return err
	}
	s.registerEventHandlers(managed, connectCancel)
	if err := s.hub.Register(managed); err != nil {
		_ = s.lock.Release(ctx, instanceID)
		return err
	}
	if err := s.reconcileInstanceState(ctx, item, managed); err != nil {
		s.hub.Remove(instanceID)
		_ = s.lock.Release(ctx, instanceID)
		return err
	}
	if client.IsConnected() {
		_ = s.lock.Release(ctx, instanceID)
		return nil
	}
	if client.Store == nil || client.Store.ID == nil {
		_ = s.lock.Release(ctx, instanceID)
		return nil
	}
	status := types.InstanceConnectionStatusConnecting
	now := time.Now().UTC()
	event := "restore_start"
	_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:              item.ID,
		ConnectionStatus:        &status,
		LastConnectionAttemptAt: &now,
		LastConnectionEvent:     types.OptionalField[string]{Set: true, Value: &event},
		IncrementAttempts:       true,
	})
	if err := client.Connect(); err != nil {
		if errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			_ = s.lock.Release(ctx, instanceID)
			return s.reconcileInstanceState(ctx, item, managed)
		}
		s.hub.Remove(instanceID)
		_ = s.lock.Release(ctx, instanceID)
		status := types.InstanceConnectionStatusConnectionError
		event := "restore_connect_error"
		_ = s.instances.UpdateConnectionState(context.Background(), types.UpdateConnectionStateInput{
			InstanceID:          item.ID,
			ConnectionStatus:    &status,
			LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		})
		return err
	}
	select {
	case <-managed.ConnectedSignal:
		return nil
	case <-connectCtx.Done():
		if client.IsConnected() {
			return nil
		}
		status := types.InstanceConnectionStatusConnectionTimeout
		event := "restore_timeout"
		_ = s.instances.UpdateConnectionState(context.Background(), types.UpdateConnectionStateInput{
			InstanceID:          item.ID,
			ConnectionStatus:    &status,
			LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		})
		return ErrQRCodeTimeout
	}
}

func (s *Service) connectExistingSession(ctx context.Context, item types.Instance, event string) (QRCodeConnectionResult, bool, error) {
	instanceID := strconv.Itoa(int(item.ID))
	if managed, ok := s.hub.GetByInstanceID(instanceID); ok && managed != nil && managed.Client != nil {
		if err := s.validateManagedDevice(item, managed); err != nil {
			return QRCodeConnectionResult{}, true, err
		}
		if managed.Client.IsConnected() && managed.Client.IsLoggedIn() {
			if err := s.reconcileInstanceState(ctx, item, managed); err != nil {
				return QRCodeConnectionResult{}, true, err
			}
			return QRCodeConnectionResult{
				InstanceName:     item.Name,
				ConnectionStatus: string(types.InstanceConnectionStatusOnline),
				AlreadyConnected: true,
				OwnerJid:         ownerJIDFromClient(managed),
			}, true, nil
		}
		if managed.Client.Store != nil && managed.Client.Store.ID != nil {
			result, err := s.connectManagedSession(ctx, item, managed, event)
			return result, true, err
		}
	}
	if item.WhatsAppDeviceJid == nil || strings.TrimSpace(*item.WhatsAppDeviceJid) == "" {
		return QRCodeConnectionResult{}, false, nil
	}
	client, err := s.factory.ClientForDevice(ctx, *item.WhatsAppDeviceJid)
	if err != nil {
		if errors.Is(err, ErrSessionMissing) {
			status := types.InstanceConnectionStatusSessionMissing
			name := "session_missing"
			_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
				InstanceID:          item.ID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
			})
		}
		return QRCodeConnectionResult{}, true, err
	}
	instanceCtx, instanceCancel := context.WithCancel(context.Background())
	managed := s.newManagedClient(item, client, instanceCtx, instanceCancel)
	if err := s.validateManagedDevice(item, managed); err != nil {
		instanceCancel()
		return QRCodeConnectionResult{}, true, err
	}
	_, connectCancel := context.WithTimeout(instanceCtx, s.config.ConnectTimeout)
	s.registerEventHandlers(managed, connectCancel)
	if err := s.hub.Register(managed); err != nil {
		instanceCancel()
		connectCancel()
		return QRCodeConnectionResult{}, true, err
	}
	result, err := s.connectManagedSession(ctx, item, managed, event)
	if err != nil {
		return QRCodeConnectionResult{}, true, err
	}
	return result, true, nil
}

func (s *Service) registerAndConnectExistingSession(ctx context.Context, item types.Instance, managed *ManagedWhatsAppClient, event string) (QRCodeConnectionResult, error) {
	instanceCtx, instanceCancel := context.WithCancel(context.Background())
	_, connectCancel := context.WithTimeout(instanceCtx, s.config.ConnectTimeout)
	managed.Context = instanceCtx
	managed.Cancel = instanceCancel
	s.registerEventHandlers(managed, connectCancel)
	if err := s.hub.Register(managed); err != nil {
		instanceCancel()
		connectCancel()
		return QRCodeConnectionResult{}, err
	}
	return s.connectManagedSession(ctx, item, managed, event)
}

func (s *Service) connectManagedSession(ctx context.Context, item types.Instance, managed *ManagedWhatsAppClient, event string) (QRCodeConnectionResult, error) {
	if managed.Client.IsConnected() && managed.Client.IsLoggedIn() {
		if err := s.reconcileInstanceState(ctx, item, managed); err != nil {
			return QRCodeConnectionResult{}, err
		}
		return QRCodeConnectionResult{
			InstanceName:     item.Name,
			ConnectionStatus: string(types.InstanceConnectionStatusOnline),
			AlreadyConnected: true,
			OwnerJid:         ownerJIDFromClient(managed),
		}, nil
	}
	now := time.Now().UTC()
	status := types.InstanceConnectionStatusConnecting
	_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:              item.ID,
		ConnectionStatus:        &status,
		LastConnectionAttemptAt: &now,
		LastConnectionEvent:     types.OptionalField[string]{Set: true, Value: &event},
		IncrementAttempts:       true,
	})
	if event == "connect_existing_session" {
		s.dispatchConnectionWebhook(ctx, managed, webhooksvc.ConnectionInternalManualLoginReconnect, 0, nil, "")
	}
	if err := managed.Client.Connect(); err != nil {
		if errors.Is(err, whatsmeow.ErrAlreadyConnected) {
			if err := s.reconcileInstanceState(ctx, item, managed); err != nil {
				return QRCodeConnectionResult{}, err
			}
			return QRCodeConnectionResult{
				InstanceName:     item.Name,
				ConnectionStatus: string(types.InstanceConnectionStatusOnline),
				AlreadyConnected: true,
				OwnerJid:         ownerJIDFromClient(managed),
			}, nil
		}
		s.markConnectionError(ctx, item.ID, types.InstanceConnectionStatusConnectionError, "connect_error")
		s.dispatchConnectionWebhook(ctx, managed, webhooksvc.ConnectionInternalConnectFailure, 0, nil, err.Error())
		return QRCodeConnectionResult{}, fmt.Errorf("%w: connect: %w", ErrWhatsAppUnavailable, err)
	}
	return QRCodeConnectionResult{
		InstanceName:      item.Name,
		ConnectionStatus:  string(types.InstanceConnectionStatusConnecting),
		AlreadyConnecting: true,
		OwnerJid:          ownerJIDFromClient(managed),
	}, nil
}

func (s *Service) reconcileInstanceState(ctx context.Context, item types.Instance, managed *ManagedWhatsAppClient) error {
	status := managedConnectionStatus(managed)
	event := "reconcile"
	input := types.UpdateConnectionStateInput{
		InstanceID:          item.ID,
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	}
	if status == types.InstanceConnectionStatusOnline {
		now := time.Now().UTC()
		input.LastConnectedAt = &now
		input.ResetAttempts = true
		empty := ""
		input.LastConnectionError = types.OptionalField[string]{Set: true, Value: &empty}
	}
	return s.instances.UpdateConnectionState(ctx, input)
}

func managedConnectionStatus(managed *ManagedWhatsAppClient) types.InstanceConnectionStatus {
	if managed == nil || managed.Client == nil || managed.Client.Store == nil || managed.Client.Store.ID == nil {
		return types.InstanceConnectionStatusSessionMissing
	}
	if managed.Client.IsConnected() && managed.Client.IsLoggedIn() {
		return types.InstanceConnectionStatusOnline
	}
	return types.InstanceConnectionStatusDisconnected
}

func (s *Service) validateManagedDevice(item types.Instance, managed *ManagedWhatsAppClient) error {
	if managed == nil || managed.Client == nil || managed.Client.Store == nil || managed.Client.Store.ID == nil {
		return nil
	}
	deviceJID := managed.Client.Store.ID.String()
	ownerJID := managed.Client.Store.ID.ToNonAD().String()
	if item.WhatsAppDeviceJid != nil && strings.TrimSpace(*item.WhatsAppDeviceJid) != "" && *item.WhatsAppDeviceJid != deviceJID {
		s.logger.Error().
			Int32("instanceId", item.ID).
			Str("instanceName", item.Name).
			Str("ownerJid", ownerJID).
			Str("deviceJid", deviceJID).
			Str("expectedDeviceJid", *item.WhatsAppDeviceJid).
			Msg("whatsapp device mismatch")
		return ErrDeviceMismatch
	}
	if item.WhatsAppOwnerJid != nil && strings.TrimSpace(*item.WhatsAppOwnerJid) != "" && *item.WhatsAppOwnerJid != ownerJID {
		s.logger.Error().
			Int32("instanceId", item.ID).
			Str("instanceName", item.Name).
			Str("ownerJid", ownerJID).
			Str("deviceJid", deviceJID).
			Str("expectedOwnerJid", *item.WhatsAppOwnerJid).
			Msg("whatsapp owner mismatch")
		return ErrDeviceMismatch
	}
	if item.OwnerJid != nil && strings.TrimSpace(*item.OwnerJid) != "" && *item.OwnerJid != ownerJID {
		s.logger.Error().
			Int32("instanceId", item.ID).
			Str("instanceName", item.Name).
			Str("ownerJid", ownerJID).
			Str("deviceJid", deviceJID).
			Str("expectedOwnerJid", *item.OwnerJid).
			Msg("legacy owner jid mismatch")
		return ErrDeviceMismatch
	}
	managed.mu.Lock()
	managed.DeviceJID = deviceJID
	managed.OwnerJID = ownerJID
	managed.mu.Unlock()
	return nil
}

func (s *Service) authenticateInstance(ctx context.Context, instanceName string, bearerToken string) (types.InstanceWithAuth, error) {
	instanceName = strings.TrimSpace(instanceName)
	bearerToken = strings.TrimSpace(bearerToken)
	if instanceName == "" || bearerToken == "" {
		return types.InstanceWithAuth{}, ErrInvalidInstanceToken
	}
	instance, err := s.instances.FindByName(ctx, instanceName)
	if err != nil {
		return types.InstanceWithAuth{}, err
	}
	if instance.Auth == nil || subtle.ConstantTimeCompare([]byte(instance.Auth.Token), []byte(bearerToken)) != 1 {
		return types.InstanceWithAuth{}, ErrInvalidInstanceToken
	}
	if instance.Instance.Status != types.InstanceStatusOnline {
		return types.InstanceWithAuth{}, ErrInstanceInactive
	}
	return instance, nil
}

func (s *Service) logoutInstance(ctx context.Context, item types.Instance) error {
	if err := s.cleanupWhatsAppClient(ctx, item); err != nil {
		return err
	}

	status := types.InstanceConnectionStatusLoggedOut
	now := time.Now().UTC()
	event := "logged_out"
	if err := s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:          item.ID,
		ConnectionStatus:    &status,
		LastDisconnectedAt:  &now,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		ResetAttempts:       true,
	}); err != nil {
		return err
	}
	if err := s.instances.ClearWhatsAppDevice(ctx, item.ID); err != nil && !errors.Is(err, repository.ErrInstanceNotFound) {
		return err
	}
	_ = s.instances.UpdateStatus(ctx, item.ID, types.InstanceStatusOffline)
	return nil
}

func (s *Service) cleanupWhatsAppClient(ctx context.Context, item types.Instance) error {
	instanceID := strconv.Itoa(int(item.ID))
	locked, err := s.lock.TryAcquire(ctx, instanceID)
	if err != nil {
		return err
	}
	if !locked {
		return ErrConnectionInProgress
	}
	defer func() {
		_ = s.lock.Release(context.Background(), instanceID)
	}()

	managed, ok := s.hub.Remove(instanceID)
	if ok && managed != nil {
		if managed.Cancel != nil {
			managed.Cancel()
		}
		if managed.Client != nil {
			return s.logoutOrDeleteStore(ctx, managed.Client)
		}
		return nil
	}

	if item.WhatsAppDeviceJid == nil || strings.TrimSpace(*item.WhatsAppDeviceJid) == "" {
		return nil
	}
	client, err := s.factory.ClientForDevice(ctx, *item.WhatsAppDeviceJid)
	if err != nil {
		if errors.Is(err, ErrSessionMissing) {
			return nil
		}
		return err
	}
	return s.logoutOrDeleteStore(ctx, client)
}

func (s *Service) logoutOrDeleteStore(ctx context.Context, client *whatsmeow.Client) error {
	if client == nil {
		return nil
	}
	if client.IsConnected() && client.IsLoggedIn() {
		if err := client.Logout(ctx); err != nil {
			if !errors.Is(err, whatsmeow.ErrNotLoggedIn) {
				return fmt.Errorf("%w: logout: %w", ErrWhatsAppUnavailable, err)
			}
		} else {
			return nil
		}
	}
	client.Disconnect()
	if client.Store != nil && client.Store.ID != nil {
		if err := client.Store.Delete(ctx); err != nil {
			return fmt.Errorf("%w: delete store: %w", ErrWhatsAppUnavailable, err)
		}
	}
	return nil
}

func ownerJIDFromClient(client *ManagedWhatsAppClient) *string {
	if client == nil || client.Client == nil || client.Client.Store == nil || client.Client.Store.ID == nil {
		return nil
	}
	owner := client.Client.Store.ID.ToNonAD().String()
	return &owner
}

func apiState(status types.InstanceConnectionStatus, connected bool, loggedIn bool, hasDevice bool) string {
	if status == types.InstanceConnectionStatusLoggedOut || status == types.InstanceConnectionStatusSessionMissing {
		return "logged_out"
	}
	if connected && loggedIn && hasDevice {
		return "open"
	}
	switch status {
	case types.InstanceConnectionStatusConnecting,
		types.InstanceConnectionStatusQRCode,
		types.InstanceConnectionStatusPairingCode,
		types.InstanceConnectionStatusPairing,
		types.InstanceConnectionStatusReconnecting:
		return "connecting"
	default:
		return "close"
	}
}

func (s *Service) newClientForManualConnect(ctx context.Context, item types.Instance) (*whatsmeow.Client, error) {
	if item.WhatsAppDeviceJid != nil && strings.TrimSpace(*item.WhatsAppDeviceJid) != "" {
		return s.factory.ClientForDevice(ctx, *item.WhatsAppDeviceJid)
	}
	return s.factory.NewDeviceClient()
}

func (s *Service) newManagedClient(item types.Instance, client *whatsmeow.Client, ctx context.Context, cancel context.CancelFunc) *ManagedWhatsAppClient {
	return &ManagedWhatsAppClient{
		InstanceID:      strconv.Itoa(int(item.ID)),
		InstanceName:    item.Name,
		Client:          client,
		Context:         ctx,
		Cancel:          cancel,
		Status:          string(item.ConnectionStatus),
		StartedAt:       time.Now().UTC(),
		ConnectedSignal: make(chan struct{}),
	}
}

func (s *Service) registerEventHandlers(managed *ManagedWhatsAppClient, pairingCancel context.CancelFunc) {
	managed.Client.AddEventHandler(func(evt any) {
		instanceID := mustAtoi32(managed.InstanceID)
		switch event := evt.(type) {
		case *events.PairSuccess:
			eventCtx := context.Background()
			s.persistDevice(eventCtx, managed, "pair_success")
			status := types.InstanceConnectionStatusPairing
			name := "pair_success"
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalPairSuccess, 0, nil, "")
			_ = event
		case *events.PairError:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusConnectionError
			name := "pair_error"
			message := errorString(event.Error)
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				LastConnectionError: types.OptionalField[string]{Set: true, Value: &message},
				IncrementAttempts:   true,
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalPairError, 0, nil, message)
		case *events.Connected:
			if managed.Client.IsConnected() && managed.Client.IsLoggedIn() && managed.Client.Store.ID != nil {
				eventCtx := context.Background()
				pairingCancel()
				s.persistDevice(eventCtx, managed, "connected")
				status := types.InstanceConnectionStatusOnline
				now := time.Now().UTC()
				name := "connected"
				empty := ""
				s.logger.Info().Str("status", name).Msg(managed.InstanceName)
				_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
					InstanceID:          instanceID,
					ConnectionStatus:    &status,
					LastConnectedAt:     &now,
					LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
					LastConnectionError: types.OptionalField[string]{Set: true, Value: &empty},
					ResetAttempts:       true,
				})
				s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalConnected, 0, &now, "")
				managed.SignalConnected()
				_ = s.lock.Release(eventCtx, managed.InstanceID)
				go s.refreshProfilePicture(eventCtx, managed)
				s.startContactSync(managed)
			}
		case *events.Disconnected:
			eventCtx := context.Background()
			s.cancelContactSync(managed.InstanceID)
			status := types.InstanceConnectionStatusReconnecting
			now := time.Now().UTC()
			name := "disconnected"
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastDisconnectedAt:  &now,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				IncrementAttempts:   true,
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalDisconnected, 0, nil, "")
		case *events.LoggedOut:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusLoggedOut
			now := time.Now().UTC()
			name := "logged_out"
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastDisconnectedAt:  &now,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
			})
			_ = s.instances.ClearWhatsAppDevice(eventCtx, instanceID)
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalLoggedOut, int(event.Reason), nil, event.Reason.String())
			managed.Cancel()
			s.cancelContactSync(managed.InstanceID)
			s.hub.Remove(managed.InstanceID)
			_ = s.lock.Release(eventCtx, managed.InstanceID)
		case *events.StreamReplaced:
			s.removePermanent(managed, types.InstanceConnectionStatusStreamReplaced, "stream_replaced")
			s.dispatchConnectionWebhook(context.Background(), managed, webhooksvc.ConnectionInternalStreamReplaced, 0, nil, "")
		case *events.KeepAliveTimeout:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusKeepAliveTimeout
			name := "keepalive_timeout"
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				IncrementAttempts:   true,
			})
			var lastSuccess *time.Time
			if !event.LastSuccess.IsZero() {
				lastSuccess = &event.LastSuccess
			}
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalKeepAliveTimeout, event.ErrorCount, lastSuccess, "")
		case *events.KeepAliveRestored:
			if managed.Client.IsConnected() && managed.Client.IsLoggedIn() {
				eventCtx := context.Background()
				status := types.InstanceConnectionStatusOnline
				name := "keepalive_restored"
				_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
					InstanceID:          instanceID,
					ConnectionStatus:    &status,
					LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				})
				now := time.Now().UTC()
				s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalKeepAliveRestored, 0, &now, "")
			}
		case *events.ConnectFailure:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusConnectionError
			name := "connect_failure"
			message := firstNonEmpty(event.Message, event.Reason.String())
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				LastConnectionError: types.OptionalField[string]{Set: true, Value: &message},
				IncrementAttempts:   true,
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalConnectFailure, int(event.Reason), nil, message)
		case *events.StreamError:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusConnectionError
			name := "stream_error"
			message := firstNonEmpty(event.Code, "unknown stream error")
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				LastConnectionError: types.OptionalField[string]{Set: true, Value: &message},
				IncrementAttempts:   true,
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalStreamError, 0, nil, message)
		case *events.CATRefreshError:
			message := errorString(event.Error)
			s.removePermanent(managed, types.InstanceConnectionStatusConnectionError, "cat_refresh_error")
			s.dispatchConnectionWebhook(context.Background(), managed, webhooksvc.ConnectionInternalCATRefreshError, 0, nil, message)
		case *events.ManualLoginReconnect:
			eventCtx := context.Background()
			status := types.InstanceConnectionStatusConnecting
			name := "manual_reconnect"
			_ = s.instances.UpdateConnectionState(eventCtx, types.UpdateConnectionStateInput{
				InstanceID:          instanceID,
				ConnectionStatus:    &status,
				LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &name},
				IncrementAttempts:   true,
			})
			s.dispatchConnectionWebhook(eventCtx, managed, webhooksvc.ConnectionInternalManualLoginReconnect, 0, nil, "")
		case *events.ClientOutdated:
			s.removePermanent(managed, types.InstanceConnectionStatusClientOutdated, "client_outdated")
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalClientOutdated, "client_outdated", "client is out of date", nil)
		case *events.TemporaryBan:
			s.removePermanent(managed, types.InstanceConnectionStatusTemporaryBan, "temporary_ban")
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalTemporaryBan, "temporary_ban", event.String(), map[string]any{
				"code":          int(event.Code),
				"expireSeconds": int(event.Expire.Seconds()),
			})
		case *events.Picture:
			s.handlePictureEvent(context.Background(), managed, event)
		case *events.UserAbout:
			s.dispatchUserAboutUpdatedWebhook(context.Background(), managed, event)
		case *events.IdentityChange:
			s.dispatchIdentityUpdatedWebhook(context.Background(), managed, event)
		case *events.MediaRetry:
			s.dispatchMediaRetryWebhook(context.Background(), managed, event)
		case *events.UndecryptableMessage:
			s.dispatchMessageUndecryptableWebhook(context.Background(), managed, event)
		case *events.Message:
			if s.events != nil {
				go s.events.HandleMessage(managed.Context, managed, event)
			}
		case *events.FBMessage:
			if s.events != nil {
				go s.events.HandleFBMessage(managed.Context, managed, event)
			}
		case *events.Receipt:
			if s.events != nil {
				go s.events.HandleReceipt(managed.Context, managed, event)
			}
		case *events.PushName:
			if s.events != nil {
				go s.events.HandlePushName(managed.Context, managed, event)
			}
		case *events.BusinessName:
			if s.events != nil {
				go s.events.HandleBusinessName(managed.Context, managed, event)
			}
		case *events.Contact:
			if s.events != nil {
				go s.events.HandleContact(managed.Context, managed, event)
			}
		case *events.Blocklist,
			*events.BlocklistChange,
			*events.Archive,
			*events.UnarchiveChatsSetting,
			*events.ClearChat,
			*events.Pin,
			*events.Mute,
			*events.MarkChatAsRead:
			s.dispatchChatUpdatedWebhook(context.Background(), managed, event)
		case *events.DeleteChat:
			s.dispatchChatDeletedWebhook(context.Background(), managed, event)
		case *events.DeleteForMe:
			s.dispatchMessageDeletedWebhook(context.Background(), managed, event)
		case *events.Star:
			s.dispatchMessageStarredWebhook(context.Background(), managed, event)
		case *events.ChatPresence, *events.Presence:
			s.dispatchPresenceUpdatedWebhook(context.Background(), managed, event)
		case *events.PushNameSetting, *events.UserStatusMute:
			s.dispatchSettingsUpdatedWebhook(context.Background(), managed, event)
		case *events.LabelAssociationChat,
			*events.LabelAssociationMessage:
			s.dispatchLabelAssociationWebhook(context.Background(), managed, event)
		case *events.LabelEdit:
			s.dispatchLabelEditWebhook(context.Background(), managed, event)
		case *events.CallOffer,
			*events.CallAccept,
			*events.CallOfferNotice,
			*events.CallPreAccept,
			*events.CallTransport,
			*events.CallTerminate,
			*events.CallReject,
			*events.CallRelayLatency,
			*events.UnknownCallEvent:
			s.dispatchCallUpsertWebhook(context.Background(), managed, event)
		case *events.GroupInfo:
			s.dispatchGroupInfoWebhooks(context.Background(), managed, event)
		case *events.JoinedGroup:
			s.dispatchGroupUpsertWebhook(context.Background(), managed, event)
		case *events.NewsletterJoin,
			*events.NewsletterLeave,
			*events.NewsletterLiveUpdate,
			*events.NewsletterMessageMeta,
			*events.NewsletterMuteChange:
			s.dispatchNewsletterWebhook(context.Background(), managed, event)
		case *events.OfflineSyncPreview:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalOfflineSyncPreview, "preview", "", map[string]int{
				"total":          event.Total,
				"appDataChanges": event.AppDataChanges,
				"messages":       event.Messages,
				"notifications":  event.Notifications,
				"receipts":       event.Receipts,
			})
		case *events.OfflineSyncCompleted:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalOfflineSyncCompleted, "completed", "", map[string]int{
				"count": event.Count,
			})
		case *events.PrivacySettings:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalPrivacySettings, "updated", "", map[string]bool{
				"groupAddChanged":     event.GroupAddChanged,
				"lastSeenChanged":     event.LastSeenChanged,
				"statusChanged":       event.StatusChanged,
				"profileChanged":      event.ProfileChanged,
				"readReceiptsChanged": event.ReadReceiptsChanged,
				"onlineChanged":       event.OnlineChanged,
				"callAddChanged":      event.CallAddChanged,
				"messagesChanged":     event.MessagesChanged,
				"defenseChanged":      event.DefenseChanged,
				"stickersChanged":     event.StickersChanged,
			})
		case *events.AppState:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalAppState, "received", "", map[string]any{
				"index": event.Index,
			})
		case *events.AppStateSyncComplete:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalAppStateSyncComplete, "completed", "", map[string]any{
				"name":     string(event.Name),
				"version":  event.Version,
				"recovery": event.Recovery,
			})
		case *events.AppStateSyncError:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalAppStateSyncError, "error", errorString(event.Error), map[string]any{
				"name":     string(event.Name),
				"fullSync": event.FullSync,
			})
		case *events.NotifyAccountReachoutTimelock:
			s.dispatchInstanceStatusWebhook(context.Background(), managed, webhooksvc.StatusInternalAccountTimelock, "updated", "", map[string]any{
				"enforcementType":     event.EnforcementType,
				"isActive":            event.IsActive,
				"timeEnforcementEnds": event.TimeEnforcementEnds,
			})
		case *events.HistorySync:
			s.dispatchHistorySyncWebhook(context.Background(), managed, event)
		default:
			s.logUnhandledWhatsAppEvent(managed, event)
		}
	})
}

func (s *Service) persistDevice(ctx context.Context, managed *ManagedWhatsAppClient, event string) {
	if managed.Client == nil || managed.Client.Store.ID == nil {
		return
	}
	deviceJID := managed.Client.Store.ID.String()
	owner := managed.Client.Store.ID.ToNonAD()
	ownerJID := owner.String()
	phone := owner.User
	_ = s.instances.SaveWhatsAppDevice(ctx, types.SaveWhatsAppDeviceInput{
		InstanceID:  mustAtoi32(managed.InstanceID),
		DeviceJID:   deviceJID,
		OwnerJID:    ownerJID,
		PhoneNumber: phone,
	})
	managed.mu.Lock()
	managed.DeviceJID = deviceJID
	managed.OwnerJID = ownerJID
	managed.mu.Unlock()
	s.logger.Info().Str("instance_id", managed.InstanceID).Str("instance_name", managed.InstanceName).Str("event", event).Msg("WhatsApp device linked")
}

func (s *Service) refreshProfilePicture(ctx context.Context, managed *ManagedWhatsAppClient) {
	if managed.Client == nil || managed.Client.Store.ID == nil {
		return
	}
	profileCtx, cancel := context.WithTimeout(ctx, s.config.ProfilePictureTimeout)
	defer cancel()
	owner := managed.Client.Store.ID.ToNonAD()
	info, err := managed.Client.GetProfilePictureInfo(profileCtx, owner, nil)
	if err != nil || info == nil || info.URL == "" {
		s.logger.Debug().Err(err).Str("instance_id", managed.InstanceID).Str("event", "profile_picture").Msg("profile picture not updated")
		return
	}
	_ = s.instances.UpdateProfilePicture(ctx, mustAtoi32(managed.InstanceID), &info.URL, &info.ID)
}

func (s *Service) handlePictureEvent(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Picture) {
	if managed.Client != nil && managed.Client.Store.ID != nil {
		owner := managed.Client.Store.ID.ToNonAD()
		if event.JID.ToNonAD() == owner {
			if event.Remove {
				_ = s.instances.UpdateProfilePicture(ctx, mustAtoi32(managed.InstanceID), nil, nil)
			} else {
				s.refreshProfilePicture(ctx, managed)
			}
		}
	}
	s.dispatchProfilePictureUpdatedWebhook(ctx, managed, event)
}

func (s *Service) removePermanent(managed *ManagedWhatsAppClient, status types.InstanceConnectionStatus, event string) {
	_ = s.instances.UpdateConnectionState(context.Background(), types.UpdateConnectionStateInput{
		InstanceID:          mustAtoi32(managed.InstanceID),
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	})
	managed.Cancel()
	if managed.Client != nil {
		managed.Client.Disconnect()
	}
	s.cancelContactSync(managed.InstanceID)
	s.hub.Remove(managed.InstanceID)
	_ = s.lock.Release(context.Background(), managed.InstanceID)
}

func (s *Service) startContactSync(managed *ManagedWhatsAppClient) {
	if s.events == nil || managed == nil {
		return
	}
	ctx, cancel := context.WithCancel(managed.Context)
	handle := &contactSyncHandle{cancel: cancel}
	s.contactSyncMu.Lock()
	if existing, ok := s.contactSyncCancels[managed.InstanceID]; ok && existing != nil {
		existing.cancel()
	}
	s.contactSyncCancels[managed.InstanceID] = handle
	s.contactSyncMu.Unlock()
	go func() {
		defer s.clearContactSync(managed.InstanceID, handle)
		s.events.StartInitialContactSync(ctx, managed)
	}()
}

func (s *Service) cancelContactSync(instanceID string) {
	s.contactSyncMu.Lock()
	handle := s.contactSyncCancels[instanceID]
	delete(s.contactSyncCancels, instanceID)
	s.contactSyncMu.Unlock()
	if handle != nil && handle.cancel != nil {
		handle.cancel()
	}
}

func (s *Service) clearContactSync(instanceID string, handle *contactSyncHandle) {
	s.contactSyncMu.Lock()
	if current := s.contactSyncCancels[instanceID]; current == handle {
		delete(s.contactSyncCancels, instanceID)
	}
	s.contactSyncMu.Unlock()
}

func (s *Service) cancelAllContactSyncs() {
	s.contactSyncMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.contactSyncCancels))
	for instanceID, handle := range s.contactSyncCancels {
		if handle != nil && handle.cancel != nil {
			cancels = append(cancels, handle.cancel)
		}
		delete(s.contactSyncCancels, instanceID)
	}
	s.contactSyncMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Service) consumeQRCodeChannel(
	ctx context.Context,
	cancel context.CancelFunc,
	session *pairingSession,
	managed *ManagedWhatsAppClient,
	qrChannel <-chan whatsmeow.QRChannelItem,
	firstQRCode chan<- QRCodeConnectionResult,
	firstError chan<- error,
) {
	instanceID := mustAtoi32(managed.InstanceID)
	startedAt := time.Now()
	count := 0
	firstSent := false
	terminalSuccess := false

	defer func() {
		cancel()
		s.pairings.remove(managed.InstanceID, session)
		if !terminalSuccess {
			if managed.Client != nil && !managed.Client.IsLoggedIn() {
				s.logger.Debug().Str("instance_id", managed.InstanceID).Str("instance_name", managed.InstanceName).Msg("disconnecting unauthenticated WhatsApp client after QR pairing terminal event")
				managed.Client.Disconnect()
			}
			if managed.Cancel != nil {
				managed.Cancel()
			}
			s.hub.Remove(managed.InstanceID)
		} else {
			s.publishConnectionUpdate(managed, string(types.InstanceConnectionStatusOnline))
		}
		_ = s.lock.Release(context.Background(), managed.InstanceID)
		s.logger.Debug().
			Str("instance_id", managed.InstanceID).
			Str("instance_name", managed.InstanceName).
			Bool("success", terminalSuccess).
			Dur("duration", time.Since(startedAt)).
			Msg("QR pairing session finished")
	}()

	for {
		select {
		case <-ctx.Done():
			if managed.Client != nil && managed.Client.IsLoggedIn() && managed.Client.Store.ID != nil {
				terminalSuccess = true
				return
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionTimeout, "qr_timeout", nil)
				s.trySendFirstError(firstError, ErrQRCodeTimeout, firstSent)
				return
			}
			s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, "qr_cancelled", ctx.Err())
			s.trySendFirstError(firstError, ctx.Err(), firstSent)
			return
		case item, ok := <-qrChannel:
			if !ok {
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, "qr_channel_closed", nil)
				s.trySendFirstError(firstError, ErrQRChannelClosed, firstSent)
				return
			}

			s.logQRChannelItem(managed, item)
			switch {
			case item.Event == whatsmeow.QRChannelEventCode:
				if session.isPasskeyInProgress() {
					s.logger.Debug().
						Str("instance_id", managed.InstanceID).
						Str("instance_name", managed.InstanceName).
						Str("event", item.Event).
						Msg("ignoring QR update while passkey pairing is active")
					continue
				}
				count++
				result, err := s.handleQRCodeItem(instanceID, managed, session, item, count)
				if err != nil {
					s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, "qr_code_generate_error", err)
					s.trySendFirstError(firstError, err, firstSent)
					return
				}
				if !firstSent {
					firstSent = true
					select {
					case firstQRCode <- result:
					default:
					}
				}
				if count >= s.config.QRCodeLimit {
					s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionTimeout, "qr_limit", nil)
					return
				}
			case item == whatsmeow.QRChannelSuccess:
				session.markPasskeyCompleted()
				terminalSuccess = true
				s.handleQRSuccess(managed)
				return
			case item == whatsmeow.QRChannelTimeout:
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionTimeout, item.Event, item.Error)
				s.trySendFirstError(firstError, ErrQRCodeTimeout, firstSent)
				return
			case item == whatsmeow.QRChannelClientOutdated:
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusClientOutdated, item.Event, item.Error)
				s.trySendFirstError(firstError, ErrClientOutdated, firstSent)
				return
			case item.Event == whatsmeow.QRChannelEventError:
				err := ErrPairingFailed
				if item.Error != nil {
					err = fmt.Errorf("%w: %w", ErrPairingFailed, item.Error)
				}
				if session.isPasskeyInProgress() {
					session.markPasskeyFailed(err)
					s.publishPasskeyFailed(managed, "PASSKEY_PAIRING_FAILED", "Nao foi possivel concluir o pareamento por Passkey.")
				}
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, item.Event, item.Error)
				s.trySendFirstError(firstError, err, firstSent)
				return
			case item == whatsmeow.QRChannelScannedWithoutMultidevice, item == whatsmeow.QRChannelErrUnexpectedEvent:
				err := fmt.Errorf("%w: %s", ErrPairingFailed, item.Event)
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, item.Event, item.Error)
				s.trySendFirstError(firstError, err, firstSent)
				return
			case item.Event == whatsmeow.QRChannelEventPasskeyRequest:
				if item.PasskeyRequest == nil || item.PasskeyRequest.PublicKey == nil {
					err := fmt.Errorf("%w: passkey request without public key", ErrPasskeyNotAvailable)
					session.markPasskeyFailed(err)
					s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, item.Event, nil)
					s.trySendFirstError(firstError, err, firstSent)
					return
				}
				challenge := session.setPasskeyChallenge(item.PasskeyRequest.PublicKey)
				s.publishPasskeyRequired(managed, challenge)
				s.logger.Info().
					Str("instance_id", managed.InstanceID).
					Str("instance_name", managed.InstanceName).
					Str("request_id", challenge.RequestID).
					Str("passkey_state", string(challenge.State)).
					Str("event", item.Event).
					Time("expires_at", challenge.ExpiresAt).
					Msg("passkey challenge received from QR channel")
			case item.Event == whatsmeow.QRChannelEventPasskeyResponse:
				if item.PasskeyConfirmation == nil {
					err := fmt.Errorf("%w: passkey confirmation missing", ErrPairingFailed)
					session.markPasskeyFailed(err)
					s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, item.Event, nil)
					s.trySendFirstError(firstError, err, firstSent)
					return
				}
				if err := s.handlePasskeyConfirmation(ctx, session, managed, item.PasskeyConfirmation.Code, item.PasskeyConfirmation.SkipHandoffUX); err != nil {
					s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, "passkey_confirmation_error", nil)
					s.trySendFirstError(firstError, err, firstSent)
					return
				}
			default:
				err := fmt.Errorf("%w: unexpected QR event %s", ErrPairingFailed, item.Event)
				s.updateQRFailure(instanceID, types.InstanceConnectionStatusConnectionError, item.Event, item.Error)
				s.trySendFirstError(firstError, err, firstSent)
				return
			}
		}
	}
}

func (s *Service) handleQRCodeItem(instanceID int32, managed *ManagedWhatsAppClient, session *pairingSession, item whatsmeow.QRChannelItem, count int) (QRCodeConnectionResult, error) {
	base64PNG, err := s.qr.GenerateDataURL(item.Code)
	if err != nil {
		return QRCodeConnectionResult{}, err
	}
	status := types.InstanceConnectionStatusQRCode
	event := "qr_code"
	_ = s.instances.UpdateConnectionState(context.Background(), types.UpdateConnectionStateInput{
		InstanceID:          instanceID,
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	})
	result := QRCodeConnectionResult{Count: count, Code: item.Code, Base64: base64PNG}
	session.setCurrentQR(result)
	s.publishQRCodeUpdate(managed, item, result)
	return result, nil
}

func (s *Service) handleQRSuccess(managed *ManagedWhatsAppClient) {
	s.persistDevice(context.Background(), managed, "qr_success")
	status := types.InstanceConnectionStatusOnline
	now := time.Now().UTC()
	event := "success"
	empty := ""
	_ = s.instances.UpdateConnectionState(context.Background(), types.UpdateConnectionStateInput{
		InstanceID:          mustAtoi32(managed.InstanceID),
		ConnectionStatus:    &status,
		LastConnectedAt:     &now,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
		LastConnectionError: types.OptionalField[string]{Set: true, Value: &empty},
		ResetAttempts:       true,
	})
	managed.SignalConnected()
	go s.refreshProfilePicture(context.Background(), managed)
	s.startContactSync(managed)
	s.publishConnectionUpdate(managed, string(status))
}

func (s *Service) updateQRFailure(instanceID int32, status types.InstanceConnectionStatus, event string, cause error) {
	input := types.UpdateConnectionStateInput{
		InstanceID:          instanceID,
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	}
	if cause != nil {
		message := cause.Error()
		input.LastConnectionError = types.OptionalField[string]{Set: true, Value: &message}
	}
	_ = s.instances.UpdateConnectionState(context.Background(), input)
}

func (s *Service) trySendFirstError(firstError chan<- error, err error, firstSent bool) {
	if firstSent || err == nil {
		return
	}
	select {
	case firstError <- err:
	default:
	}
}

func (s *Service) logQRChannelItem(managed *ManagedWhatsAppClient, item whatsmeow.QRChannelItem) {
	event := s.logger.Debug().
		Str("instanceName", managed.InstanceName).
		Str("instanceId", managed.InstanceID).
		Str("event", item.Event).
		Dur("timeout", item.Timeout).
		Err(item.Error)
	if item.Code != "" {
		event.Int("codeLength", len(item.Code))
		if len(item.Code) > 12 {
			event.Str("codePrefix", item.Code[:12])
		}
	}
	event.Msg("WhatsApp QR channel event")
}

func (s *Service) publishQRCodeUpdate(managed *ManagedWhatsAppClient, item whatsmeow.QRChannelItem, result QRCodeConnectionResult) {
	s.logger.Debug().
		Str("event", "qrcode.update").
		Str("instanceName", managed.InstanceName).
		Str("instanceId", managed.InstanceID).
		Int("count", result.Count).
		Dur("expiresIn", item.Timeout).
		Msg("QR code update ready for event stream")
	expiresAt := time.Now().UTC().Add(item.Timeout)
	s.dispatchWebhook(context.Background(), managed, types.WebhookEventQRCodeUpdated, webhooksvc.QRCodeUpdatedWebhookData{
		Count:            result.Count,
		Code:             result.Code,
		Base64:           result.Base64,
		ExpiresInSeconds: int64(item.Timeout.Seconds()),
		ExpiresAt:        expiresAt,
	})
}

func (s *Service) publishConnectionUpdate(managed *ManagedWhatsAppClient, status string) {
	managed.mu.RLock()
	ownerJID := managed.OwnerJID
	managed.mu.RUnlock()
	s.logger.Debug().
		Str("event", "connection.update").
		Str("instanceName", managed.InstanceName).
		Str("instanceId", managed.InstanceID).
		Str("status", status).
		Str("ownerJid", ownerJID).
		Msg("connection update ready for event stream")
}

func (s *Service) publishPasskeyRequired(managed *ManagedWhatsAppClient, challenge passkeyChallengeSnapshot) {
	s.logger.Info().
		Str("event", "connection.passkey.required").
		Str("instance_id", managed.InstanceID).
		Str("instance_name", managed.InstanceName).
		Str("request_id", challenge.RequestID).
		Str("passkey_state", string(challenge.State)).
		Time("expires_at", challenge.ExpiresAt).
		Msg("passkey challenge ready for panel event stream")
}

func (s *Service) publishPasskeyConfirmation(managed *ManagedWhatsAppClient, state PasskeyPairingState, code string, skipHandoffUX bool) {
	s.logger.Info().
		Str("event", "connection.passkey.confirmation").
		Str("instance_id", managed.InstanceID).
		Str("instance_name", managed.InstanceName).
		Str("passkey_state", string(state)).
		Str("confirmation_code", code).
		Bool("skip_handoff_ux", skipHandoffUX).
		Msg("passkey confirmation ready for panel event stream")
}

func (s *Service) publishPasskeyFailed(managed *ManagedWhatsAppClient, code string, message string) {
	s.logger.Info().
		Str("event", "connection.passkey.failed").
		Str("instance_id", managed.InstanceID).
		Str("instance_name", managed.InstanceName).
		Str("passkey_state", string(PasskeyStateFailed)).
		Str("error_type", code).
		Str("message", message).
		Msg("passkey failure ready for panel event stream")
}

func (s *Service) handlePasskeyConfirmation(ctx context.Context, session *pairingSession, managed *ManagedWhatsAppClient, code string, skipHandoffUX bool) error {
	session.setPasskeyConfirmation(code, skipHandoffUX)
	s.publishPasskeyConfirmation(managed, PasskeyStateAwaitingConfirmation, code, skipHandoffUX)
	if skipHandoffUX {
		s.logger.Info().
			Str("instance_id", managed.InstanceID).
			Str("instance_name", managed.InstanceName).
			Str("passkey_state", string(PasskeyStateAwaitingConfirmation)).
			Str("event", "connection.passkey.confirmation.skipped").
			Msg("passkey confirmation skipped because QR channel already confirmed")
		return nil
	}
	clientFactory := s.passkeyClient
	if clientFactory == nil {
		clientFactory = newWhatsmeowPasskeyClient
	}
	client := clientFactory(managed)
	if client == nil {
		session.markPasskeyFailed(ErrClientNotConnected)
		s.publishPasskeyFailed(managed, "PASSKEY_PAIRING_FAILED", "Nao foi possivel concluir o pareamento por Passkey.")
		return ErrClientNotConnected
	}
	session.passkeyCommandMu.Lock()
	defer session.passkeyCommandMu.Unlock()
	commandCtx, cancel := context.WithTimeout(ctx, passkeyCommandTimeout)
	defer cancel()
	startedAt := time.Now()
	if err := client.SendPasskeyConfirmation(commandCtx); err != nil {
		session.markPasskeyFailed(err)
		s.publishPasskeyFailed(managed, "PASSKEY_PAIRING_FAILED", "Nao foi possivel concluir o pareamento por Passkey.")
		s.logger.Warn().
			Err(err).
			Str("instance_id", managed.InstanceID).
			Str("instance_name", managed.InstanceName).
			Str("passkey_state", string(PasskeyStateFailed)).
			Str("event", "connection.passkey.confirmation.failed").
			Dur("duration", time.Since(startedAt)).
			Msg("passkey confirmation failed")
		return fmt.Errorf("%w: confirmation", ErrPasskeyServiceUnavailable)
	}
	session.markPasskeyConfirmationSent()
	s.logger.Info().
		Str("instance_id", managed.InstanceID).
		Str("instance_name", managed.InstanceName).
		Str("passkey_state", string(PasskeyStateConfirmationSent)).
		Str("event", "connection.passkey.confirmation.sent").
		Dur("duration", time.Since(startedAt)).
		Msg("passkey confirmation sent")
	return nil
}

func (s *Service) dispatchConnectionWebhook(ctx context.Context, managed *ManagedWhatsAppClient, internalEvent string, statusReason int, lastConnection *time.Time, message string) {
	data, ok := webhooksvc.NormalizeConnectionWebhookData(internalEvent, statusReason, lastConnection, message)
	if !ok {
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventConnectionUpdated, data)
}

func (s *Service) dispatchInstanceStatusWebhook(ctx context.Context, managed *ManagedWhatsAppClient, internalEvent string, status string, message string, data any) {
	output, ok := webhooksvc.NormalizeInstanceStatusWebhookData(internalEvent, status, message, data)
	if !ok {
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventStatusInstance, output)
}

func (s *Service) dispatchChatUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := chatUpdatedWebhookData(event, time.Now().UTC())
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventChatsUpdated, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventChatsUpdated, data)
}

func (s *Service) dispatchChatDeletedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.DeleteChat) {
	data, err := chatDeletedWebhookData(event, time.Now().UTC())
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventChatsDeleted, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventChatsDeleted, data)
}

func (s *Service) dispatchPresenceUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := presenceUpdatedWebhookData(event, time.Now().UTC())
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventPresenceUpdated, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventPresenceUpdated, data)
}

func (s *Service) dispatchProfilePictureUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Picture) {
	data := profilePictureUpdatedWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventProfilePictureUpdated, data)
}

func (s *Service) dispatchUserAboutUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.UserAbout) {
	data := userAboutUpdatedWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventUserAboutUpdated, data)
}

func (s *Service) dispatchIdentityUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.IdentityChange) {
	data := identityUpdatedWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventIdentityUpdated, data)
}

func (s *Service) dispatchMediaRetryWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.MediaRetry) {
	data := mediaRetryWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventMediaRetry, data)
}

func (s *Service) dispatchMessageDeletedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.DeleteForMe) {
	data := messageDeletedWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventMessagesDeleted, data)
}

func (s *Service) dispatchMessageStarredWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Star) {
	data := messageStarredWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventMessagesStarred, data)
}

func (s *Service) dispatchMessageUndecryptableWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.UndecryptableMessage) {
	data := messageUndecryptableWebhookData(event, time.Now().UTC())
	s.dispatchWebhook(ctx, managed, types.WebhookEventMessagesUndecryptable, data)
}

func (s *Service) dispatchSettingsUpdatedWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := settingsUpdatedWebhookData(event, time.Now().UTC())
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventSettingsUpdated, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventSettingsUpdated, data)
}

func (s *Service) dispatchHistorySyncWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.HistorySync) {
	data, err := historySyncWebhookData(event, time.Now().UTC())
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventHistorySync, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventHistorySync, data)
}

func (s *Service) dispatchCallUpsertWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := NewCallEventNormalizer().Normalize(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventCallUpsert, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventCallUpsert, data)
}

func (s *Service) dispatchGroupInfoWebhooks(ctx context.Context, managed *ManagedWhatsAppClient, event *events.GroupInfo) {
	normalizer := NewGroupEventNormalizer()
	updates, err := normalizer.NormalizeUpdate(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventGroupsUpdated, managed, event, err)
		return
	}
	for _, data := range updates {
		s.dispatchWebhook(ctx, managed, types.WebhookEventGroupsUpdated, []webhooksvc.GroupUpdateWebhookData{data})
	}
	participantUpdates, err := normalizer.NormalizeParticipantUpdates(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventGroupsParticipantsUpdated, managed, event, err)
		return
	}
	for _, data := range participantUpdates {
		s.dispatchWebhook(ctx, managed, types.WebhookEventGroupsParticipantsUpdated, data)
	}
}

func (s *Service) dispatchGroupUpsertWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event *events.JoinedGroup) {
	data, err := NewGroupEventNormalizer().NormalizeUpsert(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventGroupsUpsert, managed, event, err)
		return
	}
	if len(data) == 0 {
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventGroupsUpsert, data)
}

func (s *Service) dispatchNewsletterWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := NewNewsletterEventNormalizer().Normalize(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventNewsletter, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventNewsletter, data)
}

func (s *Service) dispatchLabelAssociationWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := NewLabelEventNormalizer().NormalizeAssociation(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventLabelsAssociation, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventLabelsAssociation, data)
}

func (s *Service) dispatchLabelEditWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event any) {
	data, err := NewLabelEventNormalizer().NormalizeEdit(event)
	if err != nil {
		s.logWebhookNormalizationFailure(types.WebhookEventLabelsEdit, managed, event, err)
		return
	}
	s.dispatchWebhook(ctx, managed, types.WebhookEventLabelsEdit, data)
}

func (s *Service) logWebhookNormalizationFailure(event types.WebhookEvent, managed *ManagedWhatsAppClient, sourceEvent any, err error) {
	if managed == nil {
		return
	}
	s.logger.Warn().
		Err(err).
		Str("event", string(event)).
		Str("instanceId", managed.InstanceID).
		Str("instanceName", managed.InstanceName).
		Str("sourceEvent", fmt.Sprintf("%T", sourceEvent)).
		Msg("webhook event normalization failed")
}

func (s *Service) logUnhandledWhatsAppEvent(managed *ManagedWhatsAppClient, sourceEvent any) {
	if managed == nil {
		return
	}
	eventType := reflect.TypeOf(sourceEvent)
	typeName := "<nil>"
	packageName := ""
	if eventType != nil {
		typeName = eventType.String()
		base := eventType
		if base.Kind() == reflect.Pointer {
			base = base.Elem()
		}
		packageName = base.PkgPath()
	}
	s.logger.Warn().
		Str("eventType", typeName).
		Str("package", packageName).
		Str("instanceId", managed.InstanceID).
		Str("instanceName", managed.InstanceName).
		Msg("unhandled whatsapp event")
}

func (s *Service) dispatchWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event types.WebhookEvent, data any) {
	if s.webhooks == nil || managed == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	instance, err := s.instances.FindByName(ctx, managed.InstanceName)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("event", string(event)).
			Str("instanceId", managed.InstanceID).
			Str("instanceName", managed.InstanceName).
			Msg("webhook instance snapshot not loaded")
		return
	}
	if err := s.webhooks.Dispatch(ctx, webhooksvc.NewWebhookInstance(instance.Instance), event, data); err != nil {
		s.logger.Warn().
			Err(err).
			Str("event", string(event)).
			Str("instanceId", managed.InstanceID).
			Str("instanceName", managed.InstanceName).
			Msg("webhook dispatch not queued")
	}
}

func (s *Service) markConnectionError(ctx context.Context, instanceID int32, status types.InstanceConnectionStatus, event string) {
	_ = s.instances.UpdateConnectionState(ctx, types.UpdateConnectionStateInput{
		InstanceID:          instanceID,
		ConnectionStatus:    &status,
		LastConnectionEvent: types.OptionalField[string]{Set: true, Value: &event},
	})
}

func mustAtoi32(value string) int32 {
	parsed, _ := strconv.ParseInt(value, 10, 32)
	return int32(parsed)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func IsAuthError(err error) bool {
	return errors.Is(err, ErrInvalidInstanceToken)
}
