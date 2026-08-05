package strategy

import (
	"encoding/json"
	"strings"

	"go-stock/backend/marketdata/anchor"
	"go-stock/backend/models"
)

// AfterClose Execution Intent defaults (PHASE6_5_6_11 / 6.5.6.12).
const (
	afterCloseEntryRuleLimitRefPlusSlip = "LIMIT_REF_PLUS_SLIP"
	afterClosePricingStage              = "after_close_intent"
	afterCloseIntentStatusSelected      = "selected"
	afterClosePricingPolicyVersion      = 1
	afterCloseRefSourcePrevClose        = anchor.RefSourcePrevClose
	afterCloseRefSourceStrategySnap     = anchor.RefSourceStrategySnapshot
)

var afterCloseDefaultMaxSlippage = 0.03

// afterCloseAnchorProvider resolves ref_price for AfterClose Intent populate (Phase10-B.0).
// Tests may override; production default is Followed-only (no fabricated prices).
var afterCloseAnchorProvider anchor.Provider = anchor.DefaultProvider()

// populateAfterCloseExecutionIntent fills Plan/Item Intent fields for AfterClose draft.
// It never materializes limit_price / target_volume (forced to 0).
// Does not alter Risk fields, filter results, or item count.
func populateAfterCloseExecutionIntent(plan *models.TradePlan, items []models.TradePlanItem, pool *models.CandidatePool) {
	if plan == nil {
		return
	}
	slip := afterCloseDefaultMaxSlippage
	plan.DefaultEntryRule = afterCloseEntryRuleLimitRefPlusSlip
	plan.DefaultMaxSlippage = &slip
	plan.PricingPolicyVersion = afterClosePricingPolicyVersion
	plan.PricingStage = afterClosePricingStage

	tradeDate := strings.TrimSpace(plan.TradeDate)
	provider := afterCloseAnchorProvider
	if provider == nil {
		provider = anchor.DefaultProvider()
	}
	baseCtx := anchorContextFromPool(pool, tradeDate)

	for i := range items {
		// Order Spec must stay unmaterialized at AfterClose.
		items[i].LimitPrice = 0
		items[i].TargetVolume = 0
		items[i].OpenRefPrice = 0
		items[i].PricedAt = nil
		items[i].PricedBy = ""

		if !isAfterCloseIntentEligibleItem(items[i]) {
			items[i].IntentStatus = ""
			items[i].EntryRule = ""
			items[i].MaxSlippage = nil
			items[i].RefPrice = 0
			items[i].RefSource = ""
			items[i].RefAsOf = ""
			continue
		}

		ctx := baseCtx
		ctx.StockCode = items[i].StockCode
		res, ok := provider.Resolve(ctx)
		if !ok || !anchor.Valid(res) {
			// Soft-fail (design §7.4 A): keep list row, do not claim selected Intent.
			items[i].IntentStatus = ""
			items[i].EntryRule = ""
			items[i].MaxSlippage = nil
			items[i].RefPrice = 0
			items[i].RefSource = ""
			items[i].RefAsOf = ""
			continue
		}

		itemSlip := afterCloseDefaultMaxSlippage
		items[i].RefPrice = res.RefPrice
		items[i].RefSource = res.RefSource
		items[i].RefAsOf = res.RefAsOf
		if items[i].RefAsOf == "" {
			items[i].RefAsOf = tradeDate
		}
		items[i].EntryRule = afterCloseEntryRuleLimitRefPlusSlip
		items[i].MaxSlippage = &itemSlip
		items[i].IntentStatus = afterCloseIntentStatusSelected
	}
}

func anchorContextFromPool(pool *models.CandidatePool, tradeDate string) anchor.Context {
	ctx := anchor.Context{TradeDate: tradeDate}
	if pool == nil {
		return ctx
	}
	ctx.PoolID = pool.ID
	ctx.PoolSource = pool.Source
	if cfg := parsePoolConfigJSON(pool.ConfigJSON); cfg != nil {
		if v, ok := cfg["source_date"].(string); ok {
			ctx.SourceDate = strings.TrimSpace(v)
		}
		if v, ok := cfg["session"].(string); ok {
			ctx.Session = strings.TrimSpace(v)
		}
	}
	if ctx.Session == "" {
		ctx.Session = models.TradePlanSourceAfterClose
	}
	return ctx
}

func parsePoolConfigJSON(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

func isAfterCloseIntentEligibleItem(it models.TradePlanItem) bool {
	side := strings.ToLower(strings.TrimSpace(it.Side))
	if side != "" && side != "buy" {
		return false
	}
	switch it.Status {
	case models.TradePlanItemSkipped, models.TradePlanItemError:
		return false
	default:
		return true
	}
}
