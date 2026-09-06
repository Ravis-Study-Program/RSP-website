//go:build integration

package api

import (
	"context"
	"testing"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

func TestMentorReassignmentPreservesPreviousStudents(t *testing.T) {
	f := newPostgresFixture(t)
	ctx := context.Background()
	nextMentor := id.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.users(id) VALUES($1)`, nextMentor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO app.enrollments(user_id,season_id,role,student_level,state) VALUES($1,$2,'mentor','not_applicable','active')`, nextMentor, seasonID); err != nil {
		t.Fatal(err)
	}
	old, err := f.db.CreateMentorship(ctx, programme.MentorshipRecord{ID: id.New(), SeasonID: seasonID, MentorUserID: otherID, StudentUserID: studentID}, otherID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	changed, err := f.db.UpdateMentorship(ctx, dal.UpdateMentorshipInput{SeasonID: seasonID, MentorshipID: old.ID, MentorUserID: nextMentor, StudentUserID: studentID, ActorID: otherID, ChangedAt: time.Now().Add(time.Hour)})
	if err != nil || changed.ID == old.ID {
		t.Fatalf("reassignment=%+v err=%v", changed, err)
	}
	items, _, total, err := f.db.ListMentorships(ctx, dal.MentorshipQuery{SeasonID: seasonID, Limit: 20})
	if err != nil || total != 1 || items[0].MentorUserID != nextMentor {
		t.Fatalf("current=%+v total=%d err=%v", items, total, err)
	}
	items, _, total, err = f.db.ListMentorships(ctx, dal.MentorshipQuery{SeasonID: seasonID, MentorUserID: otherID, IncludeEnded: true, Limit: 20})
	if err != nil || total != 1 || items[0].EndedAt == nil {
		t.Fatalf("previous=%+v total=%d err=%v", items, total, err)
	}
}
