package practice

import (
	"strings"
	"testing"
	"time"
)

func confidence(v int) *int { return &v }
func TestRecommendationRaisesAndTargetsWeakCategory(t *testing.T) {
	now := time.Now().UTC()
	attempts := []Attempt{}
	for i := 0; i < 4; i++ {
		attempts = append(attempts, Attempt{ProblemID: "done", Difficulty: Easy, Categories: []string{"arrays"}, Outcome: Independent, Confidence: confidence(5), Minutes: 15, AttemptedAt: now.Add(-time.Duration(i) * time.Hour)})
	}
	problems := []Problem{{ID: "premium", Difficulty: Medium, Categories: []string{"graphs"}, Premium: true}, {ID: "next", Title: "Next", Difficulty: Medium, Categories: []string{"arrays"}}}
	got, err := Select(Request{UserID: "u", Level: Beginner, Problems: problems, Attempts: attempts, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if got.Difficulty != Medium || got.Problem.ID != "next" || !strings.Contains(got.Rationale, "adjusted") {
		t.Fatalf("unexpected: %#v", got)
	}
}
func TestRecommendationLowersAndHonoursDismissal(t *testing.T) {
	now := time.Now().UTC()
	attempts := []Attempt{{ProblemID: "a", Difficulty: Hard, Categories: []string{"graphs"}, Outcome: NotSolved, Minutes: 60, AttemptedAt: now}, {ProblemID: "b", Difficulty: Hard, Categories: []string{"graphs"}, Outcome: NotSolved, Minutes: 60, AttemptedAt: now.Add(-time.Hour)}}
	problems := []Problem{{ID: "first", Difficulty: Medium, Categories: []string{"graphs"}}, {ID: "second", Difficulty: Medium, Categories: []string{"graphs"}}}
	got, err := Select(Request{UserID: "u", Level: Advanced, Problems: problems, Attempts: attempts, Dismissals: []Dismissal{{"first", now.Add(-time.Hour)}}, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if got.Difficulty != Medium || got.Category != "graphs" || got.Problem.ID != "second" {
		t.Fatalf("unexpected: %#v", got)
	}
}
func TestRecommendationKeepsActiveAndRetestAfterNinetyDays(t *testing.T) {
	now := time.Now().UTC()
	active := Recommendation{ID: "active", Problem: Problem{ID: "x"}}
	got, err := Select(Request{Active: &active, Now: now})
	if err != nil || got.ID != "active" {
		t.Fatal("active not retained")
	}
	p := Problem{ID: "old", Difficulty: Medium}
	got, err = Select(Request{UserID: "u", Level: NonStudent, Problems: []Problem{p}, Attempts: []Attempt{{ProblemID: "old", Difficulty: Medium, Outcome: NotSolved, AttemptedAt: now.Add(-91 * 24 * time.Hour)}}, Now: now})
	if err != nil || got.Problem.ID != "old" {
		t.Fatalf("retest not selected: %v", err)
	}
}

func TestMigratedOutcomesAffectExposureButNotQualityAndOnlyWeakProblemsRetest(t *testing.T) {
	now := time.Now().UTC()
	weak := Attempt{ProblemID: "weak", Difficulty: Medium, Categories: []string{"graphs"}, Outcome: NotSolved, Migrated: true, AttemptedAt: now.Add(-100 * 24 * time.Hour)}
	strong := Attempt{ProblemID: "strong", Difficulty: Medium, Categories: []string{"arrays"}, Outcome: Independent, Confidence: confidence(5), Minutes: 10, AttemptedAt: now.Add(-100 * 24 * time.Hour)}
	problems := []Problem{{ID: "strong", Difficulty: Medium, Categories: []string{"arrays"}}, {ID: "weak", Difficulty: Medium, Categories: []string{"graphs"}}}
	if got, err := Select(Request{UserID: "u", Level: NonStudent, Problems: problems, Attempts: []Attempt{weak, strong}, Now: now}); err != ErrNoCandidate {
		t.Fatalf("migrated outcome made a problem eligible for weak retest: result=%#v err=%v", got, err)
	}
	weak.Migrated = false
	got, err := Select(Request{UserID: "u", Level: NonStudent, Problems: problems, Attempts: []Attempt{weak, strong}, Now: now})
	if err != nil || got.Problem.ID != "weak" {
		t.Fatalf("non-migrated weak retest result=%#v err=%v", got, err)
	}
	weak.Migrated = true
	known := knownNewest([]Attempt{weak}, 20)
	if len(known) != 0 {
		t.Fatalf("migrated known outcome affected quality: %#v", known)
	}
	if category := weakestCategory(known, []Attempt{weak}, DefaultGoals()); category != "" {
		t.Fatalf("migrated exposure became quality signal: %q", category)
	}
}

func TestRecommendationUsesEnabledPersonalGoals(t *testing.T) {
	now := time.Now().UTC()
	attempts := []Attempt{}
	for i := 0; i < 3; i++ {
		attempts = append(attempts, Attempt{ProblemID: "done", Difficulty: Medium, Outcome: Independent, Confidence: confidence(5), Minutes: 40, AttemptedAt: now.Add(-time.Duration(i) * time.Hour)})
	}
	problems := []Problem{{ID: "easy", Difficulty: Easy}, {ID: "medium", Difficulty: Medium}}
	withDefaults, err := Select(Request{UserID: "u", Level: Intermediate, Problems: problems, Attempts: attempts, Now: now})
	if err != nil || withDefaults.Difficulty != Easy {
		t.Fatalf("default goal result=%#v err=%v", withDefaults, err)
	}
	withPersonal, err := Select(Request{UserID: "u", Level: Intermediate, Problems: problems, Attempts: attempts, Goals: Goals{Easy: 25, Medium: 50, Hard: 70}, Now: now})
	if err != nil || withPersonal.Difficulty != Medium {
		t.Fatalf("personal goal result=%#v err=%v", withPersonal, err)
	}
}
