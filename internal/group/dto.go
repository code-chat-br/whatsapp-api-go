package group

type CreateRequest struct {
	Subject      string   `json:"subject"`
	Description  *string  `json:"description,omitempty"`
	Participants []string `json:"participants"`
}

type UpdatePictureRequest struct {
	Image    string `json:"image"`
	GroupJID string `json:"groupJid"`
}

type UpdateParticipantRequest struct {
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

type InviteCodeResponse struct {
	Invitation string `json:"invitation"`
}

type InfoResponse struct {
	ID                  string                `json:"id"`
	Subject             string                `json:"subject"`
	SubjectOwner        string                `json:"subjectOwner,omitempty"`
	SubjectTime         int64                 `json:"subjectTime,omitempty"`
	Size                int                   `json:"size"`
	Creation            int64                 `json:"creation,omitempty"`
	Owner               string                `json:"owner,omitempty"`
	Desc                string                `json:"desc,omitempty"`
	DescID              string                `json:"descId,omitempty"`
	Restrict            bool                  `json:"restrict"`
	Announce            bool                  `json:"announce"`
	IsCommunity         bool                  `json:"isCommunity"`
	IsCommunityAnnounce bool                  `json:"isCommunityAnnounce"`
	JoinApprovalMode    bool                  `json:"joinApprovalMode"`
	MemberAddMode       bool                  `json:"memberAddMode"`
	Participants        []ParticipantResponse `json:"participants"`
}

type ParticipantResponse struct {
	ID           string `json:"id"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	LID          string `json:"lid,omitempty"`
	IsAdmin      bool   `json:"isAdmin"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
	DisplayName  string `json:"displayName,omitempty"`
	Error        int    `json:"error,omitempty"`
}
