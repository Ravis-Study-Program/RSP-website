package api

import (
	"net/http"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/ratelimit"
)

func registerRoutes(mux *http.ServeMux, a *API) {
	registerSystemRoutes(mux, a)
	registerAccountRoutes(mux, a)
	registerProgramRoutes(mux, a)
	registerPracticeRoutes(mux, a)
	registerMockInterviewRoutes(mux, a)
	registerAdminRoutes(mux, a)
	registerCanaryRoutes(mux, a)
	registerJobRoutes(mux, a)
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
	mux.HandleFunc("POST /api/v2/admin/users/{id}/deletion", a.protected(ratelimit.Sensitive, a.deleteUser))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Sensitive, a.grantUserGlobalRole))
	mux.HandleFunc("GET /api/v2/admin/users/{id}/global-roles", a.protected(ratelimit.Read, a.listUserGlobalRoles))
	mux.HandleFunc("DELETE /api/v2/admin/users/{id}/global-roles/{role}", a.protected(ratelimit.Sensitive, a.revokeUserGlobalRole))
}

func registerProgramRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/seasons/{id}/invitations", a.protected(ratelimit.Read, a.listInvitations))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations", a.protected(ratelimit.Sensitive, a.createInvitation))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/batch", a.protected(ratelimit.Sensitive, a.createInvitationBatch))
	// Previewing changes nothing, so it is limited as an ordinary write.
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/batch/preview", a.protected(ratelimit.Write, a.previewInvitationBatch))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/{invitationId}/resend", a.protected(ratelimit.Sensitive, a.resendInvitation))
	mux.HandleFunc("POST /api/v2/seasons/{id}/invitations/{invitationId}/cancel", a.protected(ratelimit.Sensitive, a.cancelInvitation))
	// Public: the link's token is the credential, and only reveals who it was sent to.
	mux.HandleFunc("POST /api/v2/invitations/invitee", a.getInvitationInvitee)
	mux.HandleFunc("POST /api/v2/invitations/preview", a.protected(ratelimit.Read, a.previewInvitation))
	mux.HandleFunc("POST /api/v2/invitations/accept", a.protected(ratelimit.Write, a.acceptInvitation))
	mux.HandleFunc("GET /api/v2/seasons", a.protected(ratelimit.Read, a.listSeasons))
	mux.HandleFunc("POST /api/v2/seasons", a.protected(ratelimit.Write, a.createSeason))
	mux.HandleFunc("POST /api/v2/seasons/import", a.protected(ratelimit.Sensitive, a.importSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/activity-summary", a.protected(ratelimit.Read, a.getSeasonActivitySummary))
	mux.HandleFunc("GET /api/v2/seasons/{id}", a.protected(ratelimit.Read, a.getSeason))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}", a.protected(ratelimit.Sensitive, a.deleteSeason))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}", a.protected(ratelimit.Write, a.updateSeason))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/resources", a.protected(ratelimit.Write, a.updateSeasonResources))
	mux.HandleFunc("POST /api/v2/seasons/{id}/close", a.protected(ratelimit.Write, a.closeSeason))
	mux.HandleFunc("POST /api/v2/seasons/{id}/reopen", a.protected(ratelimit.Write, a.reopenSeason))
	mux.HandleFunc("GET /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Read, a.listWeeks))
	mux.HandleFunc("POST /api/v2/seasons/{id}/weeks", a.protected(ratelimit.Write, a.createWeek))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.updateWeek))
	mux.HandleFunc("PUT /api/v2/seasons/{id}/weeks/{weekId}/resources", a.protected(ratelimit.Write, a.updateWeekResources))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/weeks/{weekId}", a.protected(ratelimit.Write, a.deleteWeek))
	mux.HandleFunc("GET /api/v2/seasons/{id}/members", a.protected(ratelimit.Read, a.listSeasonMembers))
	mux.HandleFunc("GET /api/v2/seasons/{id}/enrollment-candidates", a.protected(ratelimit.Read, a.listEnrollmentCandidates))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members", a.protected(ratelimit.Write, a.createSeasonMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/correction", a.protected(ratelimit.Sensitive, a.correctEnrollment))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/members/{memberId}", a.protected(ratelimit.Write, a.updateSeasonMember))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/members/{memberId}/student-level", a.protected(ratelimit.Write, a.updateStudentLevel))
	mux.HandleFunc("GET /api/v2/seasons/{id}/members/{memberId}/requirements", a.protected(ratelimit.Read, a.getStudentRequirements))
	mux.HandleFunc("PUT /api/v2/seasons/{id}/members/{memberId}/requirements", a.protected(ratelimit.Write, a.putStudentRequirements))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/promote", a.protected(ratelimit.Write, a.promoteSeasonMember))
	mux.HandleFunc("POST /api/v2/seasons/{id}/members/{memberId}/remove", a.protected(ratelimit.Write, a.removeSeasonMember))
	mux.HandleFunc("GET /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Read, a.listMentorships))
	mux.HandleFunc("POST /api/v2/seasons/{id}/mentorships", a.protected(ratelimit.Write, a.createMentorship))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.updateMentorship))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/mentorships/{mentorshipId}", a.protected(ratelimit.Write, a.deleteMentorship))
	mux.HandleFunc("GET /api/v2/seasons/{id}/monday-updates", a.protected(ratelimit.Read, a.listMondayUpdates))
	mux.HandleFunc("POST /api/v2/seasons/{id}/monday-updates", a.protected(ratelimit.Write, a.createMondayUpdate))
	mux.HandleFunc("DELETE /api/v2/seasons/{id}/monday-updates/{updateId}", a.protected(ratelimit.Write, a.deleteMondayUpdate))
	mux.HandleFunc("PATCH /api/v2/seasons/{id}/monday-updates/{updateId}", a.protected(ratelimit.Write, a.editMondayUpdate))
	mux.HandleFunc("GET /api/v2/seasons/{id}/monday-update-unlocks", a.protected(ratelimit.Read, a.listMondayUpdateUnlocks))
	mux.HandleFunc("POST /api/v2/seasons/{id}/monday-update-unlocks", a.protected(ratelimit.Write, a.createMondayUpdateUnlock))
	mux.HandleFunc("POST /api/v2/seasons/{id}/monday-update-unlocks/{unlockId}/decision", a.protected(ratelimit.Write, a.decideMondayUpdateUnlock))
	mux.HandleFunc("PUT /api/v2/seasons/{id}/monday-updates/{updateId}/mark", a.protected(ratelimit.Write, a.putMondayUpdateMark))
	mux.HandleFunc("PUT /api/v2/seasons/{id}/monday-week-marks", a.protected(ratelimit.Write, a.putMondayWeekMark))
	mux.HandleFunc("GET /api/v2/seasons/{id}/monday-mentor-reports", a.protected(ratelimit.Read, a.listMondayMentorReports))
	mux.HandleFunc("GET /api/v2/seasons/{id}/monday-update-review", a.protected(ratelimit.Read, a.listMondayUpdateReview))
	mux.HandleFunc("GET /api/v2/seasons/{id}/monday-update-activity", a.protected(ratelimit.Read, a.getMondayUpdateActivity))
}

func registerPracticeRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/me/practice-settings", a.protected(ratelimit.Read, a.getCurrentUserPracticeSettings))
	mux.HandleFunc("GET /api/v2/users/{id}/practice-settings", a.protected(ratelimit.Read, a.getUserPracticeSettings))
	mux.HandleFunc("PUT /api/v2/practice-settings", a.protected(ratelimit.Write, a.updatePracticeSettings))
	mux.HandleFunc("GET /api/v2/me/problem-suggestion", a.protected(ratelimit.Read, a.suggestProblem))
	mux.HandleFunc("GET /api/v2/leetcode-problems", a.protected(ratelimit.Read, a.listLeetCodeProblems))
	mux.HandleFunc("GET /api/v2/leetcode-categories", a.protected(ratelimit.Read, a.listLeetCodeCategories))
	mux.HandleFunc("GET /api/v2/problem-attempts", a.protected(ratelimit.Read, a.listProblemAttempts))
	mux.HandleFunc("POST /api/v2/problem-attempts", a.protected(ratelimit.Write, a.createProblemAttempt))
	mux.HandleFunc("PATCH /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.updateProblemAttempt))
	mux.HandleFunc("PATCH /api/v2/problem-attempts/{id}/review", a.protected(ratelimit.Write, a.reviewProblemAttempt))
	mux.HandleFunc("DELETE /api/v2/problem-attempts/{id}", a.protected(ratelimit.Write, a.deleteProblemAttempt))
}

func registerMockInterviewRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/mock-interviews/participants", a.protected(ratelimit.Read, a.listMockInterviewParticipants))
	mux.HandleFunc("GET /api/v2/mock-interviews/summary", a.protected(ratelimit.Read, a.getMockInterviewSummary))
	mux.HandleFunc("GET /api/v2/mock-interviews", a.protected(ratelimit.Read, a.listMockInterviews))
	mux.HandleFunc("POST /api/v2/mock-interviews", a.protected(ratelimit.Write, a.createMockInterview))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.updateMockInterview))
	mux.HandleFunc("DELETE /api/v2/mock-interviews/{id}", a.protected(ratelimit.Write, a.deleteMockInterview))
	mux.HandleFunc("POST /api/v2/mock-interviews/{id}/identity-correction", a.protected(ratelimit.Write, a.correctMockInterviewIdentities))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}/rounds/{roundId}/review", a.protected(ratelimit.Write, a.reviewMockInterviewRound))
	mux.HandleFunc("PATCH /api/v2/mock-interviews/{id}/interviewee-notes", a.protected(ratelimit.Write, a.setMockIntervieweeNotes))
}

func registerAdminRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/admin/users/{id}/profile", a.protected(ratelimit.Read, a.getAdminProfile))
	mux.HandleFunc("PATCH /api/v2/admin/users/{id}/profile", a.protected(ratelimit.Sensitive, a.updateAdminProfile))
	mux.HandleFunc("POST /api/v2/admin/users/{id}/email-change", a.protected(ratelimit.Sensitive, a.requestAdminEmailChange))
	mux.HandleFunc("POST /api/v2/admin/leetcode/sync", a.protected(ratelimit.Sensitive, a.queueLeetCodeSync))
}
