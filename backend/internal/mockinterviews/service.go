package mockinterviews

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("stale revision")
	ErrInvalid   = errors.New("invalid mock interview")
)

type RoundType string

const (
	Behavioural RoundType = "behavioural"
	LeetCode    RoundType = "leetcode"
	Custom      RoundType = "custom"
)

type Participant struct {
	UserID                                                     string
	ActiveMember, Alumni, Suspended, Deleted, Test, KickedOnly bool
}

func (p Participant) Eligible() bool {
	return (p.ActiveMember || p.Alumni) && !p.Suspended && !p.Deleted && !p.Test && !p.KickedOnly
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
	ID                       string    `json:"id"`
	Type                     RoundType `json:"type"`
	ProblemID, Content, Link string
	Scores                   Scores `json:"scores"`
	Reviewed                 bool   `json:"reviewed"`
	IntervieweeComment       string `json:"intervieweeComment"`
}
type Interview struct {
	ID, InterviewerID, IntervieweeID string
	SeasonID                         *string
	OccurredAt                       time.Time
	DurationMinutes                  int
	Notes                            string
	Rounds                           []Round
	Revision                         int64
	DeletedAt                        *time.Time
}
type Version struct {
	InterviewID     string
	Revision        int64
	ActorID, Reason string
	SavedAt         time.Time
	Snapshot        json.RawMessage
}
type Service struct {
	Sanitize func(string) string
	Versions []Version
}
type CreateInput struct {
	Interviewee     Participant
	SeasonID        *string
	OccurredAt      time.Time
	DurationMinutes int
	Notes           string
	Rounds          []Round
}

func (s *Service) Create(actorID string, in CreateInput, now time.Time) (Interview, error) {
	if actorID == "" || !in.Interviewee.Eligible() {
		return Interview{}, ErrForbidden
	}
	m := Interview{ID: "mock-" + actorID + "-" + now.UTC().Format("20060102150405.000000000"), InterviewerID: actorID, IntervieweeID: in.Interviewee.UserID, SeasonID: in.SeasonID, OccurredAt: in.OccurredAt.UTC(), DurationMinutes: in.DurationMinutes, Notes: s.clean(in.Notes), Rounds: s.cleanRounds(in.Rounds), Revision: 1}
	if err := validate(m); err != nil {
		return Interview{}, err
	}
	s.save(m, actorID, "created", now)
	return m, nil
}

type UpdateInput struct {
	ExpectedRevision int64
	OccurredAt       time.Time
	DurationMinutes  int
	Notes            string
	Rounds           []Round
}

func (s *Service) Update(m *Interview, actorID string, in UpdateInput, now time.Time) error {
	if actorID != m.InterviewerID {
		return ErrForbidden
	}
	if m.Revision != in.ExpectedRevision {
		return ErrConflict
	}
	candidate := *m
	candidate.OccurredAt = in.OccurredAt.UTC()
	candidate.DurationMinutes = in.DurationMinutes
	candidate.Notes = s.clean(in.Notes)
	candidate.Rounds = s.cleanRounds(in.Rounds)
	candidate.Revision++
	if err := validate(candidate); err != nil {
		return err
	}
	*m = candidate
	s.save(*m, actorID, "updated", now)
	return nil
}
func (s *Service) Review(m *Interview, actorID, roundID, comment string, reviewed bool, expectedRevision int64, now time.Time) error {
	if actorID != m.IntervieweeID {
		return ErrForbidden
	}
	if m.Revision != expectedRevision {
		return ErrConflict
	}
	found := false
	for i := range m.Rounds {
		if m.Rounds[i].ID == roundID {
			m.Rounds[i].IntervieweeComment = s.clean(comment)
			m.Rounds[i].Reviewed = reviewed
			found = true
			break
		}
	}
	if !found {
		return ErrInvalid
	}
	m.Revision++
	s.save(*m, actorID, "interviewee review", now)
	return nil
}
func (s *Service) CorrectIdentities(m *Interview, actorID, newInterviewer, newInterviewee string, newSeason *string, reason string, privileged bool, expectedRevision int64, now time.Time) error {
	if !privileged || reason == "" {
		return ErrForbidden
	}
	if m.Revision != expectedRevision {
		return ErrConflict
	}
	m.InterviewerID = newInterviewer
	m.IntervieweeID = newInterviewee
	m.SeasonID = newSeason
	m.Revision++
	s.save(*m, actorID, "identity correction: "+reason, now)
	return nil
}
func (s *Service) Delete(m *Interview, actorID string, expectedRevision int64, now time.Time) error {
	if actorID != m.InterviewerID {
		return ErrForbidden
	}
	if m.Revision != expectedRevision {
		return ErrConflict
	}
	at := now.UTC()
	m.DeletedAt = &at
	m.Revision++
	s.save(*m, actorID, "soft deleted", now)
	return nil
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
		values := scoreValues(r.Scores)
		if len(values) == 0 {
			return ErrInvalid
		}
		for _, v := range values {
			if v < 0 || v > 10 {
				return fmt.Errorf("%w: score outside 0-10", ErrInvalid)
			}
		}
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
func (s *Service) save(m Interview, actor, reason string, now time.Time) {
	raw, _ := json.Marshal(m)
	s.Versions = append(s.Versions, Version{m.ID, m.Revision, actor, reason, now.UTC(), raw})
}
