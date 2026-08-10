package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

const (
	morningIntentStatusPriced   = "priced"
	morningIntentStatusGapSkip  = "gap_skip"
	morningPricingStage         = "morning_materialized"
	morningPricedBy             = "morning"
	morningEntryRuleLimitAtRef  = "LIMIT_AT_REF"
	morningEntryRuleManualLimit = "MANUAL_LIMIT"
)

// MorningOpenPriceFunc resolves open/auction reference price for a stock code.
// Tests inject mocks; production may wire realtime/auction later.
type MorningOpenPriceFunc func(stockCode string) (price float64, ok bool)

// morningOpenPriceFn is the package default provider (nil = all unavailable).
var morningOpenPriceFn MorningOpenPriceFunc

// MorningPriceMaterializeResult is the outcome of limit_price-only morning materialization.
type MorningPriceMaterializeResult struct {
	PlanID          uint   `json:"planId"`
	NoOp            bool   `json:"noOp"`
	NoOpReason      string `json:"noOpReason,omitempty"`
	PricingStage    string `json:"pricingStage,omitempty"`
	PricedCount     int    `json:"pricedCount"`
	GapSkipCount    int    `json:"gapSkipCount"`
	PendingOpenCount int   `json:"pendingOpenCount"`
	SkippedLegacy   int    `json:"skippedLegacy"`
}

// MaterializeMorningLimitPrices materializes AfterClose Intent → limit_price for a draft plan.
//
// Phase6.5.6.13.1 scope:
//   - draft only; frozen → NO-OP
//   - writes limit_price / open_ref / intent_status / priced_* / pricing_stage
//   - does NOT write target_volume
//   - does NOT call Approve / Freeze / Execution / Cron / Strategy selection
func MaterializeMorningLimitPrices(planID uint, openPriceFn MorningOpenPriceFunc) (*MorningPriceMaterializeResult, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	repo := data.NewTradePlanRepo()
	plan, err := repo.GetByID(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("trade plan %d not found", planID)
	}

	out := &MorningPriceMaterializeResult{PlanID: plan.ID, PricingStage: plan.PricingStage}

	if plan.IsFrozen() {
		out.NoOp = true
		out.NoOpReason = "frozen"
		return out, nil
	}
	if plan.Status != models.TradePlanStatusDraft {
		out.NoOp = true
		out.NoOpReason = "not_draft"
		return out, nil
	}
	if plan.PricingPolicyVersion < 1 {
		out.NoOp = true
		out.NoOpReason = "legacy_no_intent"
		return out, nil
	}

	resolve := openPriceFn
	if resolve == nil {
		resolve = morningOpenPriceFn
	}

	now := time.Now()
	changed := false
	for i := range plan.Items {
		it := &plan.Items[i]
		beforeVol := it.TargetVolume

		action := materializeMorningItemLimit(plan, it, resolve, now)
		// Hard rule 13.1: never write target_volume.
		it.TargetVolume = beforeVol

		switch action {
		case morningActionPriced:
			out.PricedCount++
			changed = true
		case morningActionGapSkip:
			out.GapSkipCount++
			changed = true
		case morningActionPendingOpen:
			out.PendingOpenCount++
		case morningActionLegacySkip:
			out.SkippedLegacy++
		}

		if action == morningActionPriced || action == morningActionGapSkip {
			if err := repo.UpdateItemMorningLimitPrice(it); err != nil {
				return out, err
			}
		}
	}

	if changed && out.PendingOpenCount == 0 {
		plan.PricingStage = morningPricingStage
		if err := repo.UpdatePlanPricingStage(plan.ID, morningPricingStage); err != nil {
			return out, err
		}
		out.PricingStage = morningPricingStage
	}

	logger.SugaredLogger.Infof(
		"MaterializeMorningLimitPrices planId=%d priced=%d gapSkip=%d pendingOpen=%d legacySkip=%d stage=%s",
		plan.ID, out.PricedCount, out.GapSkipCount, out.PendingOpenCount, out.SkippedLegacy, out.PricingStage,
	)
	return out, nil
}

const (
	morningActionNone        = ""
	morningActionPriced      = "priced"
	morningActionGapSkip     = "gap_skip"
	morningActionPendingOpen = "pending_open"
	morningActionLegacySkip  = "legacy_skip"
)

func materializeMorningItemLimit(
	plan *models.TradePlan,
	it *models.TradePlanItem,
	resolve MorningOpenPriceFunc,
	now time.Time,
) string {
	if it == nil {
		return morningActionNone
	}
	if strings.TrimSpace(it.IntentStatus) != afterCloseIntentStatusSelected {
		return morningActionLegacySkip
	}
	side := strings.ToLower(strings.TrimSpace(it.Side))
	if side != "" && side != "buy" {
		return morningActionLegacySkip
	}
	if it.RefPrice <= 0 {
		return morningActionLegacySkip
	}

	open := 0.0
	ok := false
	if resolve != nil {
		open, ok = resolve(it.StockCode)
	}
	if !ok || open <= 0 {
		return morningActionPendingOpen
	}

	slip := resolvedMorningSlippage(plan, it)
	gapPct := (open - it.RefPrice) / it.RefPrice
	it.OpenRefPrice = open
	pricedAt := now
	it.PricedAt = &pricedAt
	it.PricedBy = morningPricedBy

	if gapPct > slip {
		it.IntentStatus = morningIntentStatusGapSkip
		it.LimitPrice = 0
		return morningActionGapSkip
	}

	limit, lok := computeMorningLimitPrice(plan, it, slip)
	if !lok || limit <= 0 {
		// Cannot materialize; revert priced audit for this attempt.
		it.OpenRefPrice = open
		it.PricedAt = nil
		it.PricedBy = ""
		return morningActionPendingOpen
	}

	it.IntentStatus = morningIntentStatusPriced
	it.LimitPrice = limit
	return morningActionPriced
}

func resolvedMorningSlippage(plan *models.TradePlan, it *models.TradePlanItem) float64 {
	if it != nil && it.MaxSlippage != nil {
		return *it.MaxSlippage
	}
	if plan != nil && plan.DefaultMaxSlippage != nil {
		return *plan.DefaultMaxSlippage
	}
	return afterCloseDefaultMaxSlippage
}

func computeMorningLimitPrice(plan *models.TradePlan, it *models.TradePlanItem, slip float64) (float64, bool) {
	rule := strings.TrimSpace(it.EntryRule)
	if rule == "" && plan != nil {
		rule = strings.TrimSpace(plan.DefaultEntryRule)
	}
	if rule == "" {
		rule = afterCloseEntryRuleLimitRefPlusSlip
	}
	switch rule {
	case morningEntryRuleLimitAtRef:
		return it.RefPrice, it.RefPrice > 0
	case morningEntryRuleManualLimit:
		return it.LimitPrice, it.LimitPrice > 0
	case afterCloseEntryRuleLimitRefPlusSlip:
		return it.RefPrice * (1 + slip), it.RefPrice > 0
	default:
		// Unknown rule: treat as LIMIT_REF_PLUS_SLIP for forward compatibility.
		return it.RefPrice * (1 + slip), it.RefPrice > 0
	}
}
