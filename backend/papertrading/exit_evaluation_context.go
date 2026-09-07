// Exit Evaluation Context — read-only Entry + Plan projection (Phase10-D.2.5).
//
// Attaches buy-time Item fields and Plan lifecycle status for re-assessment only.
// Does not recalculate strategy thesis, does not sell, does not write DB.

package papertrading

import (
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// ExitEntryContext is the buy-time thesis snapshot from TradePlanItem (stored fields only).
type ExitEntryContext struct {
	StrategyName string `json:"strategy_name"`
	EntryReason  string `json:"entry_reason"` // ← item.reason
	EntryRule    string `json:"entry_rule"`
	IntentStatus string `json:"intent_status"`
}

// ExitPlanContext is plan lifecycle identity for observation (not an order).
type ExitPlanContext struct {
	PlanID     uint   `json:"plan_id"`
	TradeDate  string `json:"trade_date"`
	PlanStatus string `json:"plan_status"`
}

// ExitContext nests entry + plan under ExitEvaluationLotRow.context.
type ExitContext struct {
	Entry ExitEntryContext `json:"entry"`
	Plan  ExitPlanContext  `json:"plan"`
}

// IsPlanLifecycleAbnormal reports statuses that warrant PLAN_REVIEW (re-check only).
// Examples: superseded / failed / invalidated. Not a sell signal.
func IsPlanLifecycleAbnormal(planStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(planStatus)) {
	case models.TradePlanStatusSuperseded,
		models.TradePlanStatusFailed,
		"invalidated":
		return true
	default:
		return false
	}
}

// BuildExitContextFromModels maps stored Item/Plan rows → ExitContext.
// Missing rows yield empty entry / partial plan (plan_id from lot still set by caller).
// Never invokes strategy engines.
func BuildExitContextFromModels(planID uint, item *models.TradePlanItem, plan *models.TradePlan) ExitContext {
	ctx := ExitContext{
		Plan: ExitPlanContext{PlanID: planID},
	}
	if item != nil {
		ctx.Entry = ExitEntryContext{
			StrategyName: strings.TrimSpace(item.StrategyName),
			EntryReason:  strings.TrimSpace(item.Reason),
			EntryRule:    strings.TrimSpace(item.EntryRule),
			IntentStatus: strings.TrimSpace(item.IntentStatus),
		}
		if ctx.Plan.PlanID == 0 && item.PlanID > 0 {
			ctx.Plan.PlanID = item.PlanID
		}
	}
	if plan != nil {
		if ctx.Plan.PlanID == 0 {
			ctx.Plan.PlanID = plan.ID
		}
		ctx.Plan.TradeDate = strings.TrimSpace(plan.TradeDate)
		ctx.Plan.PlanStatus = strings.TrimSpace(plan.Status)
	}
	return ctx
}

// LoadExitContextByFillID loads Item/Plan for lots in a Holding Observation view (read-only).
// Returns map keyed by fill_id. Safe with nil holding / missing IDs / nil DB (empty map).
func LoadExitContextByFillID(holding *HoldingEvalObservationView) map[uint]ExitContext {
	out := map[uint]ExitContext{}
	if holding == nil {
		return out
	}

	type lotRef struct {
		fillID     uint
		planID     uint
		planItemID uint
	}
	refs := make([]lotRef, 0)
	itemIDSet := map[uint]struct{}{}
	planIDSet := map[uint]struct{}{}
	for _, h := range holding.Holdings {
		for _, lot := range h.Lots {
			if lot.FillID == 0 {
				continue
			}
			refs = append(refs, lotRef{fillID: lot.FillID, planID: lot.PlanID, planItemID: lot.PlanItemID})
			if lot.PlanItemID > 0 {
				itemIDSet[lot.PlanItemID] = struct{}{}
			}
			if lot.PlanID > 0 {
				planIDSet[lot.PlanID] = struct{}{}
			}
		}
	}
	if len(refs) == 0 {
		return out
	}

	itemsByID := map[uint]models.TradePlanItem{}
	plansByID := map[uint]models.TradePlan{}
	if !dbDaoUnavailable() && db.Dao != nil {
		if len(itemIDSet) > 0 {
			ids := make([]uint, 0, len(itemIDSet))
			for id := range itemIDSet {
				ids = append(ids, id)
			}
			var items []models.TradePlanItem
			if err := db.Dao.Where("id IN ?", ids).Find(&items).Error; err == nil {
				for _, it := range items {
					itemsByID[it.ID] = it
				}
			}
		}
		if len(planIDSet) > 0 {
			ids := make([]uint, 0, len(planIDSet))
			for id := range planIDSet {
				ids = append(ids, id)
			}
			var plans []models.TradePlan
			if err := db.Dao.Where("id IN ?", ids).Find(&plans).Error; err == nil {
				for _, p := range plans {
					plansByID[p.ID] = p
				}
			}
		}
	}

	for _, ref := range refs {
		var itemPtr *models.TradePlanItem
		var planPtr *models.TradePlan
		if it, ok := itemsByID[ref.planItemID]; ok {
			cp := it
			itemPtr = &cp
		}
		if p, ok := plansByID[ref.planID]; ok {
			cp := p
			planPtr = &cp
		}
		out[ref.fillID] = BuildExitContextFromModels(ref.planID, itemPtr, planPtr)
	}
	return out
}
