package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type programmeEnrollmentRow struct {
	ID              string
	State           string
	AssignmentState string
	Revision        int64
}

func loadProgrammeSeason(ctx context.Context, tx pgx.Tx, seasonID string) (programme.Season, []programmeEnrollmentRow, error) {
	var season programme.Season
	if err := tx.QueryRow(ctx, `SELECT id,status::text,revision FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&season.ID, &season.Status, &season.Revision); err != nil {
		return programme.Season{}, nil, noRows(err)
	}

	rows, err := tx.Query(ctx, `SELECT id,state::text,assignment_state::text,revision FROM app.enrollments WHERE season_id=$1 AND deleted_at IS NULL ORDER BY id FOR UPDATE`, seasonID)
	if err != nil {
		return programme.Season{}, nil, err
	}
	defer rows.Close()

	items := make([]programmeEnrollmentRow, 0)
	for rows.Next() {
		var item programmeEnrollmentRow
		if err := rows.Scan(&item.ID, &item.State, &item.AssignmentState, &item.Revision); err != nil {
			return programme.Season{}, nil, err
		}
		season.Enrollments = append(season.Enrollments, programme.Enrollment{ID: item.ID, State: item.State, AssignmentState: item.AssignmentState, Revision: item.Revision})
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return programme.Season{}, nil, err
	}
	return season, items, nil
}

func loadProgrammeReopenSeason(ctx context.Context, tx pgx.Tx, seasonID, closeEventID string) (programme.Season, []programmeEnrollmentRow, error) {
	season, items, err := loadProgrammeSeason(ctx, tx, seasonID)
	if err != nil {
		return programme.Season{}, nil, err
	}
	for index := range season.Enrollments {
		var completedByCloseID *string
		var closeAssignmentState string
		if items[index].State == "completed" {
			var found string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(completed_by_close_id::text,''),COALESCE(close_assignment_state::text,'') FROM app.enrollments WHERE id=$1`, items[index].ID).Scan(&found, &closeAssignmentState); err != nil {
				return programme.Season{}, nil, err
			}
			if found == closeEventID {
				completedByCloseID = &found
			}
		}
		season.Enrollments[index].CompletedByCloseID = completedByCloseID
		season.Enrollments[index].CloseAssignmentState = closeAssignmentState
	}
	return season, items, nil
}
