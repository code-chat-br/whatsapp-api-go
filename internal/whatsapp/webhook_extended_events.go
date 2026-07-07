package whatsapp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	watypes "go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	webhooksvc "whatsapp-go-api/internal/webhook"
)

type CallEventNormalizer interface {
	Normalize(event any) (webhooksvc.CallUpsertWebhookData, error)
}

type WhatsmeowCallEventNormalizer struct{}

func NewCallEventNormalizer() WhatsmeowCallEventNormalizer {
	return WhatsmeowCallEventNormalizer{}
}

func (WhatsmeowCallEventNormalizer) Normalize(event any) (webhooksvc.CallUpsertWebhookData, error) {
	switch value := event.(type) {
	case *events.CallOffer:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusOffer, value.Data)
	case *events.CallAccept:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusAccept, value.Data)
	case *events.CallOfferNotice:
		data, err := normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusRinging, value.Data)
		if err != nil {
			return data, err
		}
		if value.Media != "" {
			isVideo := value.Media == "video"
			data.IsVideo = &isVideo
		}
		if value.Type == "group" {
			isGroup := true
			data.IsGroup = &isGroup
		}
		return data, nil
	case *events.CallPreAccept:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusPreAccept, value.Data)
	case *events.CallTransport:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusTransport, value.Data)
	case *events.CallTerminate:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusTerminate, value.Data)
	case *events.CallReject:
		return normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusReject, value.Data)
	case *events.CallRelayLatency:
		data, err := normalizeCallMeta(value.BasicCallMeta, value.Data, webhooksvc.WebhookCallStatusRelayLatency, value.Data)
		if err != nil {
			return data, err
		}
		data.Latency = latencyFromNode(value.Data)
		return data, nil
	case *events.UnknownCallEvent:
		return normalizeUnknownCall(value.Node), nil
	default:
		return webhooksvc.CallUpsertWebhookData{}, fmt.Errorf("unsupported call event %T", event)
	}
}

func normalizeCallMeta(meta watypes.BasicCallMeta, dataNode *waBinary.Node, status webhooksvc.WebhookCallStatus, attrsNode *waBinary.Node) (webhooksvc.CallUpsertWebhookData, error) {
	from := firstJID(meta.CallCreator, meta.From)
	groupJID := meta.GroupJID
	isGroup := !groupJID.IsEmpty() && groupJID.Server == watypes.GroupServer
	chatID := jidString(firstJID(groupJID, meta.From, meta.CallCreator))
	if chatID == "" {
		chatID = nodeAttrString(attrsNode, "from")
	}
	isVideo := isVideoCall(dataNode)
	date := eventDateTime(meta.Timestamp, time.Now().UTC())
	return webhooksvc.CallUpsertWebhookData{
		ChatID:   chatID,
		From:     jidString(from),
		CallerPN: pnUserPtr(firstJID(meta.CallCreatorAlt, meta.CallCreator, meta.From)),
		IsGroup:  &isGroup,
		GroupJID: stringPtrFromJID(groupJID),
		ID:       firstNonEmpty(meta.CallID, nodeAttrString(attrsNode, "id"), nodeAttrString(attrsNode, "call-id")),
		Date:     date,
		IsVideo:  isVideo,
		Status:   status,
		Offline:  boolFromNode(attrsNode, "offline"),
		Latency:  nil,
	}, nil
}

func normalizeUnknownCall(node *waBinary.Node) webhooksvc.CallUpsertWebhookData {
	date := time.Now().UTC()
	if ts := nodeAttrInt64(node, "t"); ts != nil && *ts > 0 {
		date = time.Unix(*ts, 0).UTC()
	}
	from := nodeAttrString(node, "from")
	return webhooksvc.CallUpsertWebhookData{
		ChatID:  from,
		From:    from,
		ID:      firstNonEmpty(nodeAttrString(node, "id"), nodeAttrString(node, "call-id")),
		Date:    date,
		Status:  webhooksvc.WebhookCallStatusUnknown,
		Offline: boolFromNode(node, "offline"),
		Latency: latencyFromNode(node),
	}
}

type ContactUpdateNormalizer interface {
	Normalize(event any, persistedContact webhooksvc.ContactUpdateWebhookData) ([]webhooksvc.ContactUpdateWebhookData, error)
}

type WhatsmeowContactUpdateNormalizer struct{}

func NewContactUpdateNormalizer() WhatsmeowContactUpdateNormalizer {
	return WhatsmeowContactUpdateNormalizer{}
}

func (WhatsmeowContactUpdateNormalizer) Normalize(event any, persistedContact webhooksvc.ContactUpdateWebhookData) ([]webhooksvc.ContactUpdateWebhookData, error) {
	persistedContact.Action = "updated"
	switch value := event.(type) {
	case *events.PushName:
		persistedContact.Source = "pushName"
		if strings.TrimSpace(value.NewPushName) != "" {
			persistedContact.PushName = stringPtr(value.NewPushName)
		}
		persistedContact.LID = stringPtrFromJID(value.JIDAlt)
	case *events.BusinessName:
		persistedContact.Source = "businessName"
		if strings.TrimSpace(value.NewBusinessName) != "" {
			persistedContact.BusinessName = stringPtr(value.NewBusinessName)
		}
	default:
		return nil, fmt.Errorf("unsupported contact update event %T", event)
	}
	return []webhooksvc.ContactUpdateWebhookData{persistedContact}, nil
}

type GroupEventNormalizer interface {
	NormalizeUpdate(event *events.GroupInfo) ([]webhooksvc.GroupUpdateWebhookData, error)
	NormalizeUpsert(event *events.JoinedGroup) ([]webhooksvc.GroupUpsertWebhookData, error)
	NormalizeParticipantUpdates(event *events.GroupInfo) ([]webhooksvc.GroupParticipantsUpdatedWebhookData, error)
}

type WhatsmeowGroupEventNormalizer struct{}

func NewGroupEventNormalizer() WhatsmeowGroupEventNormalizer {
	return WhatsmeowGroupEventNormalizer{}
}

func (WhatsmeowGroupEventNormalizer) NormalizeUpdate(event *events.GroupInfo) ([]webhooksvc.GroupUpdateWebhookData, error) {
	if event == nil || !hasGroupMetadataUpdate(event) {
		return nil, nil
	}
	return []webhooksvc.GroupUpdateWebhookData{{Partial: groupPartialFromEvent(event)}}, nil
}

func (WhatsmeowGroupEventNormalizer) NormalizeUpsert(event *events.JoinedGroup) ([]webhooksvc.GroupUpsertWebhookData, error) {
	if event == nil {
		return nil, nil
	}
	group := event.GroupInfo
	output := webhooksvc.GroupUpsertWebhookData{
		ID:                   jidString(group.JID),
		Notify:               stringPtr(event.Notify),
		AddressingMode:       stringPtr(string(group.AddressingMode)),
		Owner:                stringPtrFromJID(group.OwnerJID),
		OwnerPN:              pnUserPtr(group.OwnerPN),
		OwnerUsername:        stringPtr(group.OwnerJID.User),
		OwnerCountryCode:     stringPtr(group.CreatorCountryCode),
		Subject:              group.Name,
		SubjectOwner:         stringPtrFromJID(group.NameSetBy),
		SubjectOwnerPN:       pnUserPtr(group.NameSetByPN),
		SubjectOwnerUsername: stringPtr(group.NameSetBy.User),
		SubjectTime:          unixPtr(group.NameSetAt),
		Creation:             unixPtr(group.GroupCreated),
		Description:          stringPtr(group.Topic),
		DescriptionOwner:     stringPtrFromJID(group.TopicSetBy),
		DescriptionOwnerPN:   pnUserPtr(group.TopicSetByPN),
		DescriptionOwnerUser: stringPtr(group.TopicSetBy.User),
		DescriptionID:        stringPtr(group.TopicID),
		DescriptionTime:      unixPtr(group.TopicSetAt),
		LinkedParent:         stringPtrFromJID(group.LinkedParentJID),
		Restrict:             boolPtr(group.IsLocked),
		Announce:             boolPtr(group.IsAnnounce),
		MemberAddMode:        groupMemberAddModeBool(group.MemberAddMode),
		JoinApprovalMode:     boolPtr(group.IsJoinApprovalRequired),
		IsCommunity:          boolPtr(group.IsParent),
		IsCommunityAnnounce:  boolPtr(group.IsDefaultSubGroup),
		Size:                 intPtr(group.ParticipantCount),
		Participants:         groupParticipantsFromGroup(group.Participants),
		EphemeralDuration:    int64Ptr(int64(group.DisappearingTimer)),
		Author:               stringPtrFromJIDPtr(event.Sender),
		AuthorPN:             pnUserPtrFromPtr(event.SenderPN),
	}
	if output.Participants == nil {
		output.Participants = []webhooksvc.GroupParticipantWebhookData{}
	}
	return []webhooksvc.GroupUpsertWebhookData{output}, nil
}

func (WhatsmeowGroupEventNormalizer) NormalizeParticipantUpdates(event *events.GroupInfo) ([]webhooksvc.GroupParticipantsUpdatedWebhookData, error) {
	if event == nil {
		return nil, nil
	}
	output := make([]webhooksvc.GroupParticipantsUpdatedWebhookData, 0, 4)
	appendParticipants := func(action webhooksvc.GroupParticipantAction, participants []watypes.JID) {
		if len(participants) == 0 {
			return
		}
		output = append(output, webhooksvc.GroupParticipantsUpdatedWebhookData{
			ID:           jidString(event.JID),
			Author:       jidStringPtrValue(event.Sender),
			AuthorPN:     pnUserPtrFromPtr(event.SenderPN),
			Participants: groupParticipantsFromJIDs(participants),
			Action:       action,
		})
	}
	appendParticipants(webhooksvc.GroupParticipantActionAdd, event.Join)
	appendParticipants(webhooksvc.GroupParticipantActionRemove, event.Leave)
	appendParticipants(webhooksvc.GroupParticipantActionPromote, event.Promote)
	appendParticipants(webhooksvc.GroupParticipantActionDemote, event.Demote)
	return output, nil
}

type NewsletterEventNormalizer interface {
	Normalize(event any) (map[string]any, error)
}

type WhatsmeowNewsletterEventNormalizer struct{}

func NewNewsletterEventNormalizer() WhatsmeowNewsletterEventNormalizer {
	return WhatsmeowNewsletterEventNormalizer{}
}

func (WhatsmeowNewsletterEventNormalizer) Normalize(event any) (map[string]any, error) {
	var eventType string
	switch event.(type) {
	case *events.NewsletterJoin:
		eventType = "join"
	case *events.NewsletterLeave:
		eventType = "leave"
	case *events.NewsletterLiveUpdate:
		eventType = "live.update"
	case *events.NewsletterMessageMeta:
		eventType = "message.meta"
	case *events.NewsletterMuteChange:
		eventType = "mute.change"
	default:
		return nil, fmt.Errorf("unsupported newsletter event %T", event)
	}
	source, err := webhooksvc.NewEventNormalizer().ToJSONMap(event)
	if err != nil {
		return nil, err
	}
	renameMapKey(source, "id", "newsletterJid")
	renameMapKey(source, "jid", "newsletterJid")
	return webhooksvc.MergeEventData(eventType, source, time.Now().UTC()), nil
}

type LabelEventNormalizer interface {
	NormalizeAssociation(event any) (map[string]any, error)
	NormalizeEdit(event any) (map[string]any, error)
}

type WhatsmeowLabelEventNormalizer struct{}

func NewLabelEventNormalizer() WhatsmeowLabelEventNormalizer {
	return WhatsmeowLabelEventNormalizer{}
}

func (WhatsmeowLabelEventNormalizer) NormalizeAssociation(event any) (map[string]any, error) {
	var eventType string
	switch event.(type) {
	case *events.LabelAssociationChat:
		eventType = "chat"
	case *events.LabelAssociationMessage:
		eventType = "message"
	default:
		return nil, fmt.Errorf("unsupported label association event %T", event)
	}
	source, err := webhooksvc.NewEventNormalizer().ToJSONMap(event)
	if err != nil {
		return nil, err
	}
	normalizeChatEventKeys(source)
	flattenLabelAssociationAction(source)
	delete(source, "timestamp")
	return webhooksvc.MergeEventData(eventType, source, time.Now().UTC()), nil
}

func (WhatsmeowLabelEventNormalizer) NormalizeEdit(event any) (map[string]any, error) {
	source, err := webhooksvc.NewEventNormalizer().ToJSONMap(event)
	if err != nil {
		return nil, err
	}
	flattenAction(source)
	renameMapKey(source, "labelId", "id")
	delete(source, "timestamp")
	return source, nil
}

func flattenLabelAssociationAction(source map[string]any) {
	action, ok := source["action"].(map[string]any)
	if !ok {
		return
	}
	if labeled, ok := action["labeled"].(bool); ok {
		if labeled {
			source["action"] = "add"
		} else {
			source["action"] = "remove"
		}
	} else {
		delete(source, "action")
	}
	for key, value := range action {
		if key == "labeled" {
			continue
		}
		source[key] = value
	}
}

func hasGroupMetadataUpdate(event *events.GroupInfo) bool {
	return event.Name != nil ||
		event.Topic != nil ||
		event.Locked != nil ||
		event.Announce != nil ||
		event.Ephemeral != nil ||
		event.MembershipApprovalMode != nil ||
		event.Delete != nil ||
		event.Link != nil ||
		event.Unlink != nil ||
		event.NewInviteLink != nil ||
		event.Notify != "" ||
		event.Suspended ||
		event.Unsuspended ||
		len(event.UnknownChanges) > 0
}

func groupPartialFromEvent(event *events.GroupInfo) webhooksvc.GroupPartialWebhookData {
	partial := webhooksvc.GroupPartialWebhookData{
		ID:             jidString(event.JID),
		Notify:         stringPtr(event.Notify),
		InviteCode:     event.NewInviteLink,
		Author:         stringPtrFromJIDPtr(event.Sender),
		AuthorPN:       pnUserPtrFromPtr(event.SenderPN),
		AuthorUsername: jidUserPtrFromPtr(event.Sender),
		IsCommunity:    normalizeGroupIsCommunity(event),
	}
	if event.Name != nil {
		partial.Subject = stringPtr(event.Name.Name)
		partial.SubjectOwner = stringPtrFromJID(event.Name.NameSetBy)
		partial.SubjectOwnerPN = pnUserPtr(event.Name.NameSetByPN)
		partial.SubjectOwnerUsername = stringPtr(event.Name.NameSetBy.User)
		partial.SubjectTime = unixPtr(event.Name.NameSetAt)
	}
	if event.Topic != nil {
		partial.Description = stringPtr(event.Topic.Topic)
		partial.DescriptionOwner = stringPtrFromJID(event.Topic.TopicSetBy)
		partial.DescriptionOwnerPN = pnUserPtr(event.Topic.TopicSetByPN)
		partial.DescriptionOwnerUser = stringPtr(event.Topic.TopicSetBy.User)
		partial.DescriptionID = stringPtr(event.Topic.TopicID)
		partial.DescriptionTime = unixPtr(event.Topic.TopicSetAt)
	}
	if event.Locked != nil {
		partial.Restrict = boolPtr(event.Locked.IsLocked)
	}
	if event.Announce != nil {
		partial.Announce = boolPtr(event.Announce.IsAnnounce)
	}
	if event.MembershipApprovalMode != nil {
		partial.JoinApprovalMode = boolPtr(event.MembershipApprovalMode.IsJoinApprovalRequired)
	}
	if event.Ephemeral != nil {
		partial.EphemeralDuration = int64Ptr(int64(event.Ephemeral.DisappearingTimer))
	}
	if event.Link != nil {
		partial.LinkedParent = stringPtrFromJID(event.Link.Group.JID)
	}
	if event.Unlink != nil {
		partial.LinkedParent = stringPtrFromJID(event.Unlink.Group.JID)
	}
	return partial
}

func normalizeGroupIsCommunity(info *events.GroupInfo) *bool {
	if info == nil {
		return nil
	}
	if info.Link != nil {
		value := true
		return &value
	}
	if info.Unlink != nil {
		value := false
		return &value
	}
	return nil
}

func groupParticipantsFromGroup(participants []watypes.GroupParticipant) []webhooksvc.GroupParticipantWebhookData {
	output := make([]webhooksvc.GroupParticipantWebhookData, 0, len(participants))
	for _, participant := range participants {
		output = append(output, groupParticipantFromGroup(participant))
	}
	return output
}

func groupParticipantFromGroup(participant watypes.GroupParticipant) webhooksvc.GroupParticipantWebhookData {
	admin := groupParticipantAdmin(participant.IsAdmin, participant.IsSuperAdmin)
	return webhooksvc.GroupParticipantWebhookData{
		ID:           stringPtrFromJID(firstTraditionalJID(participant.PhoneNumber, participant.JID)),
		LID:          stringPtrFromJID(firstLIDJID(participant.LID, participant.JID)),
		IsAdmin:      participant.IsAdmin,
		IsSuperAdmin: participant.IsSuperAdmin,
		Admin:        admin,
	}
}

func groupParticipantsFromJIDs(participants []watypes.JID) []webhooksvc.GroupParticipantWebhookData {
	output := make([]webhooksvc.GroupParticipantWebhookData, 0, len(participants))
	for _, participant := range participants {
		output = append(output, webhooksvc.GroupParticipantWebhookData{
			ID:  stringPtrFromJID(firstTraditionalJID(participant)),
			LID: stringPtrFromJID(firstLIDJID(participant)),
		})
	}
	return output
}

func groupParticipantAdmin(isAdmin bool, isSuperAdmin bool) *string {
	switch {
	case isSuperAdmin:
		value := string(webhooksvc.GroupParticipantAdminSuperAdmin)
		return &value
	case isAdmin:
		value := string(webhooksvc.GroupParticipantAdminAdmin)
		return &value
	default:
		return nil
	}
}

func groupMemberAddModeBool(mode watypes.GroupMemberAddMode) *bool {
	switch mode {
	case watypes.GroupMemberAddModeAllMember:
		return boolPtr(true)
	case watypes.GroupMemberAddModeAdmin:
		return boolPtr(false)
	default:
		return nil
	}
}

func isVideoCall(node *waBinary.Node) *bool {
	for _, key := range []string{"media", "type"} {
		value := strings.ToLower(nodeAttrString(node, key))
		if value == "video" {
			return boolPtr(true)
		}
		if value == "audio" {
			return boolPtr(false)
		}
	}
	return nil
}

func latencyFromNode(node *waBinary.Node) *int64 {
	for _, key := range []string{"latencyMs", "latency_ms", "latency"} {
		if value := nodeAttrInt64(node, key); value != nil {
			return value
		}
	}
	return nil
}

func boolFromNode(node *waBinary.Node, key string) bool {
	if node == nil || node.Attrs == nil {
		return false
	}
	switch value := node.Attrs[key].(type) {
	case bool:
		return value
	case string:
		return value == "true" || value == "1"
	default:
		return false
	}
}

func nodeAttrString(node *waBinary.Node, key string) string {
	if node == nil || node.Attrs == nil {
		return ""
	}
	raw, ok := node.Attrs[key]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	case fmt.Stringer:
		return value.String()
	case watypes.JID:
		return value.String()
	default:
		return fmt.Sprint(value)
	}
}

func nodeAttrInt64(node *waBinary.Node, key string) *int64 {
	if node == nil || node.Attrs == nil {
		return nil
	}
	switch value := node.Attrs[key].(type) {
	case int:
		return int64Ptr(int64(value))
	case int64:
		return int64Ptr(value)
	case uint64:
		return int64Ptr(int64(value))
	case float64:
		return int64Ptr(int64(value))
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			return &parsed
		}
	}
	return nil
}

func firstJID(values ...watypes.JID) watypes.JID {
	for _, value := range values {
		if !value.IsEmpty() {
			return value
		}
	}
	return watypes.EmptyJID
}

func pnUserPtr(jid watypes.JID) *string {
	if jid.IsEmpty() {
		return nil
	}
	if jid.Server == watypes.DefaultUserServer || jid.Server == watypes.LegacyUserServer {
		return stringPtr(jid.User)
	}
	return nil
}

func pnUserPtrFromPtr(jid *watypes.JID) *string {
	if jid == nil {
		return nil
	}
	return pnUserPtr(*jid)
}

func jidUserPtrFromPtr(jid *watypes.JID) *string {
	if jid == nil || jid.IsEmpty() {
		return nil
	}
	return stringPtr(jid.User)
}

func stringPtrFromJIDPtr(jid *watypes.JID) *string {
	if jid == nil {
		return nil
	}
	return stringPtrFromJID(*jid)
}

func jidStringPtrValue(jid *watypes.JID) string {
	if jid == nil {
		return ""
	}
	return jidString(*jid)
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	if value == 0 {
		return nil
	}
	return &value
}

func int64Ptr(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}

func unixPtr(value time.Time) *int64 {
	if value.IsZero() {
		return nil
	}
	unix := value.UTC().Unix()
	return &unix
}
