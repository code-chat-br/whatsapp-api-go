package chat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
)

func TestResolveMediaDataRequestUsesInstanceScopedLookups(t *testing.T) {
	id := int64(1855)
	keyID := "ABC"
	repo := &fakeMediaMessageRepository{message: messageForMediaTest(MediaTypeImage, baseMediaJSON(`"mimetype":"image/jpeg"`))}
	service := &ChatService{messages: repo}

	_, _, _, messageID, err := service.resolveMediaDataRequest(context.Background(), 10, MediaDataModeID, MediaDataRequest{ID: &id})
	if err != nil {
		t.Fatalf("resolve by id error = %v", err)
	}
	if messageID != repo.message.ID || repo.findIDInstanceID != 10 || repo.findID != int32(id) {
		t.Fatalf("FindByIDForInstance called with instance=%d id=%d messageID=%d", repo.findIDInstanceID, repo.findID, messageID)
	}

	_, _, _, _, err = service.resolveMediaDataRequest(context.Background(), 20, MediaDataModeKeyID, MediaDataRequest{KeyID: &keyID})
	if err != nil {
		t.Fatalf("resolve by keyId error = %v", err)
	}
	if repo.findKeyInstanceID != 20 || repo.findKeyID != keyID {
		t.Fatalf("FindByKeyIDForInstance called with instance=%d keyID=%q", repo.findKeyInstanceID, repo.findKeyID)
	}
}

func TestResolveMediaDataRequestLookupErrors(t *testing.T) {
	id := int64(1)
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "not found", err: repository.ErrMessageNotFound, wantErr: ErrMediaMessageNotFound},
		{name: "database", err: errors.New("database unavailable"), wantErr: ErrDatabaseOperation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ChatService{messages: &fakeMediaMessageRepository{err: tt.err}}
			_, _, _, _, err := service.resolveMediaDataRequest(context.Background(), 10, MediaDataModeID, MediaDataRequest{ID: &id})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolve error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveMediaDataRequestPayloadDoesNotQueryRepository(t *testing.T) {
	repo := &fakeMediaMessageRepository{}
	keyID := "ABC"
	service := &ChatService{messages: repo}
	messageType, content, gotKeyID, messageID, err := service.resolveMediaDataRequest(
		context.Background(),
		10,
		MediaDataModePayload,
		MediaDataRequest{KeyID: &keyID, MessageType: MediaTypeImage, Content: json.RawMessage(`{"directPath":"/media/path"}`)},
	)
	if err != nil {
		t.Fatalf("resolve payload error = %v", err)
	}
	if repo.findIDCalls != 0 || repo.findKeyCalls != 0 {
		t.Fatalf("payload mode queried repository: id=%d key=%d", repo.findIDCalls, repo.findKeyCalls)
	}
	if messageType != MediaTypeImage || string(content) == "" || gotKeyID != keyID || messageID != 0 {
		t.Fatalf("unexpected payload resolution: type=%q content=%s key=%q id=%d", messageType, content, gotKeyID, messageID)
	}
}

type fakeMediaMessageRepository struct {
	message dbtypes.Message
	err     error

	findIDCalls      int
	findIDInstanceID int32
	findID           int32

	findKeyCalls      int
	findKeyInstanceID int32
	findKeyID         string
}

func (r *fakeMediaMessageRepository) Create(context.Context, dbtypes.CreateMessageInput) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (r *fakeMediaMessageRepository) CreateOrIgnore(context.Context, dbtypes.CreateMessageInput) error {
	return nil
}

func (r *fakeMediaMessageRepository) FindByIDForInstance(_ context.Context, instanceID int32, id int32) (dbtypes.Message, error) {
	r.findIDCalls++
	r.findIDInstanceID = instanceID
	r.findID = id
	if r.err != nil {
		return dbtypes.Message{}, r.err
	}
	return r.message, nil
}

func (r *fakeMediaMessageRepository) FindByKeyIDForInstance(_ context.Context, instanceID int32, keyID string) (dbtypes.Message, error) {
	r.findKeyCalls++
	r.findKeyInstanceID = instanceID
	r.findKeyID = keyID
	if r.err != nil {
		return dbtypes.Message{}, r.err
	}
	return r.message, nil
}

func (r *fakeMediaMessageRepository) FindByIDsForInstance(context.Context, int32, []int32) ([]dbtypes.Message, error) {
	return nil, nil
}

func (r *fakeMediaMessageRepository) FindOutgoingByIDForInstance(context.Context, int32, int32) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (r *fakeMediaMessageRepository) FindOutgoingByKeyIDForInstance(context.Context, int32, string) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (r *fakeMediaMessageRepository) MarkReadForInstance(context.Context, int32, []int32) error {
	return nil
}

func (r *fakeMediaMessageRepository) UpdateContentForInstance(context.Context, int32, int32, json.RawMessage) (dbtypes.Message, error) {
	return dbtypes.Message{}, nil
}

func (r *fakeMediaMessageRepository) Count(context.Context, int32, dbtypes.MessageFilters) (int64, error) {
	return 0, nil
}

func (r *fakeMediaMessageRepository) List(context.Context, int32, dbtypes.ListMessagesInput) (dbtypes.MessageListResult, error) {
	return dbtypes.MessageListResult{}, nil
}
