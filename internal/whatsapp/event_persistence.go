package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

const (
	defaultInitialContactSyncDelay = 30 * time.Second
	defaultContactProfileWorkers   = 5
	defaultReceiptRetryAttempts    = 3
	defaultReceiptRetryDelay       = 100 * time.Millisecond
)

type EventPersistenceConfig struct {
	SaveDataNewMessage       bool
	SaveMessageUpdate        bool
	SaveDataContacts         bool
	InitialContactSyncDelay  time.Duration
	ContactProfileWorkers    int
	ProfilePictureTimeout    time.Duration
	ReceiptRetryAttempts     int
	ReceiptRetryInitialDelay time.Duration
}

type EventPersistenceService struct {
	cfg            EventPersistenceConfig
	messages       repository.MessageRepository
	messageUpdates repository.MessageUpdateRepository
	contacts       repository.ContactRepository
	instances      webhookInstanceFinder
	webhooks       webhooksvc.WebhookManager
	normalizer     MessageEventNormalizer
	logger         zerolog.Logger
}

type webhookInstanceFinder interface {
	FindByName(ctx context.Context, name string) (dbtypes.InstanceWithAuth, error)
}

func NewEventPersistenceService(
	cfg EventPersistenceConfig,
	messages repository.MessageRepository,
	messageUpdates repository.MessageUpdateRepository,
	contacts repository.ContactRepository,
	logger zerolog.Logger,
) *EventPersistenceService {
	if cfg.InitialContactSyncDelay <= 0 {
		cfg.InitialContactSyncDelay = defaultInitialContactSyncDelay
	}
	if cfg.ContactProfileWorkers <= 0 {
		cfg.ContactProfileWorkers = defaultContactProfileWorkers
	}
	if cfg.ReceiptRetryAttempts <= 0 {
		cfg.ReceiptRetryAttempts = defaultReceiptRetryAttempts
	}
	if cfg.ReceiptRetryInitialDelay <= 0 {
		cfg.ReceiptRetryInitialDelay = defaultReceiptRetryDelay
	}
	if cfg.ProfilePictureTimeout <= 0 {
		cfg.ProfilePictureTimeout = 15 * time.Second
	}
	return &EventPersistenceService{
		cfg:            cfg,
		messages:       messages,
		messageUpdates: messageUpdates,
		contacts:       contacts,
		normalizer:     NewMessageEventNormalizer(),
		logger:         logger.With().Str("component", "event_persistence").Logger(),
	}
}

func (s *EventPersistenceService) SetWebhookDispatcher(instances webhookInstanceFinder, webhooks webhooksvc.WebhookManager) {
	s.instances = instances
	s.webhooks = webhooks
}

func (s *EventPersistenceService) HandleMessage(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Message) {
	instanceID := mustAtoi32(managed.InstanceID)
	message, err := s.normalizer.NormalizeMessage(instanceID, event)
	if err != nil {
		s.logger.Warn().Err(err).Int32("instanceId", instanceID).Str("instanceName", managed.InstanceName).Str("event", "message").Msg("message event not persisted")
		return
	}
	if !s.cfg.SaveDataNewMessage {
		return
	}
	if err := s.messages.CreateOrIgnore(ctx, message); err != nil {
		s.logger.Error().Err(err).
			Str("event", "message").
			Str("operation", "message.create_or_ignore").
			Int32("instanceId", instanceID).
			Str("instanceName", managed.InstanceName).
			Str("messageKeyId", message.KeyID).
			Str("remoteJid", stringValue(message.KeyRemoteJid)).
			Msg("failed to persist message event")
	} else {
		bin, _ := json.Marshal(message)
		s.logger.
			Trace().
			RawJSON(managed.InstanceName, bin).
			Msg("new message")
		s.dispatchMessageUpsertWebhook(ctx, managed, message.KeyID)
	}
}

func (s *EventPersistenceService) HandleFBMessage(ctx context.Context, managed *ManagedWhatsAppClient, event *events.FBMessage) {
	instanceID := mustAtoi32(managed.InstanceID)
	message, err := s.normalizer.NormalizeFBMessage(instanceID, event)
	if err != nil {
		s.logger.Warn().Err(err).Int32("instanceId", instanceID).Str("instanceName", managed.InstanceName).Str("event", "fb_message").Msg("fb message event not persisted")
		return
	}
	if !s.cfg.SaveDataNewMessage {
		return
	}
	if err := s.messages.CreateOrIgnore(ctx, message); err != nil {
		s.logger.Error().Err(err).
			Str("event", "fb_message").
			Str("operation", "message.create_or_ignore").
			Int32("instanceId", instanceID).
			Str("instanceName", managed.InstanceName).
			Str("messageKeyId", message.KeyID).
			Str("remoteJid", stringValue(message.KeyRemoteJid)).
			Msg("failed to persist fb message event")
	} else {
		s.dispatchMessageUpsertWebhook(ctx, managed, message.KeyID)
	}
}

func (s *EventPersistenceService) HandleReceipt(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Receipt) {
	if event == nil {
		return
	}
	instanceID := mustAtoi32(managed.InstanceID)
	status := normalizeReceiptStatus(event.Type)
	dateTime := event.Timestamp
	if dateTime.IsZero() {
		dateTime = time.Now().UTC()
	}
	for _, messageID := range event.MessageIDs {
		keyID := string(messageID)
		if strings.TrimSpace(keyID) == "" {
			continue
		}
		if !s.cfg.SaveMessageUpdate {
			continue
		}
		message, err := s.findMessageWithRetry(ctx, instanceID, keyID)
		if err != nil {
			if errors.Is(err, repository.ErrMessageNotFound) {
				s.logger.Warn().
					Str("event", "receipt").
					Int32("instanceId", instanceID).
					Str("instanceName", managed.InstanceName).
					Str("messageKeyId", keyID).
					Str("receiptType", string(event.Type)).
					Time("timestamp", dateTime).
					Msg("message not found for receipt")
				s.logger.Warn().
					Str("event", string(dbtypes.WebhookEventMessagesUpdated)).
					Int32("instanceId", instanceID).
					Str("instanceName", managed.InstanceName).
					Str("messageKey", keyID).
					Msg("webhook source entity not found")
				continue
			}
			s.logger.Error().Err(err).Str("event", "receipt").Int32("instanceId", instanceID).Str("instanceName", managed.InstanceName).Str("messageKeyId", keyID).Msg("failed to find message for receipt")
			continue
		}
		if err := s.messageUpdates.CreateOrIgnore(ctx, dbtypes.CreateMessageUpdateInput{
			DateTime:  dateTime,
			Status:    status,
			MessageID: message.ID,
		}); err != nil {
			s.logger.Error().Err(err).
				Str("event", "receipt").
				Str("operation", "message_update.create_or_ignore").
				Int32("instanceId", instanceID).
				Str("instanceName", managed.InstanceName).
				Str("messageKeyId", keyID).
				Msg("failed to persist receipt")
			continue
		}
		s.dispatchWebhook(ctx, managed, dbtypes.WebhookEventMessagesUpdated, webhooksvc.NewMessageUpdateWebhookData(message.ID, status, dateTime))
	}
}

func (s *EventPersistenceService) findMessageWithRetry(ctx context.Context, instanceID int32, keyID string) (dbtypes.Message, error) {
	delay := s.cfg.ReceiptRetryInitialDelay
	var lastErr error
	for attempt := 0; attempt < s.cfg.ReceiptRetryAttempts; attempt++ {
		message, err := s.messages.FindByKeyIDForInstance(ctx, instanceID, keyID)
		if err == nil {
			return message, nil
		}
		lastErr = err
		if !errors.Is(err, repository.ErrMessageNotFound) || attempt == s.cfg.ReceiptRetryAttempts-1 {
			return dbtypes.Message{}, err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return dbtypes.Message{}, ctx.Err()
		case <-timer.C:
		}
		delay *= 2
	}
	return dbtypes.Message{}, lastErr
}

func (s *EventPersistenceService) StartInitialContactSync(ctx context.Context, managed *ManagedWhatsAppClient) {
	if !s.cfg.SaveDataContacts || managed == nil || managed.Client == nil {
		return
	}
	timer := time.NewTimer(s.cfg.InitialContactSyncDelay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	if ctx.Err() != nil || !managed.IsReady() || managed.Client.Store == nil || managed.Client.Store.Contacts == nil {
		return
	}
	contacts, err := managed.Client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		s.logger.Warn().Err(err).Str("event", "contact_sync").Str("instanceId", managed.InstanceID).Str("instanceName", managed.InstanceName).Msg("failed to load WhatsApp contacts")
		return
	}
	s.syncContacts(ctx, managed, contacts)
}

func (s *EventPersistenceService) syncContacts(ctx context.Context, managed *ManagedWhatsAppClient, contacts map[types.JID]types.ContactInfo) {
	jobs := make(chan normalizedContact)
	var wg sync.WaitGroup
	workers := s.cfg.ContactProfileWorkers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for contact := range jobs {
				s.persistContact(ctx, managed, contact, true)
			}
		}()
	}
	for jid, info := range contacts {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		case jobs <- normalizeStoreContact(mustAtoi32(managed.InstanceID), jid, info):
		}
	}
	close(jobs)
	wg.Wait()
}

func (s *EventPersistenceService) HandleContact(ctx context.Context, managed *ManagedWhatsAppClient, event *events.Contact) {
	if event == nil {
		return
	}
	contact := normalizeContactEvent(mustAtoi32(managed.InstanceID), event)
	if contact.RemoteJid == "" {
		return
	}
	if !s.cfg.SaveDataContacts {
		return
	}
	persisted, ok := s.persistContact(ctx, managed, contact, true)
	if !ok {
		return
	}
	s.dispatchWebhook(ctx, managed, dbtypes.WebhookEventContactsUpsert, webhooksvc.NewContactUpsertWebhookData(persisted, contact.LID, "upserted"))
}

func (s *EventPersistenceService) HandlePushName(ctx context.Context, managed *ManagedWhatsAppClient, event *events.PushName) {
	if event == nil || !s.cfg.SaveDataContacts {
		return
	}
	remote := preferredContactJID(event.JID, event.JIDAlt)
	contact := normalizedContact{
		InstanceID: mustAtoi32(managed.InstanceID),
		JID:        remote,
		LID:        stringPtrFromJID(firstLIDJID(event.JIDAlt, event.JID)),
		RemoteJid:  jidString(remote),
		PushName:   event.NewPushName,
	}
	s.persistAndDispatchContactUpdate(ctx, managed, event, contact)
}

func (s *EventPersistenceService) HandleBusinessName(ctx context.Context, managed *ManagedWhatsAppClient, event *events.BusinessName) {
	if event == nil || !s.cfg.SaveDataContacts {
		return
	}
	remote := preferredContactJID(event.JID, types.EmptyJID)
	contact := normalizedContact{
		InstanceID: mustAtoi32(managed.InstanceID),
		JID:        remote,
		LID:        stringPtrFromJID(firstLIDJID(event.JID)),
		RemoteJid:  jidString(remote),
	}
	s.persistAndDispatchContactUpdate(ctx, managed, event, contact)
}

func (s *EventPersistenceService) persistAndDispatchContactUpdate(ctx context.Context, managed *ManagedWhatsAppClient, event any, contact normalizedContact) {
	if contact.RemoteJid == "" {
		return
	}
	persisted, ok := s.persistContact(ctx, managed, contact, false)
	if !ok {
		return
	}
	dto := webhooksvc.ContactUpdateWebhookData{
		ID:        int64(persisted.ID),
		RemoteJID: persisted.RemoteJid,
		LID:       contact.LID,
		PushName:  persisted.PushName,
		Action:    "updated",
		Source:    "unknown",
	}
	data, err := NewContactUpdateNormalizer().Normalize(event, dto)
	if err != nil {
		s.logger.Warn().
			Err(err).
			Str("event", string(dbtypes.WebhookEventContactsUpdated)).
			Int32("instanceId", contact.InstanceID).
			Str("instanceName", managed.InstanceName).
			Str("sourceEvent", fmt.Sprintf("%T", event)).
			Msg("webhook event normalization failed")
		return
	}
	s.dispatchWebhook(ctx, managed, dbtypes.WebhookEventContactsUpdated, data)
}

func (s *EventPersistenceService) persistContact(ctx context.Context, managed *ManagedWhatsAppClient, contact normalizedContact, fetchProfilePicture bool) (dbtypes.Contact, bool) {
	if contact.RemoteJid == "" {
		return dbtypes.Contact{}, false
	}
	if fetchProfilePicture && managed != nil && managed.IsReady() {
		contact.ProfilePicURL = s.profilePictureURL(ctx, managed.Client, contact.JID)
	}
	input := dbtypes.CreateContactInput{
		RemoteJid:     contact.RemoteJid,
		PushName:      stringPtr(contact.PushName),
		ProfilePicUrl: stringPtr(contact.ProfilePicURL),
		InstanceID:    contact.InstanceID,
	}
	persisted, err := s.contacts.Upsert(ctx, input)
	if err != nil {
		s.logger.Error().Err(err).
			Str("event", "contact").
			Str("operation", "contact.upsert").
			Int32("instanceId", contact.InstanceID).
			Str("instanceName", managed.InstanceName).
			Str("remoteJid", contact.RemoteJid).
			Msg("failed to persist contact")
		return dbtypes.Contact{}, false
	}
	return persisted, true
}

func (s *EventPersistenceService) profilePictureURL(ctx context.Context, client *whatsmeow.Client, jid types.JID) string {
	if client == nil || jid.IsEmpty() {
		return ""
	}
	profileCtx, cancel := context.WithTimeout(ctx, s.cfg.ProfilePictureTimeout)
	defer cancel()
	info, err := client.GetProfilePictureInfo(profileCtx, jid, nil)
	if err != nil || info == nil {
		return ""
	}
	return info.URL
}

type normalizedContact struct {
	InstanceID    int32
	JID           types.JID
	LID           *string
	RemoteJid     string
	PushName      string
	ProfilePicURL string
}

func normalizeStoreContact(instanceID int32, jid types.JID, info types.ContactInfo) normalizedContact {
	remote := preferredContactJID(jid, types.EmptyJID)
	return normalizedContact{
		InstanceID: instanceID,
		JID:        remote,
		RemoteJid:  jidString(remote),
		PushName:   firstNonEmpty(info.PushName, info.FullName, info.FirstName, info.BusinessName),
	}
}

func normalizeContactEvent(instanceID int32, event *events.Contact) normalizedContact {
	pnJID := jidFromString(contactActionPNJID(event.Action))
	lidJID := jidFromString(contactActionLIDJID(event.Action))
	remote := preferredContactJID(event.JID, pnJID)
	if remote.IsEmpty() {
		remote = preferredContactJID(pnJID, lidJID)
	}
	return normalizedContact{
		InstanceID: instanceID,
		JID:        remote,
		LID:        stringPtrFromJID(lidJID),
		RemoteJid:  jidString(remote),
		PushName:   firstNonEmpty(contactActionFullName(event.Action), contactActionFirstName(event.Action), contactActionUsername(event.Action)),
	}
}

func (s *EventPersistenceService) dispatchMessageUpsertWebhook(ctx context.Context, managed *ManagedWhatsAppClient, keyID string) {
	instanceID := mustAtoi32(managed.InstanceID)
	message, err := s.messages.FindByKeyIDForInstance(ctx, instanceID, keyID)
	if err != nil {
		if errors.Is(err, repository.ErrMessageNotFound) {
			s.logger.Warn().
				Str("event", string(dbtypes.WebhookEventMessagesUpsert)).
				Int32("instanceId", instanceID).
				Str("instanceName", managed.InstanceName).
				Str("messageKey", keyID).
				Msg("webhook source entity not found")
			return
		}
		s.logger.Warn().
			Err(err).
			Str("event", string(dbtypes.WebhookEventMessagesUpsert)).
			Int32("instanceId", instanceID).
			Str("instanceName", managed.InstanceName).
			Str("messageKey", keyID).
			Msg("webhook source entity not loaded")
		return
	}
	s.dispatchWebhook(ctx, managed, dbtypes.WebhookEventMessagesUpsert, webhooksvc.NewMessageUpsertWebhookData(message))
}

func (s *EventPersistenceService) dispatchWebhook(ctx context.Context, managed *ManagedWhatsAppClient, event dbtypes.WebhookEvent, data any) {
	if s.webhooks == nil || s.instances == nil || managed == nil {
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

func preferredContactJID(primary types.JID, fallback types.JID) types.JID {
	for _, candidate := range []types.JID{primary, fallback} {
		if candidate.IsEmpty() {
			continue
		}
		switch candidate.Server {
		case types.DefaultUserServer, types.GroupServer, types.NewsletterServer:
			return candidate
		}
	}
	for _, candidate := range []types.JID{primary, fallback} {
		if !candidate.IsEmpty() {
			return candidate
		}
	}
	return types.EmptyJID
}

func normalizeReceiptStatus(value types.ReceiptType) string {
	switch value {
	case types.ReceiptTypeDelivered:
		return "delivered"
	case types.ReceiptTypeSender:
		return "sent"
	case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
		return "read"
	case types.ReceiptTypePlayed, types.ReceiptTypePlayedSelf:
		return "played"
	case types.ReceiptTypeServerError:
		return "server_error"
	case types.ReceiptTypeRetry:
		return "retry"
	default:
		return "unknown"
	}
}

func contactActionFullName(action *waSyncAction.ContactAction) string {
	if action == nil {
		return ""
	}
	return action.GetFullName()
}

func contactActionFirstName(action *waSyncAction.ContactAction) string {
	if action == nil {
		return ""
	}
	return action.GetFirstName()
}

func contactActionUsername(action *waSyncAction.ContactAction) string {
	if action == nil {
		return ""
	}
	return action.GetUsername()
}

func contactActionPNJID(action *waSyncAction.ContactAction) string {
	if action == nil {
		return ""
	}
	return action.GetPnJID()
}

func contactActionLIDJID(action *waSyncAction.ContactAction) string {
	if action == nil {
		return ""
	}
	return action.GetLidJID()
}

func jidString(jid types.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return jid.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
