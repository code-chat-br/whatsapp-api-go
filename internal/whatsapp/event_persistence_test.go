package whatsapp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	waConsumerApplication "go.mau.fi/whatsmeow/proto/waConsumerApplication"
	wae2e "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waSyncAction"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
	webhooksvc "whatsapp-go-api/internal/webhook"
)

func TestHandleReceiptFindsMessageByInstanceAndKeyWithRetry(t *testing.T) {
	messages := &fakePersistenceMessages{
		findResults: []findMessageResult{
			{err: repository.ErrMessageNotFound},
			{message: dbtypes.Message{ID: 7}},
		},
	}
	updates := &fakePersistenceMessageUpdates{}
	service := NewEventPersistenceService(EventPersistenceConfig{
		SaveMessageUpdate:        true,
		ReceiptRetryAttempts:     2,
		ReceiptRetryInitialDelay: time.Millisecond,
	}, messages, updates, &fakePersistenceContacts{}, zerolog.Nop())

	service.HandleReceipt(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, receiptEvent("msg-1", time.Unix(10, 0).UTC()))

	if messages.findCalls != 2 {
		t.Fatalf("expected 2 find attempts, got %d", messages.findCalls)
	}
	if messages.instanceID != 42 || messages.keyID != "msg-1" {
		t.Fatalf("expected lookup by instance 42 and key msg-1, got %d/%s", messages.instanceID, messages.keyID)
	}
	if updates.createCalls != 1 || updates.input.MessageID != 7 || updates.input.Status != "delivered" {
		t.Fatalf("unexpected update input: calls=%d input=%+v", updates.createCalls, updates.input)
	}
}

func TestHandleReceiptFlagDisabledDoesNotCallRepositories(t *testing.T) {
	messages := &fakePersistenceMessages{}
	updates := &fakePersistenceMessageUpdates{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveMessageUpdate: false}, messages, updates, &fakePersistenceContacts{}, zerolog.Nop())

	service.HandleReceipt(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, receiptEvent("msg-1", time.Time{}))

	if messages.findCalls != 0 || updates.createCalls != 0 {
		t.Fatalf("expected no repository calls, got find=%d create=%d", messages.findCalls, updates.createCalls)
	}
}

func TestFindMessageWithRetryHonorsContextCancellation(t *testing.T) {
	messages := &fakePersistenceMessages{alwaysErr: repository.ErrMessageNotFound}
	service := NewEventPersistenceService(EventPersistenceConfig{
		SaveMessageUpdate:        true,
		ReceiptRetryAttempts:     3,
		ReceiptRetryInitialDelay: time.Hour,
	}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.findMessageWithRetry(ctx, 42, "msg-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestHandleMessageUsesCreateOrIgnore(t *testing.T) {
	messages := &fakePersistenceMessages{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())

	service.HandleMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, textEvent("msg-1"))

	if messages.createOrIgnoreCalls != 1 {
		t.Fatalf("expected CreateOrIgnore call, got %d", messages.createOrIgnoreCalls)
	}
	if messages.created.InstanceID != 42 || messages.created.KeyID != "msg-1" || messages.created.MessageType != "extendedTextMessage" {
		t.Fatalf("unexpected created message: %+v", messages.created)
	}
}

func TestHandleMessageDispatchesUpsertWithPersistedID(t *testing.T) {
	messages := &fakePersistenceMessages{
		findResults: []findMessageResult{{message: persistedTextMessage(193, "msg-1")}},
	}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, textEvent("msg-1"))

	if messages.createOrIgnoreCalls != 1 || messages.findCalls != 1 {
		t.Fatalf("expected create and find, got create=%d find=%d", messages.createOrIgnoreCalls, messages.findCalls)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventMessagesUpsert {
		t.Fatalf("expected messages.upsert dispatch, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	data, ok := webhooks.data.(webhooksvc.MessageUpsertWebhookData)
	if !ok {
		t.Fatalf("unexpected webhook data type %T", webhooks.data)
	}
	if data.ID != 193 || data.MessageType != "extendedTextMessage" || data.MessageTimestamp != 10 {
		t.Fatalf("unexpected webhook data: %#v", data)
	}
	if data.Content == nil || data.Content["text"] != "hello" {
		t.Fatalf("unexpected content: %#v", data.Content)
	}
}

func TestHandleFBMessageDispatchesUpsertWithPersistedID(t *testing.T) {
	messages := &fakePersistenceMessages{
		findResults: []findMessageResult{{message: persistedFBMessage(194, "fb-1")}},
	}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleFBMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, fbEvent("fb-1"))

	if messages.createOrIgnoreCalls != 1 || messages.findCalls != 1 {
		t.Fatalf("expected create and find, got create=%d find=%d", messages.createOrIgnoreCalls, messages.findCalls)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventMessagesUpsert {
		t.Fatalf("expected messages.upsert dispatch, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	data := webhooks.data.(webhooksvc.MessageUpsertWebhookData)
	if data.ID != 194 || data.MessageType != "fbMessage" {
		t.Fatalf("unexpected webhook data: %#v", data)
	}
}

func TestHandleMessagePersistenceFailurePreventsDispatch(t *testing.T) {
	messages := &fakePersistenceMessages{createErr: errors.New("database down")}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataNewMessage: true}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleMessage(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, textEvent("msg-1"))

	if webhooks.dispatchCalls != 0 || messages.findCalls != 0 {
		t.Fatalf("expected no dispatch/find after persistence failure, got dispatch=%d find=%d", webhooks.dispatchCalls, messages.findCalls)
	}
}

func TestHandleReceiptDispatchesMessageUpdateAfterPersist(t *testing.T) {
	messages := &fakePersistenceMessages{
		findResults: []findMessageResult{{message: dbtypes.Message{ID: 7}}},
	}
	updates := &fakePersistenceMessageUpdates{}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{
		SaveMessageUpdate:        true,
		ReceiptRetryAttempts:     1,
		ReceiptRetryInitialDelay: time.Millisecond,
	}, messages, updates, &fakePersistenceContacts{}, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	dateTime := time.Date(2026, 7, 4, 10, 5, 12, 0, time.FixedZone("BRT", -3*3600))
	service.HandleReceipt(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, receiptEvent("msg-1", dateTime))

	if updates.createCalls != 1 || updates.input.MessageID != 7 {
		t.Fatalf("expected update before dispatch, got calls=%d input=%+v", updates.createCalls, updates.input)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventMessagesUpdated {
		t.Fatalf("expected messages.update dispatch, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	data := webhooks.data.(webhooksvc.MessageUpdateWebhookData)
	if data.MessageID != 7 || data.Status != "delivered" || !data.DateTime.Equal(dateTime.UTC()) {
		t.Fatalf("unexpected update webhook data: %#v", data)
	}
}

func TestHandleReceiptMissingMessageDoesNotDispatch(t *testing.T) {
	messages := &fakePersistenceMessages{alwaysErr: repository.ErrMessageNotFound}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{
		SaveMessageUpdate:        true,
		ReceiptRetryAttempts:     1,
		ReceiptRetryInitialDelay: time.Millisecond,
	}, messages, &fakePersistenceMessageUpdates{}, &fakePersistenceContacts{}, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleReceipt(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, receiptEvent("msg-1", time.Now()))

	if webhooks.dispatchCalls != 0 {
		t.Fatalf("expected no dispatch for missing message, got %d", webhooks.dispatchCalls)
	}
}

func TestHandleContactDispatchesPersistedContact(t *testing.T) {
	pushName := "Contato"
	profilePic := "https://example.com/avatar.jpg"
	contacts := &fakePersistenceContacts{upserted: dbtypes.Contact{
		ID:            41,
		RemoteJid:     "5531988888888@s.whatsapp.net",
		PushName:      &pushName,
		ProfilePicUrl: &profilePic,
	}}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataContacts: true}, &fakePersistenceMessages{}, &fakePersistenceMessageUpdates{}, contacts, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleContact(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, contactEvent())

	if contacts.upsertCalls != 1 {
		t.Fatalf("expected contact upsert, got %d", contacts.upsertCalls)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventContactsUpsert {
		t.Fatalf("expected contacts.upsert dispatch, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	data := webhooks.data.(webhooksvc.ContactUpsertWebhookData)
	if data.ID != 41 || data.RemoteJID != "5531988888888@s.whatsapp.net" || data.Action != "upserted" {
		t.Fatalf("unexpected contact webhook data: %#v", data)
	}
	if data.LID == nil || *data.LID != "279847268053216@lid" {
		t.Fatalf("expected lid in payload, got %#v", data.LID)
	}
}

func TestHandlePushNameDispatchesContactsUpdateAfterPersist(t *testing.T) {
	pushName := "Novo nome"
	contacts := &fakePersistenceContacts{upserted: dbtypes.Contact{
		ID:        42,
		RemoteJid: "5531988888888@s.whatsapp.net",
		PushName:  &pushName,
	}}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataContacts: true}, &fakePersistenceMessages{}, &fakePersistenceMessageUpdates{}, contacts, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandlePushName(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, pushNameEvent())

	if contacts.upsertCalls != 1 || contacts.input.PushName == nil || *contacts.input.PushName != "Novo nome" {
		t.Fatalf("expected contact upsert before dispatch, calls=%d input=%+v", contacts.upsertCalls, contacts.input)
	}
	if webhooks.dispatchCalls != 1 || webhooks.event != dbtypes.WebhookEventContactsUpdated {
		t.Fatalf("expected contacts.update dispatch, got calls=%d event=%s", webhooks.dispatchCalls, webhooks.event)
	}
	data, ok := webhooks.data.([]webhooksvc.ContactUpdateWebhookData)
	if !ok || len(data) != 1 {
		t.Fatalf("expected one contact update item, got %T %#v", webhooks.data, webhooks.data)
	}
	if data[0].ID != 42 || data[0].Source != "pushName" || data[0].Action != "updated" || data[0].PushName == nil || *data[0].PushName != "Novo nome" {
		t.Fatalf("unexpected contacts.update data: %#v", data[0])
	}
}

func TestHandleBusinessNameDispatchesContactsUpdateWithoutSchemaColumn(t *testing.T) {
	contacts := &fakePersistenceContacts{upserted: dbtypes.Contact{
		ID:        43,
		RemoteJid: "5531988888888@s.whatsapp.net",
	}}
	webhooks := &fakePersistenceWebhooks{}
	service := NewEventPersistenceService(EventPersistenceConfig{SaveDataContacts: true}, &fakePersistenceMessages{}, &fakePersistenceMessageUpdates{}, contacts, zerolog.Nop())
	service.SetWebhookDispatcher(fakeInstanceFinder(), webhooks)

	service.HandleBusinessName(context.Background(), &ManagedWhatsAppClient{InstanceID: "42", InstanceName: "codechat"}, businessNameEvent())

	if contacts.upsertCalls != 1 || contacts.input.PushName != nil {
		t.Fatalf("expected upsert without pushName overwrite, calls=%d input=%+v", contacts.upsertCalls, contacts.input)
	}
	data := webhooks.data.([]webhooksvc.ContactUpdateWebhookData)
	if len(data) != 1 || data[0].BusinessName == nil || *data[0].BusinessName != "Empresa" || data[0].Source != "businessName" {
		t.Fatalf("unexpected businessName contact update data: %#v", data)
	}
}

type findMessageResult struct {
	message dbtypes.Message
	err     error
}

type fakePersistenceMessages struct {
	createOrIgnoreCalls int
	created             dbtypes.CreateMessageInput
	findCalls           int
	instanceID          int32
	keyID               string
	findResults         []findMessageResult
	alwaysErr           error
	createErr           error
}

func (f *fakePersistenceMessages) Create(context.Context, dbtypes.CreateMessageInput) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (f *fakePersistenceMessages) CreateOrIgnore(_ context.Context, input dbtypes.CreateMessageInput) error {
	f.createOrIgnoreCalls++
	f.created = input
	return f.createErr
}

func (f *fakePersistenceMessages) FindByIDForInstance(context.Context, int32, int32) (dbtypes.Message, error) {
	return dbtypes.Message{}, repository.ErrMessageNotFound
}

func (f *fakePersistenceMessages) FindByKeyIDForInstance(_ context.Context, instanceID int32, keyID string) (dbtypes.Message, error) {
	f.findCalls++
	f.instanceID = instanceID
	f.keyID = keyID
	if f.alwaysErr != nil {
		return dbtypes.Message{}, f.alwaysErr
	}
	if len(f.findResults) == 0 {
		return dbtypes.Message{}, repository.ErrMessageNotFound
	}
	next := f.findResults[0]
	f.findResults = f.findResults[1:]
	return next.message, next.err
}

func (f *fakePersistenceMessages) FindByIDsForInstance(context.Context, int32, []int32) ([]dbtypes.Message, error) {
	return nil, nil
}

func (f *fakePersistenceMessages) FindOutgoingByIDForInstance(context.Context, int32, int32) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (f *fakePersistenceMessages) FindOutgoingByKeyIDForInstance(context.Context, int32, string) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (f *fakePersistenceMessages) MarkReadForInstance(context.Context, int32, []int32) error {
	return nil
}

func (f *fakePersistenceMessages) UpdateContentForInstance(context.Context, int32, int32, json.RawMessage) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (f *fakePersistenceMessages) Count(context.Context, int32, dbtypes.MessageFilters) (int64, error) {
	return 0, nil
}

func (f *fakePersistenceMessages) List(context.Context, int32, dbtypes.ListMessagesInput) (dbtypes.MessageListResult, error) {
	return dbtypes.MessageListResult{}, nil
}

type fakePersistenceMessageUpdates struct {
	createCalls int
	input       dbtypes.CreateMessageUpdateInput
}

func (f *fakePersistenceMessageUpdates) Create(context.Context, dbtypes.CreateMessageUpdateInput) (dbtypes.MessageUpdate, error) {
	return dbtypes.MessageUpdate{}, nil
}

func (f *fakePersistenceMessageUpdates) CreateOrIgnore(_ context.Context, input dbtypes.CreateMessageUpdateInput) error {
	f.createCalls++
	f.input = input
	return nil
}

func (f *fakePersistenceMessageUpdates) ListByMessageID(context.Context, int32) ([]dbtypes.MessageUpdate, error) {
	return nil, nil
}

type fakePersistenceContacts struct {
	upsertCalls int
	input       dbtypes.CreateContactInput
	upserted    dbtypes.Contact
	err         error
}

func (f *fakePersistenceContacts) Create(context.Context, dbtypes.CreateContactInput) (dbtypes.Contact, error) {
	return dbtypes.Contact{}, nil
}

func (f *fakePersistenceContacts) Upsert(_ context.Context, input dbtypes.CreateContactInput) (dbtypes.Contact, error) {
	f.upsertCalls++
	f.input = input
	if f.err != nil {
		return dbtypes.Contact{}, f.err
	}
	return f.upserted, nil
}

func (f *fakePersistenceContacts) List(context.Context, int32, dbtypes.ContactFilters) ([]dbtypes.Contact, error) {
	return nil, nil
}

func receiptEvent(keyID string, timestamp time.Time) *events.Receipt {
	return &events.Receipt{
		MessageIDs: []watypes.MessageID{watypes.MessageID(keyID)},
		Timestamp:  timestamp,
		Type:       watypes.ReceiptTypeDelivered,
	}
}

func textEvent(keyID string) *events.Message {
	jid := watypes.NewJID("5511999999999", watypes.DefaultUserServer)
	return &events.Message{
		Info: watypes.MessageInfo{
			ID:        watypes.MessageID(keyID),
			Timestamp: time.Unix(10, 0).UTC(),
			MessageSource: watypes.MessageSource{
				Chat:     jid,
				Sender:   jid,
				IsFromMe: false,
			},
		},
		Message: &wae2e.Message{Conversation: proto.String("hello")},
	}
}

func fbEvent(keyID string) *events.FBMessage {
	jid := watypes.NewJID("5511999999999", watypes.DefaultUserServer)
	return &events.FBMessage{
		Info: watypes.MessageInfo{
			ID:        watypes.MessageID(keyID),
			Timestamp: time.Unix(11, 0).UTC(),
			MessageSource: watypes.MessageSource{
				Chat:     jid,
				Sender:   jid,
				IsFromMe: true,
			},
		},
		Message: &waConsumerApplication.ConsumerApplication{},
	}
}

func contactEvent() *events.Contact {
	jid := watypes.NewJID("5531988888888", watypes.DefaultUserServer)
	return &events.Contact{
		JID: jid,
		Action: &waSyncAction.ContactAction{
			FullName: proto.String("Contato"),
			PnJID:    proto.String(jid.String()),
			LidJID:   proto.String("279847268053216@lid"),
		},
	}
}

func pushNameEvent() *events.PushName {
	return &events.PushName{
		JID:         watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		JIDAlt:      watypes.NewJID("279847268053216", watypes.HiddenUserServer),
		OldPushName: "Antigo",
		NewPushName: "Novo nome",
	}
}

func businessNameEvent() *events.BusinessName {
	return &events.BusinessName{
		JID:             watypes.NewJID("5531988888888", watypes.DefaultUserServer),
		OldBusinessName: "Antiga",
		NewBusinessName: "Empresa",
	}
}

func persistedTextMessage(id int32, keyID string) dbtypes.Message {
	isGroup := false
	content := json.RawMessage(`{"text":"hello"}`)
	return dbtypes.Message{
		ID:               id,
		KeyID:            keyID,
		MessageType:      "extendedTextMessage",
		Content:          content,
		MessageTimestamp: 10,
		Device:           dbtypes.DeviceMessageUnknown,
		IsGroup:          &isGroup,
		InstanceID:       42,
	}
}

func persistedFBMessage(id int32, keyID string) dbtypes.Message {
	isGroup := false
	return dbtypes.Message{
		ID:               id,
		KeyID:            keyID,
		MessageType:      "fbMessage",
		Content:          json.RawMessage(`{}`),
		MessageTimestamp: 11,
		Device:           dbtypes.DeviceMessageUnknown,
		IsGroup:          &isGroup,
		InstanceID:       42,
	}
}

type fakePersistenceWebhooks struct {
	dispatchCalls int
	instance      webhooksvc.WebhookInstance
	event         dbtypes.WebhookEvent
	data          any
}

func (f *fakePersistenceWebhooks) Dispatch(_ context.Context, instance webhooksvc.WebhookInstance, event dbtypes.WebhookEvent, data any) error {
	f.dispatchCalls++
	f.instance = instance
	f.event = event
	f.data = data
	return nil
}

func (f *fakePersistenceWebhooks) Shutdown(context.Context) error {
	return nil
}

type fakePersistenceInstanceFinder struct {
	instance dbtypes.InstanceWithAuth
	err      error
}

func fakeInstanceFinder() fakePersistenceInstanceFinder {
	owner := "5531999999999@s.whatsapp.net"
	return fakePersistenceInstanceFinder{instance: dbtypes.InstanceWithAuth{Instance: dbtypes.Instance{
		ID:                 42,
		Name:               "codechat",
		ConnectionStatus:   dbtypes.InstanceConnectionStatusOnline,
		WhatsAppOwnerJid:   &owner,
		ExternalAttributes: json.RawMessage(`{"tenantId":"019f"}`),
	}}}
}

func (f fakePersistenceInstanceFinder) FindByName(context.Context, string) (dbtypes.InstanceWithAuth, error) {
	if f.err != nil {
		return dbtypes.InstanceWithAuth{}, f.err
	}
	return f.instance, nil
}
