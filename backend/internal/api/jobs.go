package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
	"github.com/magedmg/RSP-website/backend/internal/jobs"
	"github.com/magedmg/RSP-website/backend/internal/platform/ratelimit"
)

func registerJobRoutes(mux *http.ServeMux, a *API) {
	mux.HandleFunc("GET /api/v2/jobs/companies", a.listJobCompanies)
	mux.HandleFunc("GET /api/v2/jobs/positions", a.listJobPositions)
	mux.HandleFunc("GET /api/v2/jobs/positions/{id}", jobIDs(a.getJobPosition))
	mux.HandleFunc("POST /api/v2/jobs/positions", a.protected(ratelimit.Write, a.saveJobPosition))
	mux.HandleFunc("PATCH /api/v2/jobs/positions/{id}", a.protected(ratelimit.Write, jobIDs(a.saveJobPosition)))
	mux.HandleFunc("PATCH /api/v2/jobs/companies/{id}", a.protected(ratelimit.Write, jobIDs(a.updateJobCompany)))
	mux.HandleFunc("POST /api/v2/jobs/positions/{id}/merge", a.protected(ratelimit.Write, jobIDs(a.mergeJobPositions)))
	mux.HandleFunc("GET /api/v2/me/job-applications", a.protected(ratelimit.Read, a.listJobApplications))
	mux.HandleFunc("POST /api/v2/me/job-applications", a.protected(ratelimit.Write, a.createJobApplication))
	mux.HandleFunc("PATCH /api/v2/me/job-applications/{id}", a.protected(ratelimit.Write, jobIDs(a.updateJobApplicationStatus)))
	mux.HandleFunc("DELETE /api/v2/me/job-applications/{id}", a.protected(ratelimit.Write, jobIDs(a.deleteJobApplication)))
	mux.HandleFunc("POST /api/v2/me/job-applications/{id}/stages", a.protected(ratelimit.Write, jobIDs(a.saveJobStage)))
	mux.HandleFunc("PATCH /api/v2/me/job-applications/{id}/stages/{stageId}", a.protected(ratelimit.Write, jobIDs(a.saveJobStage)))
	mux.HandleFunc("DELETE /api/v2/me/job-applications/{id}/stages/{stageId}", a.protected(ratelimit.Write, jobIDs(a.deleteJobStage)))
	mux.HandleFunc("GET /api/v2/jobs/referrals", a.protected(ratelimit.Read, a.listJobReferrals))
	mux.HandleFunc("POST /api/v2/jobs/referrals", a.protected(ratelimit.Write, a.saveJobReferral))
	mux.HandleFunc("PATCH /api/v2/jobs/referrals/{id}", a.protected(ratelimit.Write, jobIDs(a.saveJobReferral)))
	mux.HandleFunc("DELETE /api/v2/jobs/referrals/{id}", a.protected(ratelimit.Write, jobIDs(a.deleteJobReferral)))
}

func jobIDs(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, key := range []string{"id", "stageId"} {
			if v := r.PathValue(key); v != "" && !uuidSelection(v) {
				writeErrorResponse(w, http.StatusBadRequest, "Enter a valid resource ID.")
				return
			}
		}
		next(w, r)
	}
}

func requireJobStaff(w http.ResponseWriter, r *http.Request) bool {
	actor := actorFrom(r.Context())
	allowed := actor.IsDirectorOrSystemAdmin()
	for _, e := range actor.Enrollments {
		if e.Role == authz.Coordinator && e.State == authz.Active {
			allowed = true
		}
	}
	if !allowed {
		writeErrorResponse(w, http.StatusForbidden, "Only active coordinators, directors and system admins can manage positions.")
		return false
	}
	if !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Complete two-factor verification before managing positions.")
		return false
	}
	return true
}

func (a *API) listJobCompanies(w http.ResponseWriter, r *http.Request) {
	items, err := a.db.ListJobCompanies(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"items": items})
}

func (a *API) listJobPositions(w http.ResponseWriter, r *http.Request) {
	const binding = "job_positions|id:asc"
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil || (boundary != "" && !uuidSelection(boundary)) {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid positions cursor or page size.")
		return
	}
	items, more, total, err := a.db.ListJobPositions(r.Context(), boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	info, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v jobs.Position) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[jobs.Position]{Items: items, PageInfo: info, TotalCount: total})
}

func (a *API) getJobPosition(w http.ResponseWriter, r *http.Request) {
	v, err := a.db.GetJobPosition(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) saveJobPosition(w http.ResponseWriter, r *http.Request) {
	positionID := r.PathValue("id")
	if positionID != "" && !requireJobStaff(w, r) {
		return
	}
	var input jobs.PositionInput
	if err := decodeJSON(r.Body, &input); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter valid position details.")
		return
	}
	normalized, err := input.Validate()
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if positionID == "" {
		input.Status = "open"
	}
	existing, err := a.db.JobPositionForURL(r.Context(), normalized)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	if existing != "" && existing != positionID {
		a.jobDuplicate(w, r, existing)
		return
	}
	saved, err := a.db.SaveJobPosition(r.Context(), positionID, actorFrom(r.Context()).UserID, normalized, input)
	if errors.Is(err, dal.ErrDuplicate) {
		existing, _ = a.db.JobPositionForURL(r.Context(), normalized)
		a.jobDuplicate(w, r, existing)
		return
	}
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	status := http.StatusOK
	if positionID == "" {
		status = http.StatusCreated
	}
	writeJSONResponse(w, status, saved)
}

func (a *API) jobDuplicate(w http.ResponseWriter, r *http.Request, existing string) {
	writeJSONResponse(w, http.StatusConflict, map[string]string{"message": "This application link is already listed.", "requestId": requestIDFrom(r.Context()), "existingPositionId": existing})
}

func (a *API) updateJobCompany(w http.ResponseWriter, r *http.Request) {
	if !requireJobStaff(w, r) {
		return
	}
	var input struct {
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	if err := decodeJSON(r.Body, &input); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter valid company details.")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 120 || !jobs.ValidCategory(input.Category) {
		writeErrorResponse(w, http.StatusBadRequest, "Enter a company name up to 120 characters and a valid category.")
		return
	}
	v, err := a.db.UpdateJobCompany(r.Context(), actorFrom(r.Context()).UserID, jobs.Company{ID: r.PathValue("id"), Name: input.Name, Category: input.Category})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) mergeJobPositions(w http.ResponseWriter, r *http.Request) {
	if !requireJobStaff(w, r) {
		return
	}
	var input struct {
		TargetID string `json:"targetId"`
	}
	if err := decodeJSON(r.Body, &input); err != nil || !uuidSelection(input.TargetID) {
		writeErrorResponse(w, http.StatusBadRequest, "Choose a position to keep.")
		return
	}
	if input.TargetID == r.PathValue("id") {
		writeErrorResponse(w, http.StatusBadRequest, "Choose a different position to merge into.")
		return
	}
	v, err := a.db.MergeJobPositions(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), input.TargetID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, v)
}

func (a *API) listJobApplications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	userID := actorFrom(r.Context()).UserID
	binding := "job_applications|" + userID + "|id:asc"
	limit, boundary, err := parsePagination(r.URL.Query().Get("limit"), r.URL.Query().Get("cursor"), binding, a.cursorSecret)
	if err != nil || (boundary != "" && !uuidSelection(boundary)) {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid applications cursor or page size.")
		return
	}
	items, more, total, err := a.db.ListJobApplications(r.Context(), userID, boundary, limit)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	info, err := pageInfoForKeyset(a.cursorSecret, binding, "forward", boundary, items, more, func(v jobs.Application) string { return v.ID })
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, Page[jobs.Application]{Items: items, PageInfo: info, TotalCount: total})
}

func (a *API) createJobApplication(w http.ResponseWriter, r *http.Request) {
	var input struct {
		PositionID string `json:"positionId"`
		Date       string `json:"date"`
	}
	if err := decodeJSON(r.Body, &input); err != nil || !uuidSelection(input.PositionID) {
		writeErrorResponse(w, http.StatusBadRequest, "Choose a position and application date.")
		return
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter a valid application date.")
		return
	}
	appID, err := a.db.ApplyToJob(r.Context(), actorFrom(r.Context()).UserID, input.PositionID, input.Date)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]string{"id": appID})
}

func (a *API) updateJobApplicationStatus(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Status string `json:"status"`
		Date   string `json:"date"`
	}
	if err := decodeJSON(r.Body, &input); err != nil || !jobs.ValidApplicationStatus(input.Status) {
		writeErrorResponse(w, http.StatusBadRequest, "Choose Active, Rejected or Offer.")
		return
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter a valid status update date.")
		return
	}
	if err := a.db.UpdateJobApplicationStatus(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), input.Status, input.Date); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteJobApplication(w http.ResponseWriter, r *http.Request) {
	if err := a.db.DeleteJobApplication(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id")); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) saveJobStage(w http.ResponseWriter, r *http.Request) {
	var input jobs.StageInput
	if err := decodeJSON(r.Body, &input); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Enter valid stage details.")
		return
	}
	if err := input.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	v, err := a.db.SaveJobStage(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), r.PathValue("stageId"), input)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	status := http.StatusOK
	if r.PathValue("stageId") == "" {
		status = http.StatusCreated
	}
	writeJSONResponse(w, status, v)
}

func (a *API) deleteJobStage(w http.ResponseWriter, r *http.Request) {
	if err := a.db.DeleteJobStage(r.Context(), actorFrom(r.Context()).UserID, r.PathValue("id"), r.PathValue("stageId")); err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
