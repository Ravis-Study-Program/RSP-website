package postgres

import (
	"context"

	"github.com/magedmg/RSP-website/backend/internal/programme"
	"gorm.io/gorm"
)

type programmeEnrollmentRow struct {
	ID              string
	State           string
	AssignmentState string
	Revision        int64
}

func loadProgrammeSeason(ctx context.Context, tx *gorm.DB, seasonID string) (programme.Season, []programmeEnrollmentRow, error) {
	var season programme.Season
	err := tx.WithContext(ctx).Raw(`SELECT id,status::text,revision
		FROM app.seasons WHERE id=? AND deleted_at IS NULL FOR UPDATE`, seasonID).
		Row().Scan(&season.ID, &season.Status, &season.Revision)
	if err != nil {
		return programme.Season{}, nil, noRows(err)
	}

	rows, err := tx.Raw(`SELECT id,state::text,assignment_state::text,revision
		FROM app.enrollments WHERE season_id=? AND deleted_at IS NULL ORDER BY id FOR UPDATE`, seasonID).Rows()
	if err != nil {
		return programme.Season{}, nil, err
	}
	defer rows.Close()

	items := []programmeEnrollmentRow{}
	for rows.Next() {
		var item programmeEnrollmentRow
		if err := rows.Scan(&item.ID, &item.State, &item.AssignmentState, &item.Revision); err != nil {
			return programme.Season{}, nil, err
		}
		season.Enrollments = append(season.Enrollments, programme.Enrollment{
			ID: item.ID, State: item.State, AssignmentState: item.AssignmentState, Revision: item.Revision,
		})
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return programme.Season{}, nil, err
	}
	return season, items, nil
}

func loadProgrammeReopenSeason(ctx context.Context, tx *gorm.DB, seasonID, closeEventID string) (programme.Season, []programmeEnrollmentRow, error) {
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
		err := tx.Raw(`SELECT completed_by_close_id,COALESCE(close_assignment_state::text,'')
			FROM app.enrollments WHERE id=?`, items[index].ID).
			Row().Scan(&completedByCloseID, &closeAssignmentState)
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
