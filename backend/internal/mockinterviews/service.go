// Package mockinterviews models and manages mock interviews.
package mockinterviews

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrInvalid   = errors.New("invalid mock interview")
)

type RoundType string

const (
	Behavioural RoundType = "behavioural"
	LeetCode    RoundType = "leetcode"
	Custom      RoundType = "custom"
)

type ParticipantSummary struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatarUrl,omitempty"`
}

type Scores struct {
	Behavioural        *int `json:"behavioural,omitempty"`
	ConfirmQuestions   *int `json:"confirmQuestions,omitempty"`
	AlgorithmDesign    *int `json:"algorithmDesign,omitempty"`
	ComplexityAnalysis *int `json:"complexityAnalysis,omitempty"`
	Coding             *int `json:"coding,omitempty"`
	Testing            *int `json:"testing,omitempty"`
	Custom             *int `json:"custom,omitempty"`
}

type Round struct {
	ID                 string    `json:"id"`
	Type               RoundType `json:"type"`
	ProblemID          string    `json:"problemId,omitempty"`
	Content            string    `json:"content,omitempty"`
	Link               string    `json:"link,omitempty"`
	Scores             Scores    `json:"scores"`
	Reviewed           bool      `json:"reviewed"`
	IntervieweeComment string    `json:"intervieweeComment"`
}

type Interview struct {
	ID              string             `json:"id"`
	InterviewerID   string             `json:"interviewerId"`
	IntervieweeID   string             `json:"intervieweeId"`
	Interviewer     ParticipantSummary `json:"interviewer"`
	Interviewee     ParticipantSummary `json:"interviewee"`
	SeasonID        *string            `json:"seasonId,omitempty"`
	OccurredAt      time.Time          `json:"occurredAt"`
	DurationMinutes int                `json:"durationMinutes"`
	Notes           string             `json:"notes"`
	Rounds          []Round            `json:"rounds"`
	DeletedAt       *time.Time         `json:"-"`
}

// Service contains deterministic interview rules and input normalization.
type Service struct {
	Sanitize func(string) string
}

type CreateInput struct {
	Interviewee     accounts.MemberStatus
	SeasonID        *string
	OccurredAt      time.Time
	DurationMinutes int
	Notes           string
	Rounds          []Round
}

func (s *Service) Create(actorID string, in CreateInput, now time.Time) (Interview, error) {
	if actorID == "" || !in.Interviewee.CanBeMockInterviewParticipant() {
		return Interview{}, ErrForbidden
	}
	rounds := s.cleanRounds(in.Rounds)
	for i := range rounds {
		rounds[i].Reviewed = false
		rounds[i].IntervieweeComment = ""
	}
	m := Interview{
		ID:              "mock-" + actorID + "-" + now.UTC().Format("20060102150405.000000000"),
		InterviewerID:   actorID,
		IntervieweeID:   in.Interviewee.UserID,
		SeasonID:        in.SeasonID,
		OccurredAt:      in.OccurredAt.UTC(),
		DurationMinutes: in.DurationMinutes,
		Notes:           s.clean(in.Notes),
		Rounds:          rounds,
	}
	if err := validate(m); err != nil {
		return Interview{}, err
	}

	return m, nil
}

type UpdateInput struct {
	OccurredAt      time.Time
	DurationMinutes int
	Notes           string
	Rounds          []Round
}

// Update updates a value without changing the input.
func (s Service) Update(m Interview, actorID string, in UpdateInput) (Interview, error) {
	if actorID != m.InterviewerID {
		return Interview{}, ErrForbidden
	}
	candidate := m
	candidate.OccurredAt = in.OccurredAt.UTC()
	candidate.DurationMinutes = in.DurationMinutes
	candidate.Notes = s.clean(in.Notes)
	candidate.Rounds = s.cleanRounds(in.Rounds)
	existingReviews := make(map[string]Round, len(m.Rounds))
	for _, round := range m.Rounds {
		existingReviews[round.ID] = round
	}
	for i := range candidate.Rounds {
		if existing, ok := existingReviews[candidate.Rounds[i].ID]; ok {
			candidate.Rounds[i].Reviewed = existing.Reviewed
			candidate.Rounds[i].IntervieweeComment = existing.IntervieweeComment
		} else {
			candidate.Rounds[i].Reviewed = false
			candidate.Rounds[i].IntervieweeComment = ""
		}
	}
	if err := validate(candidate); err != nil {
		return Interview{}, err
	}

	return candidate, nil
}

type ReviewRoundInput struct {
	ActorID  string
	RoundID  string
	Comment  string
	Reviewed bool
}

// Review updates a review without changing the input.
func (s Service) ReviewRound(m Interview, input ReviewRoundInput) (Interview, error) {
	if input.ActorID != m.IntervieweeID {
		return Interview{}, ErrForbidden
	}
	candidate := cloneInterview(m)
	found := false
	for i := range candidate.Rounds {
		if candidate.Rounds[i].ID == input.RoundID {
			candidate.Rounds[i].IntervieweeComment = s.clean(input.Comment)
			candidate.Rounds[i].Reviewed = input.Reviewed
			found = true
			break
		}
	}
	if !found {
		return Interview{}, ErrInvalid
	}
	return candidate, nil
}

type CorrectIdentitiesInput struct {
	InterviewerID string
	IntervieweeID string
	SeasonID      *string
	Reason        string
}

// CorrectIdentities applies an authorized correction without changing the input.
func (s Service) CorrectIdentities(m Interview, input CorrectIdentitiesInput) (Interview, error) {
	if input.Reason == "" {
		return Interview{}, fmt.Errorf("%w: reason is required", ErrInvalid)
	}
	updated := m
	updated.InterviewerID = input.InterviewerID
	updated.IntervieweeID = input.IntervieweeID
	updated.SeasonID = input.SeasonID
	return updated, nil
}

// Delete soft-deletes an interview without changing the input.
func (s Service) Delete(m Interview, actorID string, now time.Time) (Interview, error) {
	if actorID != m.InterviewerID {
		return Interview{}, ErrForbidden
	}
	at := now.UTC()
	updated := m
	updated.DeletedAt = &at
	return updated, nil
}

func Passed(m Interview) bool {
	has := false
	for _, r := range m.Rounds {
		for _, score := range scoreValues(r.Scores) {
			has = true
			if score < 5 {
				return false
			}
		}
	}
	return has
}

func validate(m Interview) error {
	if m.InterviewerID == "" || m.IntervieweeID == "" || m.InterviewerID == m.IntervieweeID || m.DurationMinutes <= 0 || len(m.Rounds) == 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for _, r := range m.Rounds {
		if r.ID == "" || seen[r.ID] {
			return ErrInvalid
		}
		seen[r.ID] = true
		if err := validateRound(r); err != nil {
			return err
		}

		values := scoreValues(r.Scores)
		for _, v := range values {
			if v < 0 || v > 10 {
				return fmt.Errorf("%w: score outside 0-10", ErrInvalid)
			}
		}
	}
	return nil
}

func validateRound(r Round) error {
	s := r.Scores
	allNil := func(values ...*int) bool {
		for _, value := range values {
			if value != nil {
				return false
			}
		}
		return true
	}
	switch r.Type {
	case Behavioural:
		if s.Behavioural == nil || !allNil(s.ConfirmQuestions, s.AlgorithmDesign, s.ComplexityAnalysis, s.Coding, s.Testing, s.Custom) {
			return fmt.Errorf("%w: behavioural rounds require only a behavioural score", ErrInvalid)
		}
	case LeetCode:
		if r.ProblemID == "" || s.ConfirmQuestions == nil || s.AlgorithmDesign == nil || s.ComplexityAnalysis == nil || s.Coding == nil || s.Testing == nil || !allNil(s.Behavioural, s.Custom) {
			return fmt.Errorf("%w: leetcode rounds require a problem and all five scores", ErrInvalid)
		}
	case Custom:
		if s.Custom == nil || !allNil(s.Behavioural, s.ConfirmQuestions, s.AlgorithmDesign, s.ComplexityAnalysis, s.Coding, s.Testing) {
			return fmt.Errorf("%w: custom rounds require only a custom score", ErrInvalid)
		}
		if r.Link != "" {
			parsed, err := url.Parse(r.Link)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
				return fmt.Errorf("%w: custom round link must be HTTP or HTTPS", ErrInvalid)
			}
		}
	default:
		return ErrInvalid
	}
	return nil
}

func scoreValues(s Scores) []int {
	out := []int{}
	for _, v := range []*int{s.Behavioural, s.ConfirmQuestions, s.AlgorithmDesign, s.ComplexityAnalysis, s.Coding, s.Testing, s.Custom} {
		if v != nil {
			out = append(out, *v)
		}
	}
	return out
}

func (s *Service) clean(v string) string {
	if s.Sanitize == nil {
		return v
	}
	return s.Sanitize(v)
}

func (s *Service) cleanRounds(in []Round) []Round {
	out := append([]Round(nil), in...)
	for i := range out {
		out[i].Content = s.clean(out[i].Content)
		out[i].IntervieweeComment = s.clean(out[i].IntervieweeComment)
	}
	return out
}

func cloneInterview(in Interview) Interview {
	in.Rounds = append([]Round(nil), in.Rounds...)
	return in
}
