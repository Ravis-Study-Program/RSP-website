package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/magedmg/RSP-website/backend/internal/authz"
)

func activityFilters(r *http.Request, actor authz.Actor) (string, int, error) {
	seasonID := r.URL.Query().Get("seasonId")
	if seasonID != "" {
		var value pgtype.UUID
		if err := value.Scan(seasonID); err != nil {
			return "", 0, errors.New("seasonId must be a UUID")
		}
	}
	if seasonID != "" && !actor.IsDirectorOrSystemAdmin() {
		if _, ok := actor.Enrollment(seasonID); !ok {
			return "", 0, errors.New("the selected season is not one of your memberships")
		}
	}
	year := 0
	if value := r.URL.Query().Get("year"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1900 || parsed > 9999 {
			return "", 0, errors.New("year must be between 1900 and 9999")
		}
		year = parsed
	}
	return seasonID, year, nil
}
