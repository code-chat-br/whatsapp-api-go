package message

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestMessageProcessingManagerSubmitQueueFull(t *testing.T) {
	manager, err := NewMessageProcessingManager(context.Background(), &MessageService{}, ProcessingConfig{
		Workers:           1,
		QueueSize:         1,
		ProcessingTimeout: time.Second,
		GroupInfoTimeout:  time.Second,
		SendTimeout:       time.Second,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewMessageProcessingManager() error = %v", err)
	}

	if err := manager.Submit(MessageProcessingJob{ProcessID: "one"}); err != nil {
		t.Fatalf("first Submit() error = %v", err)
	}
	if err := manager.Submit(MessageProcessingJob{ProcessID: "two"}); !errors.Is(err, ErrMessageQueueFull) {
		t.Fatalf("expected ErrMessageQueueFull, got %v", err)
	}
}

func TestMessageProcessingManagerSubmitAfterShutdown(t *testing.T) {
	manager, err := NewMessageProcessingManager(context.Background(), &MessageService{}, ProcessingConfig{
		Workers:           1,
		QueueSize:         1,
		ProcessingTimeout: time.Second,
		GroupInfoTimeout:  time.Second,
		SendTimeout:       time.Second,
	}, zerolog.Nop())
	if err != nil {
		t.Fatalf("NewMessageProcessingManager() error = %v", err)
	}
	manager.Start()
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := manager.Submit(MessageProcessingJob{ProcessID: "after"}); !errors.Is(err, ErrMessageProcessorStopped) {
		t.Fatalf("expected ErrMessageProcessorStopped, got %v", err)
	}
}
