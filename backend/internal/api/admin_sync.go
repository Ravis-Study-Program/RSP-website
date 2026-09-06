package api

import (
	"net/http"
	"time"
)

func (a *API) queueLeetCodeSync(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Recent MFA is required.")
		return
	}
	if a.queueLeetCodeSyncRun == nil {
		writeErrorResponse(w, http.StatusServiceUnavailable, "The LeetCode worker queue is unavailable.")
		return
	}
	if err := a.queueLeetCodeSyncRun(r.Context(), actor.UserID, requestIDFrom(r.Context())); err != nil {
		a.logger.Error("LeetCode sync enqueue failed", "requestId", requestIDFrom(r.Context()), "error", err.Error())
		writeErrorResponse(w, http.StatusServiceUnavailable, "The LeetCode sync request could not be queued.")
		return
	}

	writeJSONResponse(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
