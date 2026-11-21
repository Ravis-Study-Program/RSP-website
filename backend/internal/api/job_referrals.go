package api

import (
	"errors"
	"net/http"

	"github.com/Ravis-Study-Program/RSP-website/backend/internal/jobs"
	"github.com/Ravis-Study-Program/RSP-website/backend/internal/platform/ratelimit"
)

const referralProfilePath = "/api/v2/jobs/referrals/profile"

type jobReferralAudience int

const (
	referralMembers jobReferralAudience = iota
	referralApplicants
	referralGraduates
)

func registerJobReferralRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/jobs/referrals", a.jobReferralAccess(ratelimit.Read, referralMembers, a.getJobReferralBoard))
	mux.HandleFunc("GET /api/v2/jobs/referrals/counts", a.jobReferralAccess(ratelimit.Read, referralMembers, a.getJobReferralCounts))
	mux.HandleFunc("PUT "+referralProfilePath, a.jobReferralAccess(ratelimit.Write, referralApplicants, a.saveJobReferralProfile))
	mux.HandleFunc("DELETE "+referralProfilePath, a.jobReferralAccess(ratelimit.Write, referralApplicants, a.deleteJobReferralProfile))
	mux.HandleFunc("PUT /api/v2/jobs/referrals/requests/{id}", a.jobReferralAccess(ratelimit.Write, referralApplicants, a.setJobReferralRequest))
	mux.HandleFunc("DELETE /api/v2/jobs/referrals/requests/{id}", a.jobReferralAccess(ratelimit.Write, referralApplicants, a.setJobReferralRequest))
	mux.HandleFunc("PUT /api/v2/jobs/referrals/offers/{id}", a.jobReferralAccess(ratelimit.Write, referralGraduates, a.setJobReferralOffer))
	mux.HandleFunc("DELETE /api/v2/jobs/referrals/offers/{id}", a.jobReferralAccess(ratelimit.Write, referralGraduates, a.setJobReferralOffer))
	mux.HandleFunc("GET /api/v2/jobs/referrals/companies/{id}/candidates", a.jobReferralAccess(ratelimit.Read, referralGraduates, a.getJobReferralCandidates))
}

func (a *API) jobReferralAccess(class ratelimit.Class, audience jobReferralAudience, next http.HandlerFunc) http.HandlerFunc {
	return a.protected(class, jobIDs(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		actor := actorFrom(r.Context())
		if !actor.CanAccessProgram() {
			writeErrorResponse(w, http.StatusForbidden, "Referrals are available to active RSP members and alumni.")
			return
		}
		if audience == referralApplicants && !actor.CanRequestJobReferral() {
			writeErrorResponse(w, http.StatusForbidden, "Only RSP students and alumni can request referrals.")
			return
		}
		if audience == referralGraduates && !actor.IsStudentAlumnus() {
			writeErrorResponse(w, http.StatusForbidden, "Only RSP graduates can refer candidates.")
			return
		}
		next(w, r)
	}))
}

func (a *API) getJobReferralBoard(w http.ResponseWriter, r *http.Request) {
	board, err := a.db.GetJobReferralBoard(r.Context(), actorFrom(r.Context()).UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, board)
}

func (a *API) getJobReferralCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := a.db.ListJobReferralCounts(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"items": counts})
}

func (a *API) saveJobReferralProfile(w http.ResponseWriter, r *http.Request) {
	var profile jobs.ReferralProfile
	if err := decodeJSON(r.Body, &profile); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter your reason and a Google Drive link to your résumé file.")
		return
	}
	if err := profile.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.db.SaveJobReferralProfile(r.Context(), actorFrom(r.Context()).UserID, profile); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteJobReferralProfile(w http.ResponseWriter, r *http.Request) {
	if err := a.db.DeleteJobReferralProfile(r.Context(), actorFrom(r.Context()).UserID); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) setJobReferralRequest(w http.ResponseWriter, r *http.Request) {
	err := a.db.SetJobReferralRequest(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), r.Method == http.MethodPut)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, jobs.ErrReferralProfileRequired), errors.Is(err, jobs.ErrReferralLimit), errors.Is(err, jobs.ErrReferralPositionClosed):
		writeErrorResponse(w, http.StatusConflict, err.Error())
	default:
		a.writeStoreErrorResponse(w, err)
	}
}

func (a *API) setJobReferralOffer(w http.ResponseWriter, r *http.Request) {
	if err := a.db.SetJobReferralOffer(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), r.Method == http.MethodPut); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getJobReferralCandidates(w http.ResponseWriter, r *http.Request) {
	actor, companyID := actorFrom(r.Context()), r.PathValue("id")
	binding := "job_referral_candidates|" + actor.UserID + "|" + companyID + "|id:asc"
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil || (boundary != "" && !uuidSelection(boundary)) {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid candidate cursor or page size.")
		return
	}
	items, more, total, err := a.db.ListJobReferralCandidates(r.Context(), actor.UserID, companyID, boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	info, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v jobs.ReferralCandidate) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[jobs.ReferralCandidate]{Items: items, PageInfo: info, TotalCount: total})
}
