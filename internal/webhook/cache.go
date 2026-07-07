package webhook

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"whatsapp-go-api/internal/database/types"
)

type CachedWebhook struct {
	ID           int64
	InstanceID   int64
	InstanceName string
	URL          string
	Enabled      bool
	Events       types.WebhookEvents
	UpdatedAt    time.Time
}

type WebhookCache interface {
	Get(instanceID int64, instanceName string) (CachedWebhook, bool)
	Set(instanceID int64, instanceName string, webhook CachedWebhook)
	Delete(instanceID int64, instanceName string)
	Load(ctx context.Context, webhooks []CachedWebhook)
	Clear()
}

type MemoryWebhookCache struct {
	mu    sync.RWMutex
	items map[string]CachedWebhook
}

func NewMemoryWebhookCache() *MemoryWebhookCache {
	return &MemoryWebhookCache{items: make(map[string]CachedWebhook)}
}

func (c *MemoryWebhookCache) Get(instanceID int64, instanceName string) (CachedWebhook, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	webhook, ok := c.items[webhookCacheKey(instanceID, instanceName)]
	return webhook, ok
}

func (c *MemoryWebhookCache) Set(instanceID int64, instanceName string, webhook CachedWebhook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := webhookCacheKey(instanceID, instanceName)
	if !webhook.Enabled {
		delete(c.items, key)
		return
	}
	webhook.InstanceID = instanceID
	webhook.InstanceName = strings.TrimSpace(instanceName)
	c.items[key] = webhook
}

func (c *MemoryWebhookCache) Delete(instanceID int64, instanceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, webhookCacheKey(instanceID, instanceName))
}

func (c *MemoryWebhookCache) Load(_ context.Context, webhooks []CachedWebhook) {
	next := make(map[string]CachedWebhook, len(webhooks))
	for _, webhook := range webhooks {
		if !webhook.Enabled {
			continue
		}
		next[webhookCacheKey(webhook.InstanceID, webhook.InstanceName)] = webhook
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = next
}

func (c *MemoryWebhookCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]CachedWebhook)
}

func webhookCacheKey(instanceID int64, instanceName string) string {
	return fmt.Sprintf("%d:%s", instanceID, strings.ToLower(strings.TrimSpace(instanceName)))
}
