package jobs

import (
	"errors"
	"regexp"
	"strings"
)

const MaxReferralRequests = 5

var driveFileURL = regexp.MustCompile(`^https://drive\.google\.com/(file/d/[A-Za-z0-9_-]+(/(view|preview))?/?(\?[^#[:space:][:cntrl:]]*)?|open\?id=[A-Za-z0-9_-]+(&[^#[:space:][:cntrl:]]*)?)$`)

var (
	ErrReferralProfileRequired = errors.New("complete your referral profile before requesting a referral")
	ErrReferralLimit           = errors.New("you can request referrals for up to five open positions")
	ErrReferralPositionClosed  = errors.New("this position is no longer open for referrals")
)

type ReferralProfile struct {
	Reason    string `json:"reason"`
	ResumeURL string `json:"resumeUrl"`
}

type ReferralBoard struct {
	Profile   ReferralProfile `json:"profile"`
	Requests  []string        `json:"requests"`
	Companies []Company       `json:"companies"`
}

type ReferralCount struct {
	CompanyID string `json:"companyId"`
	Count     int    `json:"count"`
}

type ReferralCandidate struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Reason    string             `json:"reason"`
	ResumeURL string             `json:"resumeUrl"`
	Positions []ReferralPosition `json:"positions"`
}

type ReferralPosition struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (v *ReferralProfile) Validate() error {
	v.Reason = strings.TrimSpace(v.Reason)
	v.ResumeURL = strings.TrimSpace(v.ResumeURL)
	if !textLength(v.Reason, 1, 4000) {
		return errors.New("enter why you would be a good referral, up to 4,000 characters")
	}
	if len(v.ResumeURL) > 2048 || !driveFileURL.MatchString(v.ResumeURL) {
		return errors.New("enter a Google Drive link to your résumé file")
	}
	return nil
}
