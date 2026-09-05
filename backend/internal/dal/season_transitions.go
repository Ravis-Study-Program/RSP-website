package dal

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/magedmg/RSP-website/backend/internal/programme"
)

type programmeEnrollmentRow struct {
	ID              string
	State           string
	AssignmentState string
}

func loadProgrammeSeason(ctx context.Context, tx pgx.Tx, seasonID string) (programme.Season, []programmeEnrollmentRow, error) {
	var season programme.Season
	err := tx.QueryRow(ctx, `SELECT id,status::text
		FROM app.seasons WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, seasonID).Scan(&season.ID, &season.Status)
	if err != nil {
		return programme.Season{}, nil, noRows(err)
	}

	rows, err := tx.Query(ctx, `SELECT id,state::text,assignment_state::text
		FROM app.enrollments WHERE season_id=$1 AND deleted_at IS NULL ORDER BY id FOR UPDATE`, seasonID)
	if err != nil {
		return programme.Season{}, nil, err
	}
	defer rows.Close()

	items := []programmeEnrollmentRow{}
	for rows.Next() {
		var item programmeEnrollmentRow
		if err := rows.Scan(&item.ID, &item.State, &item.AssignmentState); err != nil {
			return programme.Season{}, nil, err
		}
		season.Enrollments = append(season.Enrollments, programme.Enrollment{
			ID: item.ID, State: item.State, AssignmentState: item.AssignmentState,
		})
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
		if items[index].State != "completed" {
			continue
		}
		var completedByCloseID *string
		var closeAssignmentState string
		err := tx.QueryRow(ctx, `SELECT completed_by_close_id,COALESCE(close_assignment_state::text,'')
			FROM app.enrollments WHERE id=$1`, items[index].ID).Scan(&completedByCloseID, &closeAssignmentState)
		if err != nil {
			return programme.Season{}, nil, err
		}
		if completedByCloseID == nil || *completedByCloseID != closeEventID {
			completedByCloseID = nil
		}
		season.Enrollments[index].CompletedByCloseID = completedByCloseID
		season.Enrollments[index].CloseAssignmentState = closeAssignmentState
	}
	return season, items, nil
}
