package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"

	"whatsapp-go-api/internal/database/migrations"
	db "whatsapp-go-api/internal/database/sqlc"
	"whatsapp-go-api/internal/database/types"
)

func TestValidateWebhookEventsJSONRejectsUnknownEvent(t *testing.T) {
	err := validateWebhookEventsJSON(json.RawMessage(`{"unknown":true}`))
	if !errors.Is(err, ErrInvalidWebhookEvent) {
		t.Fatalf("expected ErrInvalidWebhookEvent, got %v", err)
	}
}

func TestValidateWebhookEventsJSONAcceptsNewEvents(t *testing.T) {
	err := validateWebhookEventsJSON(json.RawMessage(`{
		"contactsUpdated": true,
		"groupsUpdated": true,
		"callUpsert": true,
		"labelsAssociation": true,
		"labelsEdit": true,
		"messagesDeleted": true,
		"profilePictureUpdated": true,
		"settingsUpdated": true
	}`))
	if err != nil {
		t.Fatalf("expected new webhook event fields to be valid, got %v", err)
	}
}

func TestInstanceDependenciesErrorUnwrap(t *testing.T) {
	err := &InstanceDependenciesError{InstanceID: 1, Messages: 2}
	if !errors.Is(err, ErrInstanceHasDependencies) {
		t.Fatal("expected errors.Is to match ErrInstanceHasDependencies")
	}
}

func TestRepositoriesWithPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration tests")
	}

	ctx := context.Background()
	pool := newTestPool(ctx, t, databaseURL)
	defer pool.Close()

	logger := zerolog.Nop()
	instanceRepo := NewInstanceRepository(pool, logger)
	authRepo := NewAuthRepository(pool, logger)
	messageRepo := NewMessageRepository(pool, logger)
	messageUpdateRepo := NewMessageUpdateRepository(pool, logger)
	chatRepo := NewChatRepository(pool, logger)
	contactRepo := NewContactRepository(pool, logger)
	webhookRepo := NewWebhookRepository(pool, logger)

	description := "primary"
	attributes := json.RawMessage(`{"env":"test"}`)
	instance, err := instanceRepo.Create(ctx, types.CreateInstanceInput{
		Name:               "main",
		Description:        &description,
		ExternalAttributes: attributes,
	})
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if instance.Instance.Status != types.InstanceStatusOnline {
		t.Fatalf("expected default ONLINE, got %s", instance.Instance.Status)
	}
	if instance.Instance.ConnectionStatus != types.InstanceConnectionStatusOffline {
		t.Fatalf("expected default WhatsApp connection offline, got %s", instance.Instance.ConnectionStatus)
	}

	if _, err := instanceRepo.Create(ctx, types.CreateInstanceInput{Name: "main"}); !errors.Is(err, ErrInstanceNameAlreadyExists) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}

	auth, err := authRepo.Create(ctx, types.CreateAuthInput{Token: "token-1", InstanceID: instance.Instance.ID})
	if err != nil {
		t.Fatalf("create auth: %v", err)
	}
	if auth.InstanceID != instance.Instance.ID {
		t.Fatalf("unexpected auth instance id %d", auth.InstanceID)
	}

	found, err := instanceRepo.FindByName(ctx, "main")
	if err != nil {
		t.Fatalf("find instance: %v", err)
	}
	if found.Auth == nil || found.Auth.Token != "token-1" {
		t.Fatalf("expected joined auth token, got %#v", found.Auth)
	}

	listed, err := instanceRepo.List(ctx)
	if err != nil {
		t.Fatalf("list instances: %v", err)
	}
	if len(listed) != 1 || listed[0].Auth == nil {
		t.Fatalf("expected one instance with auth, got %#v", listed)
	}

	newName := "renamed"
	nullJSON := json.RawMessage("null")
	updated, err := instanceRepo.Update(ctx, instance.Instance.ID, types.UpdateInstanceInput{
		Name:               &newName,
		Description:        types.OptionalField[string]{Set: true},
		ExternalAttributes: types.OptionalField[json.RawMessage]{Set: true, Value: &nullJSON},
	})
	if err != nil {
		t.Fatalf("update instance: %v", err)
	}
	if updated.Instance.Name != newName || updated.Instance.Description != nil || string(updated.Instance.ExternalAttributes) != "null" {
		t.Fatalf("unexpected updated instance: %#v", updated.Instance)
	}

	if err := instanceRepo.UpdateStatus(ctx, instance.Instance.ID, types.InstanceStatusOffline); err != nil {
		t.Fatalf("update instance status: %v", err)
	}

	message, err := messageRepo.Create(ctx, types.CreateMessageInput{
		KeyID:            "key-1",
		KeyFromMe:        true,
		MessageType:      "conversation",
		Content:          json.RawMessage(`{"text":"hello"}`),
		MessageTimestamp: 100,
		Device:           types.DeviceMessageIOS,
		InstanceID:       instance.Instance.ID,
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if _, err := messageUpdateRepo.Create(ctx, types.CreateMessageUpdateInput{
		DateTime:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Status:    "READ",
		MessageID: message.ID,
	}); err != nil {
		t.Fatalf("create message update: %v", err)
	}

	status := "READ"
	count, err := messageRepo.Count(ctx, instance.Instance.ID, types.MessageFilters{MessageStatus: &status})
	if err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	page, err := messageRepo.List(ctx, instance.Instance.ID, types.ListMessagesInput{
		Limit:     10,
		Direction: types.CursorDirectionNext,
		Filters:   types.MessageFilters{ID: &message.ID},
	})
	if err != nil {
		t.Fatalf("list message by id: %v", err)
	}
	if page.Messages.Total != 1 || len(page.Messages.Records) != 1 || len(page.Messages.Records[0].MessageUpdate) != 1 {
		t.Fatalf("unexpected message page: %#v", page.Messages)
	}

	groupJid := "123@g.us"
	chatType := types.ChatTypeGroup
	if _, err := chatRepo.Create(ctx, types.CreateChatInput{RemoteJid: "123@s.whatsapp.net", InstanceID: instance.Instance.ID}); err != nil {
		t.Fatalf("create chat: %v", err)
	}
	if _, err := chatRepo.Create(ctx, types.CreateChatInput{RemoteJid: groupJid, InstanceID: instance.Instance.ID}); err != nil {
		t.Fatalf("create group chat: %v", err)
	}
	groupChats, err := chatRepo.List(ctx, instance.Instance.ID, &chatType)
	if err != nil {
		t.Fatalf("list group chats: %v", err)
	}
	if len(groupChats) != 1 || groupChats[0].RemoteJid != groupJid {
		t.Fatalf("expected only suffix @g.us group, got %#v", groupChats)
	}

	pushName := "Alice"
	contact, err := contactRepo.Create(ctx, types.CreateContactInput{RemoteJid: "alice@s.whatsapp.net", PushName: &pushName, InstanceID: instance.Instance.ID})
	if err != nil {
		t.Fatalf("create contact: %v", err)
	}
	wrongPushName := "Bob"
	contacts, err := contactRepo.List(ctx, instance.Instance.ID, types.ContactFilters{ID: &contact.ID, PushName: &wrongPushName})
	if err != nil {
		t.Fatalf("list contact: %v", err)
	}
	if len(contacts) != 1 || contacts[0].ID != contact.ID {
		t.Fatalf("expected ID filter to ignore other filters, got %#v", contacts)
	}

	webhook, err := webhookRepo.Create(ctx, types.CreateWebhookInput{
		URL:        "https://example.com/webhook",
		Events:     json.RawMessage(`{"messagesUpsert":true,"connectionUpdated":true}`),
		InstanceID: instance.Instance.ID,
	})
	if err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	if _, err := webhookRepo.Create(ctx, types.CreateWebhookInput{URL: "https://example.com/2", InstanceID: instance.Instance.ID}); !errors.Is(err, ErrWebhookAlreadyExists) {
		t.Fatalf("expected duplicate webhook error, got %v", err)
	}
	foundWebhook, err := webhookRepo.FindByInstanceName(ctx, newName)
	if err != nil {
		t.Fatalf("find webhook by instance name: %v", err)
	}
	if foundWebhook.ID != webhook.ID {
		t.Fatalf("unexpected webhook %#v", foundWebhook)
	}
	merged, err := webhookRepo.UpsertEvents(ctx, webhook.ID, map[string]bool{"messagesUpsert": false})
	if err != nil {
		t.Fatalf("merge events: %v", err)
	}
	if !strings.Contains(string(merged.Events), `"connectionUpdated": true`) || !strings.Contains(string(merged.Events), `"messagesUpsert": false`) {
		t.Fatalf("events were not merged as expected: %s", string(merged.Events))
	}
	cleared, err := webhookRepo.UpsertEvents(ctx, webhook.ID, map[string]bool{})
	if err != nil {
		t.Fatalf("clear events: %v", err)
	}
	if string(cleared.Events) != "{}" {
		t.Fatalf("expected cleared events, got %s", string(cleared.Events))
	}

	err = instanceRepo.Delete(ctx, instance.Instance.ID, false)
	var depErr *InstanceDependenciesError
	if !errors.As(err, &depErr) || depErr.Messages != 1 || depErr.Chats != 2 || depErr.Contacts != 1 || depErr.Webhooks != 1 {
		t.Fatalf("expected dependency counts, got %#v err=%v", depErr, err)
	}
	if err := instanceRepo.Delete(ctx, instance.Instance.ID, true); err != nil {
		t.Fatalf("force delete instance: %v", err)
	}

	exists, err := db.New(pool).InstanceExists(ctx, instance.Instance.ID)
	if err != nil {
		t.Fatalf("check cascade instance exists: %v", err)
	}
	if exists {
		t.Fatal("expected instance to be deleted")
	}
}

func newTestPool(ctx context.Context, t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect admin pool: %v", err)
	}

	schemaName := "test_" + time.Now().Format("150405000000")
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA "`+schemaName+`"`); err != nil {
		adminPool.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS "`+schemaName+`" CASCADE`)
		adminPool.Close()
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("connect test pool: %v", err)
	}

	if err := migrations.Run(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("apply migrations: %v", err)
	}

	return pool
}
