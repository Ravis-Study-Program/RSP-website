// Package jobs contains the public board and private application records.
package jobs

import (
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type Company struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type Position struct {
	ID             string   `json:"id"`
	Company        Company  `json:"company"`
	Title          string   `json:"title"`
	JobType        string   `json:"jobType"`
	Eligibility    string   `json:"eligibility"`
	ApplicationURL string   `json:"applicationUrl"`
	Locations      []string `json:"locations"`
	Notes          string   `json:"notes"`
	Status         string   `json:"status"`
}

type PositionInput struct {
	CompanyName    string   `json:"companyName"`
	Category       string   `json:"category"`
	Title          string   `json:"title"`
	JobType        string   `json:"jobType"`
	Eligibility    string   `json:"eligibility"`
	ApplicationURL string   `json:"applicationUrl"`
	Locations      []string `json:"locations"`
	Notes          string   `json:"notes"`
	Status         string   `json:"status"`
}

type Stage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date"`
	Notes   string `json:"notes"`
	Initial bool   `json:"initial"`
}

type StageInput struct {
	Title string `json:"title"`
	Date  string `json:"date"`
	Notes string `json:"notes"`
}

type Application struct {
	ID       string   `json:"id"`
	Status   string   `json:"status"`
	Position Position `json:"position"`
	Stages   []Stage  `json:"stages"`
}

func ValidApplicationStatus(v string) bool {
	return slices.Contains([]string{"active", "inactive", "offer"}, v)
}

func ValidCategory(v string) bool {
	return slices.Contains([]string{"faang", "big_tech", "quant", "other"}, v)
}

func (v *PositionInput) Validate() (string, error) {
	v.CompanyName = strings.TrimSpace(v.CompanyName)
	v.Title = strings.TrimSpace(v.Title)
	v.ApplicationURL = strings.TrimSpace(v.ApplicationURL)
	v.Eligibility = strings.TrimSpace(v.Eligibility)
	v.Notes = strings.TrimSpace(v.Notes)
	v.Locations = UniqueLocations(v.Locations)
	if v.Status == "" {
		v.Status = "open"
	}
	if !textLength(v.CompanyName, 1, 120) || !textLength(v.Title, 1, 300) {
		return "", errors.New("enter a company name and job title (up to 120 and 300 characters)")
	}
	if !ValidCategory(v.Category) || !slices.Contains([]string{"intern", "grad", "experienced"}, v.JobType) || !slices.Contains([]string{"open", "closed"}, v.Status) {
		return "", errors.New("choose a valid company category, job type and listing status")
	}
	if !textLength(v.Eligibility, 0, 4000) || !textLength(v.Notes, 0, 10000) || len(v.Locations) < 1 || len(v.Locations) > 20 {
		return "", errors.New("enter 1–20 locations; eligibility and notes must fit within 4,000 and 10,000 characters")
	}
	for _, location := range v.Locations {
		if !textLength(location, 1, 200) {
			return "", errors.New("each location must be at most 200 characters")
		}
	}
	return NormalizeURL(v.ApplicationURL)
}

func (v *StageInput) Validate() error {
	v.Title, v.Notes = strings.TrimSpace(v.Title), strings.TrimSpace(v.Notes)
	if !textLength(v.Title, 1, 120) || !textLength(v.Notes, 0, 10000) {
		return errors.New("enter a stage title up to 120 characters and notes up to 10,000 characters")
	}
	if _, err := time.Parse("2006-01-02", v.Date); err != nil {
		return errors.New("enter a valid stage date")
	}
	return nil
}

func textLength(v string, min, max int) bool {
	n := utf8.RuneCountInString(v)
	return n >= min && n <= max
}

func UniqueLocations(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value != "" && !seen[key] {
			seen[key] = true
			out = append(out, value)
		}
	}
	return out
}

// NormalizeURL only removes known tracking metadata; gh_jid, pid and other
// job identifiers are significant. Links are never fetched by the server.
func NormalizeURL(raw string) (string, error) {
	invalid := errors.New("enter a public HTTPS application link, up to 2,048 characters")
	if len(raw) > 2048 {
		return "", invalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return "", invalid
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", invalid
	}
	if ip := net.ParseIP(host); ip != nil {
		return "", invalid
	}
	u.Host, u.Fragment = host, ""
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", invalid
	}
	for key := range q {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") || slices.Contains([]string{"gh_src", "gclid", "fbclid"}, lower) {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
