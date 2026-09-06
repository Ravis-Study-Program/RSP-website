package api

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/magedmg/RSP-website/backend/internal/accounts"
	"github.com/magedmg/RSP-website/backend/internal/authz"
	"github.com/magedmg/RSP-website/backend/internal/dal"
)

type currentUserSeasonRole struct {
	SeasonID   string `json:"seasonId"`
	SeasonSlug string `json:"seasonSlug"`
	Role       string `json:"role"`
	State      string `json:"state"`
}

type currentUserResponse struct {
	accounts.User
	EmailVerified bool                    `json:"emailVerified"`
	MFAVerified   bool                    `json:"mfaVerified"`
	SeasonRoles   []currentUserSeasonRole `json:"seasonRoles"`
	Alumni        bool                    `json:"alumni"`
}

func (a *API) loadCurrentUserSeasonRoles(ctx context.Context, enrollments []authz.Enrollment) ([]currentUserSeasonRole, error) {
	roles := make([]currentUserSeasonRole, 0, len(enrollments))
	for _, enrollment := range enrollments {
		if enrollment.State != authz.Active && enrollment.State != authz.Completed {
			continue
		}

		season, err := a.db.GetSeason(ctx, enrollment.SeasonID)
		if err != nil {
			return nil, err
		}

		roles = append(roles, currentUserSeasonRole{
			SeasonID:   enrollment.SeasonID,
			SeasonSlug: season.Slug,
			Role:       string(enrollment.Role),
			State:      string(enrollment.State),
		})
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].SeasonSlug < roles[j].SeasonSlug })
	return roles, nil
}

func buildCurrentUserResponse(actor authz.Actor, user accounts.User, roles []currentUserSeasonRole, now time.Time) currentUserResponse {
	return currentUserResponse{
		User:          user,
		EmailVerified: actor.EmailVerified,
		MFAVerified:   actor.HasRecentMFA(now),
		SeasonRoles:   roles,
		Alumni:        actor.IsStudentAlumnus(),
	}
}

func (a *API) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	user, err := a.db.GetUser(r.Context(), actor.UserID)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	roles, err := a.loadCurrentUserSeasonRoles(r.Context(), actor.Enrollments)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	response := buildCurrentUserResponse(actor, user, roles, time.Now())
	writeJSONResponse(w, http.StatusOK, response)
}

func (a *API) suggestCurrentUserSlug(w http.ResponseWriter, r *http.Request) {
	slug, err := a.db.SuggestUserSlug(r.Context())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]string{"slug": slug})
}

type updateCurrentUserRequest struct {
	Name      string  `json:"name"`
	Slug      *string `json:"slug"`
	AvatarURL *string `json:"avatarUrl"`
	Timezone  string  `json:"timezone"`
}

func (a *API) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	var request updateCurrentUserRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	if _, err := time.LoadLocation(request.Timezone); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "timezone must be an IANA timezone")
		return
	}
	if request.AvatarURL != nil && *request.AvatarURL != "" && !isValidHTTPSURL(*request.AvatarURL) {
		writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "avatarUrl must be an HTTPS URL")
		return
	}

	if request.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*request.Slug))
		if len(slug) < 3 || len(slug) > 50 || !validSlug(slug) {
			writeErrorResponse(w, http.StatusBadRequest, "validation_failed", "slug must be 3-50 lowercase letters, numbers, or hyphens")
			return
		}
		request.Slug = &slug
	}

	updated, err := a.db.UpdateUserProfile(r.Context(), dal.UpdateUserProfileInput{
		UserID:    actor.UserID,
		Name:      strings.TrimSpace(request.Name),
		Slug:      request.Slug,
		AvatarURL: request.AvatarURL,
		Timezone:  request.Timezone,
		ActorID:   actor.UserID,
		ChangedAt: time.Now().UTC(),
	})
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	roles, err := a.loadCurrentUserSeasonRoles(r.Context(), actor.Enrollments)
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}

	response := buildCurrentUserResponse(actor, updated, roles, time.Now())
	writeJSONResponse(w, http.StatusOK, response)
}

func validSlug(slug string) bool {
	if slug[0] == '-' || slug[len(slug)-1] == '-' {
		return false
	}
	for _, ch := range slug {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}
