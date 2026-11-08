package dal

import (
	"context"

	"github.com/magedmg/RSP-website/backend/internal/jobs"
)

const jobReferralColumns = `r.id,c.id,c.name,c.category,u.id,p.slug,p.display_name,r.notes`
const jobReferralJoin = `app.job_referrals r JOIN app.job_companies c ON c.id=r.company_id
 JOIN app.users u ON u.id=r.user_id JOIN app.user_profiles p ON p.user_id=u.id`
const visibleReferrer = `u.account_state='active' AND u.deleted_at IS NULL`

func referralTargets(v *jobs.Referral) []any {
	return []any{&v.ID, &v.Company.ID, &v.Company.Name, &v.Company.Category, &v.Referrer.ID, &v.Referrer.Slug, &v.Referrer.Name, &v.Notes}
}

func (p *Store) ListJobReferrals(ctx context.Context, boundary string, limit int) ([]jobs.Referral, bool, int64, error) {
	var total int64
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM `+jobReferralJoin+` WHERE `+visibleReferrer).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	rows, err := p.pool.Query(ctx, `SELECT `+jobReferralColumns+` FROM `+jobReferralJoin+`
 WHERE `+visibleReferrer+` AND ($1='' OR r.id>NULLIF($1,'')::uuid) ORDER BY r.id LIMIT $2`, boundary, limit+1)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	out := []jobs.Referral{}
	for rows.Next() {
		var v jobs.Referral
		if err := rows.Scan(referralTargets(&v)...); err != nil {
			return nil, false, 0, err
		}
		out = append(out, v)
	}
	out, more := finishPage(out, limit, "forward")
	return out, more, total, rows.Err()
}

func (p *Store) SaveJobReferral(ctx context.Context, userID, referralID string, input jobs.ReferralInput) (jobs.Referral, error) {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return jobs.Referral{}, err
	}
	defer tx.Rollback(context.Background())
	if referralID != "" {
		var found string
		if err := tx.QueryRow(ctx, `SELECT id FROM app.job_referrals WHERE id=$1 AND user_id=$2`, referralID, userID).Scan(&found); err != nil {
			return jobs.Referral{}, noRows(err)
		}
	}
	companyID, err := jobCompanyID(ctx, tx, input.CompanyName, "other")
	if err != nil {
		return jobs.Referral{}, err
	}
	if referralID == "" {
		err = tx.QueryRow(ctx, `INSERT INTO app.job_referrals(user_id,company_id,notes) VALUES($1,$2,$3) RETURNING id`, userID, companyID, input.Notes).Scan(&referralID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE app.job_referrals SET company_id=$1,notes=$2,updated_at=now() WHERE id=$3 AND user_id=$4`, companyID, input.Notes, referralID, userID)
	}
	if err != nil {
		return jobs.Referral{}, mapDatabaseError(err)
	}
	var v jobs.Referral
	if err = tx.QueryRow(ctx, `SELECT `+jobReferralColumns+` FROM `+jobReferralJoin+` WHERE r.id=$1`, referralID).Scan(referralTargets(&v)...); err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

func (p *Store) DeleteJobReferral(ctx context.Context, userID, referralID string) error {
	tx, err := p.beginJobs(ctx, userID)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	var found string
	if err = tx.QueryRow(ctx, `DELETE FROM app.job_referrals WHERE id=$1 AND user_id=$2 RETURNING id`, referralID, userID).Scan(&found); err != nil {
		return noRows(err)
	}
	return tx.Commit(ctx)
}
