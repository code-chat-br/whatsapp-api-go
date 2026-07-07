package group

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mau.fi/whatsmeow"
	watypes "go.mau.fi/whatsmeow/types"

	"whatsapp-go-api/internal/database/repository"
	dbtypes "whatsapp-go-api/internal/database/types"
	"whatsapp-go-api/internal/whatsapp"
)

const maxGroupPictureBytes = 16 * 1024 * 1024

type ConnectedClientResolver interface {
	ResolveConnectedClient(ctx context.Context, instanceName string) (*whatsapp.ManagedWhatsAppClient, error)
}

type Service interface {
	Create(ctx context.Context, instanceName string, bearerToken string, input CreateRequest) (InfoResponse, error)
	UpdatePicture(ctx context.Context, instanceName string, bearerToken string, input UpdatePictureRequest) (InfoResponse, error)
	InviteCode(ctx context.Context, instanceName string, bearerToken string, groupJID string) (InviteCodeResponse, error)
	RevokeInviteCode(ctx context.Context, instanceName string, bearerToken string, groupJID string) error
	UpdateParticipant(ctx context.Context, instanceName string, bearerToken string, groupJID string, input UpdateParticipantRequest) error
	Leave(ctx context.Context, instanceName string, bearerToken string, groupJID string) error
}

type GroupService struct {
	instances repository.InstanceRepository
	clients   ConnectedClientResolver
	http      *http.Client
	logger    zerolog.Logger
}

func NewService(instances repository.InstanceRepository, clients ConnectedClientResolver, logger zerolog.Logger) *GroupService {
	return &GroupService{
		instances: instances,
		clients:   clients,
		http:      &http.Client{Timeout: 20 * time.Second},
		logger:    logger.With().Str("component", "group_service").Logger(),
	}
}

func (s *GroupService) Create(ctx context.Context, instanceName string, bearerToken string, input CreateRequest) (InfoResponse, error) {
	if err := validateCreate(input); err != nil {
		return InfoResponse{}, err
	}
	instance, client, err := s.authorizedClient(ctx, instanceName, bearerToken)
	if err != nil {
		return InfoResponse{}, err
	}
	participants, err := parseParticipants(input.Participants)
	if err != nil {
		return InfoResponse{}, err
	}
	info, err := client.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         strings.TrimSpace(input.Subject),
		Participants: participants,
	})
	if err != nil {
		return InfoResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	if input.Description != nil && strings.TrimSpace(*input.Description) != "" {
		if err := client.SetGroupTopic(ctx, info.JID, info.TopicID, "", strings.TrimSpace(*input.Description)); err != nil {
			return InfoResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
		}
		info, err = client.GetGroupInfo(ctx, info.JID)
		if err != nil {
			return InfoResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
		}
	}
	s.logOperation(instance, "create", info.JID.String())
	return groupInfoResponse(info), nil
}

func (s *GroupService) UpdatePicture(ctx context.Context, instanceName string, bearerToken string, input UpdatePictureRequest) (InfoResponse, error) {
	if err := validateUpdatePicture(input); err != nil {
		return InfoResponse{}, err
	}
	instance, client, err := s.authorizedClient(ctx, instanceName, bearerToken)
	if err != nil {
		return InfoResponse{}, err
	}
	jid, err := parseGroupJID(input.GroupJID)
	if err != nil {
		return InfoResponse{}, err
	}
	image, err := s.downloadImage(ctx, input.Image)
	if err != nil {
		return InfoResponse{}, err
	}
	if _, err := client.SetGroupPhoto(ctx, jid, image); err != nil {
		return InfoResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	info, err := client.GetGroupInfo(ctx, jid)
	if err != nil {
		return InfoResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	s.logOperation(instance, "update-picture", jid.String())
	return groupInfoResponse(info), nil
}

func (s *GroupService) InviteCode(ctx context.Context, instanceName string, bearerToken string, groupJID string) (InviteCodeResponse, error) {
	instance, client, jid, err := s.authorizedGroup(ctx, instanceName, bearerToken, groupJID)
	if err != nil {
		return InviteCodeResponse{}, err
	}
	invitation, err := client.GetGroupInviteLink(ctx, jid, false)
	if err != nil {
		return InviteCodeResponse{}, fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	s.logOperation(instance, "invite-code", jid.String())
	return InviteCodeResponse{Invitation: invitation}, nil
}

func (s *GroupService) RevokeInviteCode(ctx context.Context, instanceName string, bearerToken string, groupJID string) error {
	instance, client, jid, err := s.authorizedGroup(ctx, instanceName, bearerToken, groupJID)
	if err != nil {
		return err
	}
	if _, err := client.GetGroupInviteLink(ctx, jid, true); err != nil {
		return fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	s.logOperation(instance, "revoke-invite-code", jid.String())
	return nil
}

func (s *GroupService) UpdateParticipant(ctx context.Context, instanceName string, bearerToken string, groupJID string, input UpdateParticipantRequest) error {
	if err := validateUpdateParticipant(input); err != nil {
		return err
	}
	instance, client, jid, err := s.authorizedGroup(ctx, instanceName, bearerToken, groupJID)
	if err != nil {
		return err
	}
	participants, err := parseParticipants(input.Participants)
	if err != nil {
		return err
	}
	if _, err := client.UpdateGroupParticipants(ctx, jid, participants, whatsmeow.ParticipantChange(strings.TrimSpace(input.Action))); err != nil {
		return fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	s.logOperation(instance, "update-participant", jid.String())
	return nil
}

func (s *GroupService) Leave(ctx context.Context, instanceName string, bearerToken string, groupJID string) error {
	instance, client, jid, err := s.authorizedGroup(ctx, instanceName, bearerToken, groupJID)
	if err != nil {
		return err
	}
	if err := client.LeaveGroup(ctx, jid); err != nil {
		return fmt.Errorf("%w: %w", ErrRemoteOperation, err)
	}
	s.logOperation(instance, "leave", jid.String())
	return nil
}

func (s *GroupService) authorizedGroup(ctx context.Context, instanceName string, bearerToken string, rawGroupJID string) (dbtypes.Instance, *whatsmeow.Client, watypes.JID, error) {
	instance, client, err := s.authorizedClient(ctx, instanceName, bearerToken)
	if err != nil {
		return dbtypes.Instance{}, nil, watypes.JID{}, err
	}
	jid, err := parseGroupJID(rawGroupJID)
	if err != nil {
		return dbtypes.Instance{}, nil, watypes.JID{}, err
	}
	return instance, client, jid, nil
}

func (s *GroupService) authorizedClient(ctx context.Context, instanceName string, bearerToken string) (dbtypes.Instance, *whatsmeow.Client, error) {
	instance, err := s.authenticateInstance(ctx, instanceName, bearerToken)
	if err != nil {
		return dbtypes.Instance{}, nil, err
	}
	managed, err := s.clients.ResolveConnectedClient(ctx, instance.Name)
	if err != nil {
		return dbtypes.Instance{}, nil, err
	}
	if managed == nil || managed.Client == nil || managed.Client.Store == nil ||
		managed.Client.Store.ID == nil || !managed.Client.IsConnected() || !managed.Client.IsLoggedIn() {
		return dbtypes.Instance{}, nil, ErrInstanceDisconnected
	}
	return instance, managed.Client, nil
}

func (s *GroupService) authenticateInstance(ctx context.Context, instanceName string, bearerToken string) (dbtypes.Instance, error) {
	name := strings.TrimSpace(instanceName)
	token := strings.TrimSpace(bearerToken)
	if name == "" || token == "" {
		return dbtypes.Instance{}, repository.ErrInvalidInput
	}
	instance, err := s.instances.FindByName(ctx, name)
	if err != nil {
		return dbtypes.Instance{}, err
	}
	if instance.Auth == nil || subtle.ConstantTimeCompare([]byte(instance.Auth.Token), []byte(token)) != 1 {
		return dbtypes.Instance{}, whatsapp.ErrInvalidInstanceToken
	}
	if instance.Instance.Status != dbtypes.InstanceStatusOnline {
		return dbtypes.Instance{}, whatsapp.ErrInstanceInactive
	}
	return instance.Instance, nil
}

func (s *GroupService) downloadImage(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("%w: image must be a valid URL", ErrInvalidRequest)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidRequest, err)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d", ErrDownloadFailed, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGroupPictureBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDownloadFailed, err)
	}
	if len(data) > maxGroupPictureBytes {
		return nil, ErrImageTooLarge
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: image is empty", ErrInvalidRequest)
	}
	return data, nil
}

func (s *GroupService) logOperation(instance dbtypes.Instance, operation string, groupJID string) {
	s.logger.Info().
		Int32("instanceId", instance.ID).
		Str("instanceName", instance.Name).
		Str("operation", operation).
		Str("groupJid", groupJID).
		Msg("WhatsApp group operation completed")
}

func validateCreate(input CreateRequest) error {
	if strings.TrimSpace(input.Subject) == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidRequest)
	}
	if len(input.Participants) == 0 {
		return fmt.Errorf("%w: participants is required", ErrInvalidRequest)
	}
	for _, participant := range input.Participants {
		if strings.TrimSpace(participant) == "" {
			return fmt.Errorf("%w: participant is required", ErrInvalidParticipant)
		}
	}
	return nil
}

func validateUpdatePicture(input UpdatePictureRequest) error {
	if strings.TrimSpace(input.Image) == "" {
		return fmt.Errorf("%w: image is required", ErrInvalidRequest)
	}
	if _, err := parseGroupJID(input.GroupJID); err != nil {
		return err
	}
	return nil
}

func validateUpdateParticipant(input UpdateParticipantRequest) error {
	switch strings.TrimSpace(input.Action) {
	case string(whatsmeow.ParticipantChangeAdd), string(whatsmeow.ParticipantChangeRemove), string(whatsmeow.ParticipantChangePromote), string(whatsmeow.ParticipantChangeDemote):
	default:
		return fmt.Errorf("%w: action is invalid", ErrInvalidRequest)
	}
	if len(input.Participants) == 0 {
		return fmt.Errorf("%w: participants is required", ErrInvalidRequest)
	}
	for _, participant := range input.Participants {
		if strings.TrimSpace(participant) == "" {
			return fmt.Errorf("%w: participant is required", ErrInvalidParticipant)
		}
	}
	return nil
}

func parseGroupJID(raw string) (watypes.JID, error) {
	jid, err := watypes.ParseJID(strings.TrimSpace(raw))
	if err != nil || jid.Server != watypes.GroupServer || jid.User == "" {
		return watypes.JID{}, ErrInvalidGroupJID
	}
	return jid.ToNonAD(), nil
}

func parseParticipants(values []string) ([]watypes.JID, error) {
	participants := make([]watypes.JID, 0, len(values))
	for _, value := range values {
		jid, err := parseParticipant(value)
		if err != nil {
			return nil, err
		}
		participants = append(participants, jid)
	}
	return participants, nil
}

func parseParticipant(raw string) (watypes.JID, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return watypes.JID{}, ErrInvalidParticipant
	}
	if strings.Contains(value, "@") {
		jid, err := watypes.ParseJID(value)
		if err != nil || jid.User == "" || jid.Server == watypes.GroupServer {
			return watypes.JID{}, ErrInvalidParticipant
		}
		return jid.ToNonAD(), nil
	}
	digits := strings.NewReplacer("+", "", " ", "", "-", "", "(", "", ")", "", ".", "").Replace(value)
	if digits == "" {
		return watypes.JID{}, ErrInvalidParticipant
	}
	for _, char := range digits {
		if char < '0' || char > '9' {
			return watypes.JID{}, ErrInvalidParticipant
		}
	}
	return watypes.NewJID(digits, watypes.DefaultUserServer), nil
}

func groupInfoResponse(info *watypes.GroupInfo) InfoResponse {
	if info == nil {
		return InfoResponse{}
	}
	participants := make([]ParticipantResponse, 0, len(info.Participants))
	for _, participant := range info.Participants {
		participants = append(participants, participantResponse(participant))
	}
	size := info.ParticipantCount
	if size == 0 {
		size = len(participants)
	}
	return InfoResponse{
		ID:                  jidString(info.JID),
		Subject:             info.Name,
		SubjectOwner:        jidString(info.NameSetBy),
		SubjectTime:         unix(info.NameSetAt),
		Size:                size,
		Creation:            unix(info.GroupCreated),
		Owner:               firstJIDString(info.OwnerJID, info.OwnerPN),
		Desc:                info.Topic,
		DescID:              info.TopicID,
		Restrict:            info.IsLocked,
		Announce:            info.IsAnnounce,
		IsCommunity:         info.IsParent,
		IsCommunityAnnounce: info.IsParent && info.IsAnnounce,
		JoinApprovalMode:    info.IsJoinApprovalRequired,
		MemberAddMode:       info.MemberAddMode == watypes.GroupMemberAddModeAdmin,
		Participants:        participants,
	}
}

func participantResponse(participant watypes.GroupParticipant) ParticipantResponse {
	return ParticipantResponse{
		ID:           firstJIDString(participant.JID, participant.PhoneNumber, participant.LID),
		PhoneNumber:  jidString(participant.PhoneNumber),
		LID:          jidString(participant.LID),
		IsAdmin:      participant.IsAdmin,
		IsSuperAdmin: participant.IsSuperAdmin,
		DisplayName:  participant.DisplayName,
		Error:        participant.Error,
	}
}

func firstJIDString(jids ...watypes.JID) string {
	for _, jid := range jids {
		if value := jidString(jid); value != "" {
			return value
		}
	}
	return ""
}

func jidString(jid watypes.JID) string {
	if jid.IsEmpty() {
		return ""
	}
	return jid.ToNonAD().String()
}

func unix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}
