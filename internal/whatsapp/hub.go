package whatsapp

import (
	"context"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

type ManagedWhatsAppClient struct {
	InstanceID   string
	InstanceName string
	DeviceJID    string
	OwnerJID     string
	Client       *whatsmeow.Client
	Context      context.Context
	Cancel       context.CancelFunc
	Status       string
	StartedAt    time.Time
	ConnectedAt  time.Time

	ConnectedSignal chan struct{}
	ConnectedOnce   sync.Once

	mu sync.RWMutex
}

func (c *ManagedWhatsAppClient) SignalConnected() {
	c.ConnectedOnce.Do(func() {
		c.mu.Lock()
		c.ConnectedAt = time.Now().UTC()
		c.mu.Unlock()
		close(c.ConnectedSignal)
	})
}

func (c *ManagedWhatsAppClient) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Client != nil && c.Client.IsConnected() && c.Client.IsLoggedIn()
}

type ClientHub interface {
	Reserve(instanceID string, instanceName string) error
	Register(client *ManagedWhatsAppClient) error
	GetByInstanceID(instanceID string) (*ManagedWhatsAppClient, bool)
	Remove(instanceID string) (*ManagedWhatsAppClient, bool)
	Exists(instanceID string) bool
	List() []*ManagedWhatsAppClient
	Shutdown(ctx context.Context) error
}

type WhatsAppClientHub struct {
	mu                  sync.RWMutex
	clientsByInstanceID map[string]*ManagedWhatsAppClient
	reservations        map[string]struct{}
}

func NewClientHub() *WhatsAppClientHub {
	return &WhatsAppClientHub{
		clientsByInstanceID: make(map[string]*ManagedWhatsAppClient),
		reservations:        make(map[string]struct{}),
	}
}

func (h *WhatsAppClientHub) Reserve(instanceID string, instanceName string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clientsByInstanceID[instanceID]; ok {
		return ErrInstanceConnected
	}
	if _, ok := h.reservations[instanceID]; ok {
		return ErrConnectionInProgress
	}
	h.reservations[instanceID] = struct{}{}
	return nil
}

func (h *WhatsAppClientHub) Register(client *ManagedWhatsAppClient) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clientsByInstanceID[client.InstanceID]; ok {
		return ErrInstanceConnected
	}
	delete(h.reservations, client.InstanceID)
	h.clientsByInstanceID[client.InstanceID] = client
	return nil
}

func (h *WhatsAppClientHub) GetByInstanceID(instanceID string) (*ManagedWhatsAppClient, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	client, ok := h.clientsByInstanceID[instanceID]
	return client, ok
}

func (h *WhatsAppClientHub) Remove(instanceID string) (*ManagedWhatsAppClient, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client, ok := h.clientsByInstanceID[instanceID]
	delete(h.clientsByInstanceID, instanceID)
	delete(h.reservations, instanceID)
	return client, ok
}

func (h *WhatsAppClientHub) Exists(instanceID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, hasClient := h.clientsByInstanceID[instanceID]
	_, reserved := h.reservations[instanceID]
	return hasClient || reserved
}

func (h *WhatsAppClientHub) List() []*ManagedWhatsAppClient {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := make([]*ManagedWhatsAppClient, 0, len(h.clientsByInstanceID))
	for _, client := range h.clientsByInstanceID {
		clients = append(clients, client)
	}
	return clients
}

func (h *WhatsAppClientHub) Shutdown(ctx context.Context) error {
	for _, client := range h.List() {
		if client.Cancel != nil {
			client.Cancel()
		}
		if client.Client != nil {
			client.Client.Disconnect()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		h.Remove(client.InstanceID)
	}
	return nil
}
