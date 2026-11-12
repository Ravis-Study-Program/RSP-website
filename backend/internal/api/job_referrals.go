package api

import (
	"errors"
	"net/http"

	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/jobs"
)

func (a *API) listJobReferrals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	binding := "job_referrals|" + actorFrom(r.Context()).UserID + "|id:asc"
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil || (boundary != "" && !uuidSelection(boundary)) {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid referrals cursor or page size.")
		return
	}
	items, more, total, err := a.db.ListJobReferrals(r.Context(), boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	info, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v jobs.Referral) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[jobs.Referral]{Items: items, PageInfo: info, TotalCount: total})
}

func (a *API) saveJobReferral(w http.ResponseWriter, r *http.Request) {
	var input jobs.ReferralInput
	if err := decodeJSON(r.Body, &input); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter valid referral details.")
		return
	}
	if err := input.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := a.db.SaveJobReferral(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), input)
	if errors.Is(err, dal.ErrDuplicate) {
		writeErrorResponse(w, http.StatusConflict, "You already offer referrals at this company. Edit your existing listing.")
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	status := http.StatusOK
	if r.PathValue("id") == "" {
		status = http.StatusCreated
	}
	writeJSONResponse(w, status, v)
}

func (a *API) deleteJobReferral(w http.ResponseWriter, r *http.Request) {
	if err := a.db.DeleteJobReferral(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id")); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
