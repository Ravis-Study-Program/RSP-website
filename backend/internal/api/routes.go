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
	mux.HandleFunc("GET /api/v2/health/live", a.checkLiveness)
	mux.HandleFunc("GET /api/v2/health/ready", a.checkReadiness)
	mux.HandleFunc("GET /api/v2/metrics", a.getMetrics)
}

func registerAccountRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/me", a.protected(ratelimit.Read, a.getCurrentUser))
	mux.HandleFunc("PATCH /api/v2/me", a.protected(ratelimit.Write, a.updateCurrentUser))
	mux.HandleFunc("GET /api/v2/me/slug-suggestion", a.protected(ratelimit.Read, a.suggestCurrentUserSlug))
	mux.HandleFunc("GET /api/v2/users", a.protected(ratelimit.Read, a.listUsers))
	mux.HandleFunc("GET /api/v2/users/{id}/participation", a.protected(ratelimit.Read, a.getUserParticipation))
	mux.HandleFunc("GET /api/v2/users/{id}/activity-summary", a.protected(ratelimit.Read, a.getUserActivitySummary))
	mux.HandleFunc("GET /api/v2/users/{id}", a.protected(ratelimit.Read, a.getUser))
	mux.HandleFunc("GET /api/v2/admin/users", a.protected(ratelimit.Read, a.listAdminUsers))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/account-state", a.protected(ratelimit.Sensitive, a.setUserAccountState))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Sensitive, a.grantUserGlobalRole))
	mux.HandleFunc("GET /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Read, a.listUserGlobalRoles))
	mux.HandleFunc("DELETE /api/v2/admin/users/{id}/global-roles/{role}", a.protected(ratelimit.Sensitive, a.revokeUserGlobalRole))
}

func registerProgrammeRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/seasons/{id}/invitations", a.protected(ratelimit.Read, a.listInvitations))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations", a.protected(ratelimit.Sensitive, a.createInvitation))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/{invitationId}/resend", a.protected(ratelimit.Sensitive, a.resendInvitation))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/{invitationId}/cancel", a.protected(ratelimit.Sensitive, a.cancelInvitation))
	mux.HandleFunc("POST /api/v2/invitations/preview", a.protected(ratelimit.Read, a.previewInvitation))
	mux.HandleFunc("POST /api/v2/invitations/accept", a.protected(ratelimit.Write, a.acceptInvitation))
	mux.HandleFunc("GET /api/v2/seasons", a.protected(ratelimit.Read, a.listSeasons))
	mux.HandleFunc("POST /api/v2/seasons", a.protected(ratelimit.Write, a.createSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/activity-summary", a.protected(ratelimit.Read, a.getSeasonActivitySummary))
	mux.HandleFunc("GET /api/v2/seasons/{id}", a.protected(ratelimit.Read, a.getSeason))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}", a.protected(ratelimit.Write, a.updateSeason))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/resources", a.protected(ratelimit.Write, a.updateSeasonResources))
	mux.HandleFunc("POST /api/v2/seasons/{id}/close", a.protected(ratelimit.Write, a.closeSeason))
	mux.HandleFunc("POST /api/v2/seasons/{id}/reopen", a.protected(ratelimit.Write, a.reopenSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Read, a.listWeeks))
	mux.HandleFunc("POST /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Write, a.createWeek))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.updateWeek))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.deleteWeek))
	mux.HandleFunc("GET /api/v2/seasons/{id}/members", a.protected(ratelimit.Read, a.listSeasonMembers))
	mux.HandleFunc("GET /api/v2/seasons/{id}/enrollment-candidates", a.protected(ratelimit.Read, a.listEnrollmentCandidates))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members", a.protected(ratelimit.Write, a.createSeasonMember))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/members/{memberId}", a.protected(ratelimit.Write, a.updateSeasonMember))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/members/{memberId}/student-level", a.protected(ratelimit.Write, a.updateStudentLevel))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/promote", a.protected(ratelimit.Write, a.promoteSeasonMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/remove", a.protected(ratelimit.Write, a.removeSeasonMember))
	mux.HandleFunc("GET /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Read, a.listMentorships))
	mux.HandleFunc("POST /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Write, a.createMentorship))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.updateMentorship))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.deleteMentorship))
}

func registerPracticeRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/me/practice-settings", a.protected(ratelimit.Read, a.getCurrentUserPracticeSettings))
	mux.HandleFunc("GET /api/v2/users/{id}/practice-settings", a.protected(ratelimit.Read, a.getUserPracticeSettings))
	mux.HandleFunc("GET /api/v2/me/problem-suggestion", a.protected(ratelimit.Read, a.suggestProblem))
	mux.HandleFunc("GET /api/v2/leetcode-problems", a.protected(ratelimit.Read, a.listLeetCodeProblems))
	mux.HandleFunc("GET /api/v2/problem-attempts", a.protected(ratelimit.Read, a.listProblemAttempts))
	mux.HandleFunc("POST /api/v2/problem-attempts", a.protected(ratelimit.Write, a.createProblemAttempt))
	mux.HandleFunc("PATCH /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.updateProblemAttempt))
	mux.HandleFunc("DELETE /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.deleteProblemAttempt))
}

func registerMockInterviewRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/mock-interviews/participants", a.protected(ratelimit.Read, a.listMockInterviewParticipants))
	mux.HandleFunc("GET /api/v2/mock-interviews", a.protected(ratelimit.Read, a.listMockInterviews))
	mux.HandleFunc("POST /api/v2/mock-interviews", a.protected(ratelimit.Write, a.createMockInterview))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.updateMockInterview))
	mux.HandleFunc("DELETE /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.deleteMockInterview))
	mux.HandleFunc("POST /api/v2/mock-interviews/{id}/identity-correction", a.protected(ratelimit.Write, a.correctMockInterviewIdentities))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}/rounds/{roundId}/review", a.protected(ratelimit.Write, a.reviewMockInterviewRound))
}

func registerAdminRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/admin/users/{id}/profile", a.protected(ratelimit.Read, a.getAdminProfile))
	mux.HandleFunc("PATCH /api/v2/admin/users/{id}/profile", a.protected(ratelimit.Sensitive, a.updateAdminProfile))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/email-change", a.protected(ratelimit.Sensitive, a.requestAdminEmailChange))
	mux.HandleFunc("POST /api/v2/admin/leetcode/sync", a.protected(ratelimit.Sensitive, a.queueLeetCodeSync))
}
