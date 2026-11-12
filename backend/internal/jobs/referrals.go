package jobs

import (
	"errors"
	"strings"
)

type Referrer struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type Referral struct {
	ID       string   `json:"id"`
	Company  Company  `json:"company"`
	Referrer Referrer `json:"referrer"`
	Notes    string   `json:"notes"`
}

type ReferralInput struct {
	CompanyName string `json:"companyName"`
	Notes       string `json:"notes"`
}

func (v *ReferralInput) Validate() error {
	v.CompanyName, v.Notes = strings.TrimSpace(v.CompanyName), strings.TrimSpace(v.Notes)
	if !textLength(v.CompanyName, 1, 120) || !textLength(v.Notes, 0, 10000) {
		return errors.New("enter a company name up to 120 characters and notes up to 10,000 characters")
	}
	return nil
}
