package api

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

// registerCanaryRoutes exists only in canary deployments, where testers choose
// their own roles. Production never registers these routes, so they return 404.
func registerCanaryRoutes(mux *http.ServeMux, a *API) {
	if !a.canary {
		return
	}
	mux.HandleFunc("GET /api/v2/canary/access", a.protected(ratelimit.Read, a.getCanaryAccess))
	mux.HandleFunc("PUT /api/v2/canary/access", a.protected(ratelimit.Write, a.setCanaryAccess))
}

type setCanaryAccessRequest struct {
	GlobalRoles []string `json:"globalRoles"`
	SeasonRole  string   `json:"seasonRole"`
}

func (a *API) getCanaryAccess(w http.ResponseWriter, r *http.Request) {
	access, err := a.db.GetCanaryAccess(r.Context(), actorFrom(r.Context()).UserID)
	if err != nil {
		a.writeCanaryError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, access)
}

func (a *API) setCanaryAccess(w http.ResponseWriter, r *http.Request) {
	var request setCanaryAccessRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.GlobalRoles == nil {
		request.GlobalRoles = []string{}
	}
	for _, role := range request.GlobalRoles {
		if role != "director" && role != "system_admin" {
			writeErrorResponse(w, http.StatusBadRequest, "globalRoles may contain only director and system_admin")
			return
		}
	}
	slices.Sort(request.GlobalRoles)
	request.GlobalRoles = slices.Compact(request.GlobalRoles)
	if !slices.Contains([]string{"student", "mentor", "coordinator", dal.CanaryRemovedRole}, request.SeasonRole) {
		writeErrorResponse(w, http.StatusBadRequest, "seasonRole must be student, mentor, coordinator or removed")
		return
	}

	actor := actorFrom(r.Context())
	access, err := a.db.SetCanaryAccess(r.Context(), dal.SetCanaryAccessInput{
		UserID:      actor.UserID,
		GlobalRoles: request.GlobalRoles,
		SeasonRole:  request.SeasonRole,
		ChangedAt:   time.Now().UTC(),
	})
	if err != nil {
		a.writeCanaryError(w, err)
		return
	}
	a.logger.Info("canary access changed", "requestId", requestIDFrom(r.Context()), "userId", actor.UserID, "globalRoles", request.GlobalRoles, "seasonRole", request.SeasonRole)
	writeJSONResponse(w, http.StatusOK, access)
}

func (a *API) writeCanaryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dal.ErrNotFound):
		writeErrorResponse(w, http.StatusConflict, "The canary season is missing. Redeploy the canary to seed it again.")
	case errors.Is(err, dal.ErrConflict):
		writeErrorResponse(w, http.StatusConflict, "The canary season is closed. Reopen it or redeploy the canary.")
	default:
		a.writeStoreErrorResponse(w, err)
	}
}
