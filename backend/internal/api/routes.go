package api

import (
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

func registerRoutes(mux *http.ServeMux, a *API) {
	registerSystemRoutes(mux, a)
	registerAccountRoutes(mux, a)
	registerProgrammeRoutes(mux, a)
	registerPracticeRoutes(mux, a)
	registerMockInterviewRoutes(mux, a)
	registerAdminRoutes(mux, a)
}

func registerSystemRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/health/live", a.live)
	mux.HandleFunc("GET /api/v2/health/ready", a.readiness)
	mux.HandleFunc("GET /api/v2/metrics", a.metrics)
}

func registerAccountRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/me", a.protected(ratelimit.Read, a.me))
	mux.HandleFunc("PATCH /api/v2/me", a.protected(ratelimit.Write, a.updateMe))
	mux.HandleFunc("GET /api/v2/me/slug-suggestion", a.protected(ratelimit.Read, a.suggestMeSlug))
	mux.HandleFunc("GET /api/v2/users", a.protected(ratelimit.Read, a.users))
	mux.HandleFunc("GET /api/v2/users/{id}", a.protected(ratelimit.Read, a.user))
	mux.HandleFunc("GET /api/v2/admin/users", a.protected(ratelimit.Read, a.listAdminUsers))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/account-state", a.protected(ratelimit.Sensitive, a.setUserAccountState))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Sensitive, a.grantUserGlobalRole))
	mux.HandleFunc("GET /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Read, a.listUserGlobalRoles))
	mux.HandleFunc("DELETE /api/v2/admin/users/{id}/global-roles/{role}", a.protected(ratelimit.Sensitive, a.revokeUserGlobalRole))
}

func registerProgrammeRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/seasons", a.protected(ratelimit.Read, a.seasons))
	mux.HandleFunc("POST /api/v2/seasons", a.protected(ratelimit.Write, a.createSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}", a.protected(ratelimit.Read, a.season))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}", a.protected(ratelimit.Write, a.updateSeason))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/resources", a.protected(ratelimit.Write, a.updateSeasonResources))
	mux.HandleFunc("POST /api/v2/seasons/{id}/close", a.protected(ratelimit.Write, a.closeSeason))
	mux.HandleFunc("POST /api/v2/seasons/{id}/reopen", a.protected(ratelimit.Write, a.reopenSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Read, a.listWeeks))
	mux.HandleFunc("POST /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Write, a.createWeek))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.updateWeek))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.deleteWeek))
	mux.HandleFunc("GET /api/v2/seasons/{id}/members", a.protected(ratelimit.Read, a.listMembers))
	mux.HandleFunc("GET /api/v2/seasons/{id}/enrollment-candidates", a.protected(ratelimit.Read, a.listEnrollmentCandidates))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members", a.protected(ratelimit.Write, a.createMember))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/members/{memberId}", a.protected(ratelimit.Write, a.updateMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/promote", a.protected(ratelimit.Write, a.promoteMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/remove", a.protected(ratelimit.Write, a.removeMember))
	mux.HandleFunc("GET /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Read, a.listMentorships))
	mux.HandleFunc("POST /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Write, a.createMentorship))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.updateMentorship))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.deleteMentorship))
}

func registerPracticeRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/me/practice-settings", a.protected(ratelimit.Read, a.practiceSettings))
	mux.HandleFunc("PATCH /api/v2/me/practice-settings", a.protected(ratelimit.Write, a.updatePracticeSettings))
	mux.HandleFunc("GET /api/v2/users/{id}/practice-settings", a.protected(ratelimit.Read, a.userPracticeSettings))
	mux.HandleFunc("POST /api/v2/users/{id}/practice-goals/enable", a.protected(ratelimit.Write, a.enablePracticeGoals))
	mux.HandleFunc("GET /api/v2/leetcode-problems", a.protected(ratelimit.Read, a.problems))
	mux.HandleFunc("GET /api/v2/problem-attempts", a.protected(ratelimit.Read, a.attempts))
	mux.HandleFunc("POST /api/v2/problem-attempts", a.protected(ratelimit.Write, a.createAttempt))
	mux.HandleFunc("PATCH /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.updateAttempt))
	mux.HandleFunc("DELETE /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.deleteAttempt))
}

func registerMockInterviewRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/mock-interviews/participants", a.protected(ratelimit.Read, a.listMockParticipants))
	mux.HandleFunc("GET /api/v2/mock-interviews", a.protected(ratelimit.Read, a.listMocks))
	mux.HandleFunc("POST /api/v2/mock-interviews", a.protected(ratelimit.Write, a.createMock))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.updateMock))
	mux.HandleFunc("DELETE /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.deleteMock))
	mux.HandleFunc("POST /api/v2/mock-interviews/{id}/identity-correction", a.protected(ratelimit.Write, a.correctMockIdentities))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}/rounds/{roundId}/review", a.protected(ratelimit.Write, a.reviewRound))
}

func registerAdminRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("POST /api/v2/admin/leetcode/sync", a.protected(ratelimit.Sensitive, a.adminSync))
}
