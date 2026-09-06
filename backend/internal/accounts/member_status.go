package accounts

type MemberStatus struct {
	UserID       string `json:"userId"`
	ActiveMember bool   `json:"activeMember,omitempty"`
	Alumni       bool   `json:"alumni,omitempty"`
	FormerMember bool   `json:"formerMember,omitempty"`
	Inactive     bool   `json:"-"`
	Suspended    bool   `json:"suspended,omitempty"`
	Deleted      bool   `json:"deleted,omitempty"`
	KickedOnly   bool   `json:"kickedOnly,omitempty"`
}

func (p MemberStatus) CanBeMockInterviewParticipant() bool {
	return (p.ActiveMember || p.Alumni) && !p.Inactive && !p.Suspended && !p.Deleted && !p.KickedOnly
}

func (p MemberStatus) CanAccessProgramme() bool {
	return (p.ActiveMember || p.Alumni || p.FormerMember) && !p.Inactive && !p.Suspended && !p.Deleted && !p.KickedOnly
}

func (p MemberStatus) IsVisibleInDirectory() bool {
	return (p.ActiveMember || p.Alumni) && !p.Inactive && !p.Suspended && !p.Deleted && !p.KickedOnly
}
