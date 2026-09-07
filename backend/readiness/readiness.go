// Package readiness evaluates Execution Intent readiness before Approve discussion.
//
// Phase6.5.6.14.1: read-only evaluator. It does NOT Approve, Freeze, Execute,
// alter Strategy selection, rewrite Risk, or register Cron.
package readiness

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/sellgate"
	"go-stock/backend/tradingrule"
)

const (
	// LotSize moved to lot_threshold.go (legacy 100; Flag-aware helpers alongside).

	StageLegacy             = "LEGACY"
	StageIntentDraft        = "S1_INTENT_DRAFT"
	StageIntentMaterialized = "S2_INTENT_MATERIALIZED"
	StageIntentLocked       = "S3_INTENT_LOCKED"
	StageFrozenSnapshot     = "S4_FROZEN_SNAPSHOT"

	IntentSelected   = "selected"
	IntentPriced     = "priced"
	IntentGapSkip    = "gap_skip"
	IntentSizeSkip   = "size_skip"
	IntentManualSkip = "manual_skip"

	SeverityBlock = "BLOCK"
	SeverityWarn  = "WARN"

	RuleIRLimit  = "IR-LIMIT"
	RuleIRVolume = "IR-VOLUME"
	RuleIRPriced = "IR-PRICED-VOLUME"
	RuleIREmpty  = "IR-EMPTY"
	RuleIRLegacy = "IR-LEGACY"
	RuleIRStage  = "IR-STAGE"

	CodeLimitMissing          = "LIMIT_PRICE_MISSING"
	CodeVolumeBelowLot        = "TARGET_VOLUME_BELOW_LOT"
	CodePricedNoVolume        = "PRICED_VOLUME_NOT_MATERIALIZED"
	CodeNoTradeable           = "NO_TRADEABLE_ITEMS"
	CodeLegacyNoIntent        = "LEGACY_NO_INTENT"
	CodeIntentNotMaterialized = "INTENT_NOT_MATERIALIZED"

	RuleIRSell                = "IR-SELL"
	CodeSellNoPosition        = "SELL_NO_POSITION"
	CodeSellNotSellable       = "SELL_NOT_SELLABLE"
	CodeSellInsufficientAvail = "SELL_INSUFFICIENT_AVAILABLE"
	CodeSellInvalidQuantity   = "SELL_INVALID_QUANTITY"
)

// Finding is one readiness / quality finding line.
type Finding struct {
	RuleCode string         `json:"rule_code"`
	Code     string         `json:"code"`
	Severity string         `json:"severity"` // BLOCK | WARN
	Message  string         `json:"message"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

// Options configures injectable QualityGate inputs (read-only).
type Options struct {
	Positions  []qualitygate.AccountPosition
	MarketData qualitygate.MarketDataSnapshot
	Config     qualitygate.Config
	// SkipQualityGate disables embedding QG (tests / diagnostics). Default false.
	SkipQualityGate bool
}

// ExecutionIntentReadinessResult is the 14.1 read-only readiness report.
type ExecutionIntentReadinessResult struct {
	PlanID          uint      `json:"planId"`
	Ready           bool      `json:"ready"` // true iff len(Blockers)==0 (WARN does not clear Ready)
	LifecycleStage  string    `json:"lifecycle_stage"`
	Materialized    bool      `json:"materialized"`
	Blockers        []Finding `json:"blockers"`
	Warnings        []Finding `json:"warnings"`
	TradeableCount  int       `json:"tradeableCount"`
	SkipCount       int       `json:"skipCount"`
	CheckedAt       time.Time `json:"checkedAt"`
	QualitySeverity string    `json:"qualitySeverity,omitempty"`
}

// EvaluateExecutionIntentReadiness inspects a TradePlan for Approve-time readiness.
// It never mutates the plan and does not call Approve / Freeze / Execution.
func EvaluateExecutionIntentReadiness(plan *models.TradePlan, opts *Options) ExecutionIntentReadinessResult {
	now := time.Now()
	out := ExecutionIntentReadinessResult{
		CheckedAt:      now,
		Blockers:       make([]Finding, 0),
		Warnings:       make([]Finding, 0),
		LifecycleStage: StageLegacy,
	}
	if opts == nil {
		opts = &Options{}
	}
	if plan == nil {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: "IR-META",
			Code:     "PLAN_NIL",
			Severity: SeverityBlock,
			Message:  "trade plan is nil",
		})
		return out
	}
	out.PlanID = plan.ID
	out.LifecycleStage = inferLifecycleStage(plan)

	if plan.PricingPolicyVersion < 1 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIRLegacy,
			Code:     CodeLegacyNoIntent,
			Severity: SeverityBlock,
			Message:  "pricing_policy_version < 1: legacy plan has no Execution Intent contract",
		})
		out.Ready = false
		return out
	}

	if sellgate.IsPureSellPlan(plan) {
		return evaluatePureSellReadiness(plan, opts, out)
	}

	missingLimit := make([]string, 0)
	missingVolume := make([]string, 0)
	pricedNoVol := make([]string, 0)
	stillSelected := make([]string, 0)
	tradeable := 0
	skips := 0
	materializedOK := true

	for _, it := range plan.Items {
		if !isBuy(it.Side) {
			continue
		}
		st := strings.TrimSpace(it.IntentStatus)
		if isIntentSkip(st) {
			skips++
			continue
		}

		full := st == IntentPriced && it.LimitPrice > 0 && VolumeMeetsBuyLot(it.StockCode, it.TargetVolume)
		if full {
			tradeable++
			continue
		}
		materializedOK = false

		if st == IntentSelected || st == "" {
			stillSelected = append(stillSelected, it.StockCode)
		}
		if it.LimitPrice <= 0 {
			missingLimit = append(missingLimit, it.StockCode)
		}
		if !VolumeMeetsBuyLot(it.StockCode, it.TargetVolume) {
			missingVolume = append(missingVolume, it.StockCode)
		}
		if st == IntentPriced && !VolumeMeetsBuyLot(it.StockCode, it.TargetVolume) {
			pricedNoVol = append(pricedNoVol, it.StockCode)
		}
	}

	out.TradeableCount = tradeable
	out.SkipCount = skips
	out.Materialized = materializedOK && len(stillSelected) == 0 &&
		(tradeable > 0 || (skips > 0 && len(missingLimit) == 0 && len(missingVolume) == 0 && len(pricedNoVol) == 0))

	// Refine materialized: all non-skip buys are fully Spec'd.
	if len(stillSelected) > 0 || len(missingLimit) > 0 || len(missingVolume) > 0 || len(pricedNoVol) > 0 {
		out.Materialized = false
	} else if tradeable > 0 {
		out.Materialized = true
	} else if skips > 0 {
		// Morning finished as all-skip; Spec path "complete" but not approvable.
		out.Materialized = true
	} else {
		out.Materialized = false
	}

	if len(stillSelected) > 0 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIRStage,
			Code:     CodeIntentNotMaterialized,
			Severity: SeverityBlock,
			Message:  fmt.Sprintf("buy items still selected (not materialized): %s", strings.Join(stillSelected, ",")),
			Evidence: map[string]any{"codes": stillSelected},
		})
	}
	if len(missingLimit) > 0 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIRLimit,
			Code:     CodeLimitMissing,
			Severity: SeverityBlock,
			Message:  fmt.Sprintf("non-skip buy items missing limit_price: %s", strings.Join(missingLimit, ",")),
			Evidence: map[string]any{"codes": missingLimit},
		})
	}
	if len(pricedNoVol) > 0 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIRPriced,
			Code:     CodePricedNoVolume,
			Severity: SeverityBlock,
			Message:  fmt.Sprintf("priced items without volume materialization: %s", strings.Join(pricedNoVol, ",")),
			Evidence: map[string]any{"codes": pricedNoVol},
		})
	}
	// Non-skip volume below lot (includes selected and half-priced); avoid dup-only noise when already listed as pricedNoVol-only set.
	if len(missingVolume) > 0 {
		// Still emit when there are non-priced volume gaps, or always for rule 2 coverage.
		// Evidence keeps lot_size for compatibility; Flag ON adds policy mins.
		ev := map[string]any{
			"codes":           missingVolume,
			"lot_size":        LotSize, // legacy compat field
			"quantity_policy": tradingrule.EnableQuantityPolicy(),
		}
		if tradingrule.EnableQuantityPolicy() {
			mins := make(map[string]int64, len(missingVolume))
			for _, code := range missingVolume {
				mins[code] = EffectiveBuyLotThreshold(code)
			}
			ev["min_buy_qty_by_code"] = mins
		}
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIRVolume,
			Code:     CodeVolumeBelowLot,
			Severity: SeverityBlock,
			Message:  fmt.Sprintf("non-skip buy items target_volume below buy-lot gate: %s", strings.Join(missingVolume, ",")),
			Evidence: ev,
		})
	}
	if tradeable == 0 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIREmpty,
			Code:     CodeNoTradeable,
			Severity: SeverityBlock,
			Message:  "no tradeable items with full Order Spec (priced + limit>0 + volume>=lot)",
			Evidence: map[string]any{"skip_count": skips},
		})
	}

	if !opts.SkipQualityGate {
		qg := evaluateQualityGateSkipAware(plan, opts)
		out.QualitySeverity = qg.Severity
		for _, f := range qg.Blockers {
			out.Blockers = append(out.Blockers, Finding{
				RuleCode: f.RuleCode,
				Code:     f.Code,
				Severity: SeverityBlock,
				Message:  f.Message,
				Evidence: f.Evidence,
			})
		}
		for _, f := range qg.Warnings {
			out.Warnings = append(out.Warnings, Finding{
				RuleCode: f.RuleCode,
				Code:     f.Code,
				Severity: SeverityWarn,
				Message:  f.Message,
				Evidence: f.Evidence,
			})
		}
	}

	out.Ready = len(out.Blockers) == 0
	return out
}

func inferLifecycleStage(plan *models.TradePlan) string {
	if plan == nil {
		return StageLegacy
	}
	if plan.IsFrozen() {
		return StageFrozenSnapshot
	}
	if plan.PricingStage == "approve_locked" || plan.ApprovedAt != nil {
		return StageIntentLocked
	}
	if plan.PricingPolicyVersion < 1 {
		return StageLegacy
	}
	if plan.PricingStage == "morning_materialized" {
		return StageIntentMaterialized
	}
	return StageIntentDraft
}

func isIntentSkip(status string) bool {
	switch strings.TrimSpace(status) {
	case IntentGapSkip, IntentSizeSkip, IntentManualSkip:
		return true
	default:
		return false
	}
}

func isBuy(side string) bool {
	s := strings.ToLower(strings.TrimSpace(side))
	return s == "" || s == "buy"
}

func isSell(side string) bool {
	return strings.EqualFold(strings.TrimSpace(side), "sell")
}

func evaluatePureSellReadiness(plan *models.TradePlan, opts *Options, out ExecutionIntentReadinessResult) ExecutionIntentReadinessResult {
	tradeable := 0
	for _, it := range plan.Items {
		if !isSell(it.Side) {
			out.Blockers = append(out.Blockers, Finding{
				RuleCode: RuleIRSell,
				Code:     "SELL_MIXED_PLAN",
				Severity: SeverityBlock,
				Message:  fmt.Sprintf("mixed buy/sell plan not supported: item %s side=%s", it.StockCode, it.Side),
			})
			continue
		}
		if it.TargetVolume <= 0 || !sellgate.VolumeMeetsSellLot(it.StockCode, it.TargetVolume) {
			out.Blockers = append(out.Blockers, Finding{
				RuleCode: RuleIRSell,
				Code:     CodeSellInvalidQuantity,
				Severity: SeverityBlock,
				Message:  fmt.Sprintf("sell item invalid quantity: %s vol=%d", it.StockCode, it.TargetVolume),
			})
			continue
		}
		_, err := sellgate.CheckSellPositionGate(it.StockCode, it.TargetVolume, plan.TradeDate, out.CheckedAt)
		if err != nil {
			code := CodeSellInvalidQuantity
			switch err {
			case sellgate.ErrSellNoPosition:
				code = CodeSellNoPosition
			case sellgate.ErrSellNotSellable:
				code = CodeSellNotSellable
			case sellgate.ErrSellInsufficientAvailable:
				code = CodeSellInsufficientAvail
			}
			out.Blockers = append(out.Blockers, Finding{
				RuleCode: RuleIRSell,
				Code:     code,
				Severity: SeverityBlock,
				Message:  fmt.Sprintf("sell gate failed for %s: %v", it.StockCode, err),
			})
			continue
		}
		tradeable++
	}
	out.TradeableCount = tradeable
	out.Materialized = tradeable > 0
	if tradeable == 0 {
		out.Blockers = append(out.Blockers, Finding{
			RuleCode: RuleIREmpty,
			Code:     CodeNoTradeable,
			Severity: SeverityBlock,
			Message:  "no tradeable sell items",
		})
	}
	// Pure sell plans skip buy QualityGate (I1 position-overlap would false-positive).
	out.Ready = len(out.Blockers) == 0
	return out
}

// evaluateQualityGateSkipAware runs MVP QualityGate on a plan projection that
// excludes gap_skip / size_skip / manual_skip items so they cannot trip E1.
// I1 only sees remaining (tradeable / pending) codes.
func evaluateQualityGateSkipAware(plan *models.TradePlan, opts *Options) qualitygate.Result {
	if sellgate.IsPureSellPlan(plan) {
		return qualitygate.Result{
			Passed:    true,
			Severity:  qualitygate.SeverityPASS,
			PlanID:    plan.ID,
			TradeDate: plan.TradeDate,
			CheckedAt: time.Now(),
		}
	}
	md := opts.MarketData
	// After morning materialization, open-gap check is usually N/A.
	if plan.PricingStage == "morning_materialized" && !md.SkipGapEval {
		md.SkipGapEval = true
	}
	filtered := *plan
	filtered.Items = make([]models.TradePlanItem, 0, len(plan.Items))
	for _, it := range plan.Items {
		if isIntentSkip(strings.TrimSpace(it.IntentStatus)) {
			continue
		}
		filtered.Items = append(filtered.Items, it)
	}
	return qualitygate.EvaluateTradePlan(&filtered, opts.Positions, md, opts.Config)
}
