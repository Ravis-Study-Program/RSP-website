//go:build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/mockinterviews"
	"github.com/magedmg/RSP-website/backend/internal/program"
)

func TestProfileSaveReturnsCompleteUser(t *testing.T) {
	f := newPostgresFixture(t)
	createMockForWrites(t, f)
	for _, name := range []string{"Changed Name", "Changed Name"} {
		res := testRequest(t, f.handler, http.MethodPatch, "/api/v2/me", "student", `{"name":"`+name+`","timezone":"Australia/Adelaide"}`)
		var saved accounts.User
		if err := json.Unmarshal(res.Body.Bytes(), &saved); err != nil || res.Code != http.StatusOK {
			t.Fatalf("save profile: %d %s %v", res.Code, res.Body, err)
		}
		if saved.Name != name || saved.Timezone != "Australia/Adelaide" || !saved.TimezoneConfigured {
			t.Fatalf("profile changes missing: %+v", saved)
		}
		if saved.GlobalRoles == nil || saved.SeasonRoles == nil || len(saved.SeasonRoles) != 1 || saved.MockInterviewCount == 0 {
			t.Fatalf("incomplete saved user: %+v", saved)
		}
		res = testRequest(t, f.handler, http.MethodGet, "/api/v2/me", "student", "")
		var loaded accounts.User
		if err := json.Unmarshal(res.Body.Bytes(), &loaded); err != nil || res.Code != http.StatusOK {
			t.Fatalf("reload profile: %d %s %v", res.Code, res.Body, err)
		}
		if !reflect.DeepEqual(saved, loaded) {
			t.Fatalf("save response differs from reload: saved=%+v loaded=%+v", saved, loaded)
		}
	}
}

func TestProfileSlugParticipationAndTargetMockModes(t *testing.T) {
	f := newPostgresFixture(t)
	mock := createMockForWrites(t, f)
	res := testRequest(t, f.handler, http.MethodGet, "/api/v2/users/other", "student", "")
	if res.Code != 200 {
		t.Fatalf("slug profile: %d %s", res.Code, res.Body)
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/users/"+studentID+"/participation", "other", "")
	var history Page[program.Participation]
	if err := json.Unmarshal(res.Body.Bytes(), &history); err != nil || res.Code != 200 || len(history.Items) != 1 || len(history.Items[0].Periods) != 1 || history.Items[0].LastStudentLevel != "beginner" {
		t.Fatalf("participation: %d %s %v", res.Code, res.Body, err)
	}
	for _, tc := range []struct {
		target, mode string
		want         int64
	}{{otherID, "received", 1}, {otherID, "given", 0}, {studentID, "received", 0}, {studentID, "given", 1}, {otherID, "all", 1}} {
		res := testRequest(t, f.handler, http.MethodGet, "/api/v2/mock-interviews?userId="+tc.target+"&mode="+tc.mode, "student", "")
		var page Page[mockinterviews.Interview]
		if err := json.Unmarshal(res.Body.Bytes(), &page); err != nil || res.Code != 200 || page.TotalCount != tc.want {
			t.Fatalf("target %s/%s: %d %s %v", tc.target, tc.mode, res.Code, res.Body, err)
		}
		if tc.want > 0 && page.Items[0].ID != mock.ID {
			t.Fatal("wrong mock in target history")
		}
	}
	// Former staff can navigate to retained history through the season link.
	if _, err := f.pool.Exec(context.Background(), `UPDATE app.enrollments SET state='withdrawn',assignment_state='revoked',activated_at=NULL WHERE id=$1`, studentMemberID); err != nil {
		t.Fatal(err)
	}
	res = testRequest(t, f.handler, http.MethodGet, "/api/v2/users/student?seasonId="+seasonID, "other", "")
	if res.Code != 200 {
		t.Fatalf("departed profile: %d %s", res.Code, res.Body)
	}
}
