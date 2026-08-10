package data

import (
	"fmt"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// UpdateItemMorningLimitPrice persists morning Intent→limit_price materialization fields.
// It intentionally does not update target_volume (Phase6.5.6.13.1 scope).
func (r *TradePlanRepo) UpdateItemMorningLimitPrice(item *models.TradePlanItem) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if item == nil || item.ID == 0 {
		return fmt.Errorf("invalid trade plan item")
	}
	item.UpdatedAt = time.Now()
	return db.Dao.Model(&models.TradePlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"limit_price":    item.LimitPrice,
		"open_ref_price": item.OpenRefPrice,
		"intent_status":  item.IntentStatus,
		"priced_at":      item.PricedAt,
		"priced_by":      item.PricedBy,
		"updated_at":     item.UpdatedAt,
	}).Error
}

// UpdateItemMorningTargetVolume persists morning position materialization (Phase6.5.6.13.2.1).
// Updates target_volume + intent_status only; does not touch limit_price / risk_* / filled_*.
func (r *TradePlanRepo) UpdateItemMorningTargetVolume(item *models.TradePlanItem) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if item == nil || item.ID == 0 {
		return fmt.Errorf("invalid trade plan item")
	}
	item.UpdatedAt = time.Now()
	return db.Dao.Model(&models.TradePlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{
		"target_volume": item.TargetVolume,
		"intent_status": item.IntentStatus,
		"updated_at":    item.UpdatedAt,
	}).Error
}

// UpdatePlanPricingStage updates trade_plans.pricing_stage only.
func (r *TradePlanRepo) UpdatePlanPricingStage(planID uint, stage string) error {
	if db.Dao == nil {
		return fmt.Errorf("数据库未初始化")
	}
	if planID == 0 {
		return fmt.Errorf("plan id is required")
	}
	return db.Dao.Model(&models.TradePlan{}).Where("id = ?", planID).Updates(map[string]any{
		"pricing_stage": stage,
		"updated_at":    time.Now(),
	}).Error
}
