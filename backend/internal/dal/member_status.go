package dal

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/magedmg/RSP-website/backend/internal/accounts"
)

func (p *Store) GetMemberStatus(ctx context.Context, userID string) (accounts.MemberStatus, error) {
	var row struct {
		ID, AccountState                   string
		ActiveMember, Alumni, FormerMember bool
		HasEnrollment, HasKicked           bool
	}
	err := p.pool.QueryRow(ctx, `SELECT u.id,u.account_state::text AS account_state,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='active' AND e.deleted_at IS NULL) AS active_member,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.role='student' AND e.state='completed' AND e.deleted_at IS NULL) AS alumni,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='completed' AND e.deleted_at IS NULL) AS former_member,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.deleted_at IS NULL) AS has_enrollment,
		EXISTS(SELECT 1 FROM app.enrollments e WHERE e.user_id=u.id AND e.state='kicked' AND e.deleted_at IS NULL) AS has_kicked
		FROM app.users u WHERE u.id=$1`, userID).Scan(&row.ID, &row.AccountState, &row.ActiveMember, &row.Alumni, &row.FormerMember, &row.HasEnrollment, &row.HasKicked)
	if err != nil || row.ID == "" {
		if err == nil {
			err = pgx.ErrNoRows
		}
		return accounts.MemberStatus{}, noRows(err)
	}

	ptn := accounts.MemberStatus{
		UserID:       row.ID,
		ActiveMember: row.ActiveMember,
		Alumni:       row.Alumni,
		FormerMember: row.FormerMember,
		Inactive:     row.AccountState != "active",
		Suspended:    row.AccountState == "suspended",
		Deleted:      row.AccountState == "deleted",
	}
	ptn.KickedOnly = row.HasEnrollment && row.HasKicked && !ptn.ActiveMember && !ptn.FormerMember
	return ptn, nil
}
