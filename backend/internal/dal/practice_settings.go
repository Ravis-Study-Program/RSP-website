package dal

import (
	"context"

	"github.com/magedmg/RSP-website/backend/internal/practice"
)

func (p *Store) GetPracticeSettings(ctx context.Context, userID string) (practice.PracticeSettings, error) {
	// Personal settings are historical data only. All members now use the
	// programme thresholds, regardless of whether goals were previously enabled.
	var storedID string
	if err := p.pool.QueryRow(ctx, `SELECT id FROM app.users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&storedID); err != nil {
		return practice.PracticeSettings{}, noRows(err)
	}
	return practice.FixedSettings(), nil
}
