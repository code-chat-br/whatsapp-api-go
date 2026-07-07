package request

type MessageOptions struct {
	Delay              *int64         `json:"delay"`
	Presence           *string        `json:"presence"`
	QuotedMessageID    *int64         `json:"quotedMessageId"`
	QuotedMessage      map[string]any `json:"quotedMessage"`
	ExternalAttributes map[string]any `json:"externalAttributes"`
	MentionAll         *bool          `json:"mentionAll"`
}

type SendTextRequest struct {
	Number      *string         `json:"number"`
	Chat        *string         `json:"chat"`
	Recipient   *string         `json:"recipient"`
	Options     *MessageOptions `json:"options"`
	TextMessage *TextMessage    `json:"textMessage"`
}

type TextMessage struct {
	Text string `json:"text"`
}

type SendLinkRequest struct {
	Number      *string         `json:"number"`
	Chat        *string         `json:"chat"`
	Recipient   *string         `json:"recipient"`
	Options     *MessageOptions `json:"options"`
	LinkMessage *LinkMessage    `json:"linkMessage"`
}

type LinkMessage struct {
	Link         string  `json:"link"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	Title        *string `json:"title"`
	Description  *string `json:"description"`
}

type SendMediaRequest struct {
	Number       *string         `json:"number"`
	Chat         *string         `json:"chat"`
	Recipient    *string         `json:"recipient"`
	Options      *MessageOptions `json:"options"`
	MediaMessage *MediaMessage   `json:"mediaMessage"`
}

type MediaMessage struct {
	MediaType string  `json:"mediatype"`
	FileName  *string `json:"fileName"`
	Caption   *string `json:"caption"`
	Media     string  `json:"media"`
}

type SendWhatsAppAudioRequest struct {
	Number       string               `json:"number"`
	Options      *MessageOptions      `json:"options"`
	AudioMessage *AudioMessageRequest `json:"audioMessage"`
}

type AudioMessageRequest struct {
	Audio string `json:"audio"`
}

type SendContactRequest struct {
	Number         *string          `json:"number"`
	Chat           *string          `json:"chat"`
	Recipient      *string          `json:"recipient"`
	Options        *MessageOptions  `json:"options"`
	ContactMessage []ContactMessage `json:"contactMessage"`
}

type ContactMessage struct {
	FullName     string  `json:"fullName"`
	WUID         string  `json:"wuid"`
	PhoneNumber  string  `json:"phoneNumber"`
	Organization *string `json:"organization"`
	VCard        *string `json:"vcard"`
}

type SendLocationRequest struct {
	Number          *string          `json:"number"`
	Chat            *string          `json:"chat"`
	Recipient       *string          `json:"recipient"`
	Options         *MessageOptions  `json:"options"`
	LocationMessage *LocationMessage `json:"locationMessage"`
}

type LocationMessage struct {
	Name      *string  `json:"name"`
	Address   *string  `json:"address"`
	URL       *string  `json:"url"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type SendReactionRequest struct {
	Options         *MessageOptions  `json:"options"`
	ReactionMessage *ReactionMessage `json:"reactionMessage"`
}

type ReactionMessage struct {
	Key      ReactionKey `json:"key"`
	Reaction string      `json:"reaction"`
}

type ReactionKey struct {
	RemoteJID   string  `json:"remoteJid"`
	FromMe      *bool   `json:"fromMe"`
	ID          string  `json:"id"`
	Participant *string `json:"participant"`
}
