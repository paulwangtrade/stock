package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// ApproveDraftGate CAS-writes approved_at / approved_by / approved_source for a first-time
// draft approval (approved_at IS NULL). Keeps status=draft; does not touch items / Intent /
// limit_price / target_volume / freeze_* / enable_execute.
// Returns false when CAS misses (not draft, already approved, or concurrent race).
func (r *TradePlanRepo) ApproveDraftGate(planID uint, approvedBy, approvedSource string, at time.Time) (bool, error) {
	if db.Dao == nil {
		return false, fmt.Errorf("数据库未初始化")
	}
	if planID == 0 {
		return false, fmt.Errorf("plan id is required")
	}
	if at.IsZero() {
		at = time.Now()
	}
	approvedBy = strings.TrimSpace(approvedBy)
	approvedSource = strings.TrimSpace(approvedSource)
	res := db.Dao.Model(&models.TradePlan{}).
		Where("id = ? AND status = ? AND approved_at IS NULL", planID, models.TradePlanStatusDraft).
		Updates(map[string]any{
			"approved_at":     at,
			"approved_by":     approvedBy,
			"approved_source": approvedSource,
			"updated_at":      at,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}
