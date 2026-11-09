package api

import (
	"fmt"
	"net/http"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/magedmg/RSP-website/backend/internal/platform/id"
	"github.com/magedmg/RSP-website/backend/internal/platform/sanitize"
	"github.com/magedmg/RSP-website/backend/internal/program"
)

// maxImportedWeeks bounds one season import.
const maxImportedWeeks = 104

type seasonImportWeek struct {
	Number        int       `json:"number"`
	StartAt       time.Time `json:"startAt"`
	EndAt         time.Time `json:"endAt"`
	ResourcesHTML string    `json:"resourcesHtml"`
}

type seasonImportRequest struct {
	seasonInput
	Weeks []seasonImportWeek `json:"weeks"`
}

// importSeason creates a season with all its weeks and their resources, as
// from a spreadsheet. The season runs from its first week's start to its last
// week's end; the weeks must be numbered once each and must not overlap.
func (a *API) importSeason(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r.Context())
	if !actor.IsDirectorOrSystemAdmin() || !actor.HasRecentMFA(time.Now()) {
		writeErrorResponse(w, http.StatusForbidden, "Director or System Admin access with recent MFA is required.")
		return
	}
	var request seasonImportRequest
	if err := decodeJSON(r.Body, &request); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	weeks, err := a.importedWeeks(request.Weeks)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	request.StartAt, request.EndAt = weeks[0].StartAt, weeks[0].EndAt
	for _, week := range weeks {
		if week.EndAt.After(request.EndAt) {
			request.EndAt = week.EndAt
		}
	}
	if err := validateSeason(request.seasonInput); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	season := program.SeasonRecord{
		ID:           id.New(),
		Slug:         request.Slug,
		Name:         request.Name,
		Status:       "open",
		StartAt:      request.StartAt,
		EndAt:        request.EndAt,
		Location:     request.Location,
		ImageURL:     request.ImageURL,
		ResourcesURL: request.ResourcesURL,
	}
	created, err := a.db.ImportSeason(r.Context(), season, weeks, actor.UserID, time.Now().UTC())
	if err != nil {
		a.writeStoreErrorResponse(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, created)
}

// importedWeeks checks and sanitises an import's weeks, sorted by start.
func (a *API) importedWeeks(input []seasonImportWeek) ([]program.WeekRecord, error) {
	if len(input) == 0 || len(input) > maxImportedWeeks {
		return nil, fmt.Errorf("a season needs between 1 and %d weeks", maxImportedWeeks)
	}
	numbers := map[int]bool{}
	weeks := make([]program.WeekRecord, 0, len(input))
	for _, week := range input {
		if week.Number < 1 {
			return nil, fmt.Errorf("week numbers must be at least 1")
		}
		if numbers[week.Number] {
			return nil, fmt.Errorf("week %d is listed more than once", week.Number)
		}
		numbers[week.Number] = true
		if week.StartAt.IsZero() || !week.EndAt.After(week.StartAt) {
			return nil, fmt.Errorf("week %d must end after it starts", week.Number)
		}
		content := a.sanitizeHTML(week.ResourcesHTML)
		if sanitize.IsBlank(content) {
			content = ""
		}
		if utf8.RuneCountInString(content) > maxWeekResourcesHTML {
			return nil, fmt.Errorf("week %d's resources are too long", week.Number)
		}
		weeks = append(weeks, program.WeekRecord{ID: id.New(), Number: week.Number, StartAt: week.StartAt.UTC(), EndAt: week.EndAt.UTC(), ResourcesHTML: content})
	}
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].StartAt.Before(weeks[j].StartAt) })
	for i := 1; i < len(weeks); i++ {
		if weeks[i].StartAt.Before(weeks[i-1].EndAt) {
			return nil, fmt.Errorf("weeks %d and %d overlap", weeks[i-1].Number, weeks[i].Number)
		}
	}
	return weeks, nil
}
