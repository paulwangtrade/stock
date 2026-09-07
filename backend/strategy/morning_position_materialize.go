package strategy

import (
	"fmt"
	"math"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/instrument"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/tradingconfig"
	"go-stock/backend/tradingrule"
)

const (
	morningIntentStatusSizeSkip = "size_skip"
	morningLotSize              = int64(100)

	morningBindBudget           = "budget"
	morningBindCash             = "cash"
	morningBindSingleName       = "single_name"
	morningBindGross            = "gross"
	morningBindPositionConflict = "position_conflict"
	morningBindLot              = "lot"
	morningBindNoBudget         = "no_budget"
	morningBindIdempotent       = "idempotent"
)

// MorningAccountSnapshot is a read-only cash/position view for position materialization.
// Tests inject mocks; production loads from portfolio.Snapshot (paper_sim_*).
type MorningAccountSnapshot struct {
	Cash             float64
	Equity           float64
	LongMarketValue  float64
	ShortMarketValue float64
	NameMarketValue  map[string]float64 // code → MV
	PositionVolumes  map[string]int64   // code → shares (conflict check)
}

// MorningRiskLimits is a read-only exposure cap view (does not mutate Risk config).
type MorningRiskLimits struct {
	MaxGrossExposurePct float64
	MaxSingleNamePct    float64
}

// MorningPositionMaterializeOpts configures injectable providers for 13.2.1.
type MorningPositionMaterializeOpts struct {
	Snapshot *MorningAccountSnapshot // nil → default loader
	Limits   *MorningRiskLimits      // nil → default from tradingconfig.Default().Risk() (read-only)
	Force    bool                    // true → recompute even if target_volume>=100
}

// MorningPositionMaterializeResult is the outcome of limit_price → target_volume materialization.
type MorningPositionMaterializeResult struct {
	PlanID         uint   `json:"planId"`
	NoOp           bool   `json:"noOp"`
	NoOpReason     string `json:"noOpReason,omitempty"`
	PricingStage   string `json:"pricingStage,omitempty"`
	SizedCount     int    `json:"sizedCount"`
	SizeSkipCount  int    `json:"sizeSkipCount"`
	IdempotentSkip int    `json:"idempotentSkip"`
	SkippedOther   int    `json:"skippedOther"`
	// QuantityShadows: Phase12-M2.2.5 read-only Policy vs legacy compare (does not affect target_volume).
	QuantityShadows []QuantityShadowObservation `json:"quantityShadows,omitempty"`
}

// MaterializeMorningTargetVolumes materializes priced Intent → target_volume for a draft plan.
//
// Phase6.5.6.13.2.1 scope:
//   - draft + Intent policy only; frozen / legacy → NO-OP
//   - only items with intent_status=priced and limit_price>0
//   - volume = lot(min(budget, cash, single_name, gross) / limit_price); NOT cash/price alone
//   - writes target_volume (+ intent_status size_skip when unreachable)
//   - does NOT call Execution / Prepare / Buy / Freeze / Approve / Risk rewrite
func MaterializeMorningTargetVolumes(planID uint, opts *MorningPositionMaterializeOpts) (*MorningPositionMaterializeResult, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	if opts == nil {
		opts = &MorningPositionMaterializeOpts{}
	}

	repo := data.NewTradePlanRepo()
	plan, err := repo.GetByID(planID)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("trade plan %d not found", planID)
	}

	out := &MorningPositionMaterializeResult{PlanID: plan.ID, PricingStage: plan.PricingStage}

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

	snap := opts.Snapshot
	if snap == nil {
		snap = loadMorningAccountSnapshotDefault()
	}
	limits := opts.Limits
	if limits == nil {
		l := loadMorningRiskLimitsDefault()
		limits = &l
	}
	normalizeMorningRiskLimits(limits)

	remainingCash := snap.Cash
	committedExtra := 0.0
	baseGross := snap.LongMarketValue + snap.ShortMarketValue
	equity := snap.Equity
	if equity <= 0 {
		equity = snap.Cash + snap.LongMarketValue
	}
	if equity <= 0 {
		equity = 1 // avoid div-by-zero; headrooms collapse via caps
	}

	changed := false
	for i := range plan.Items {
		it := &plan.Items[i]
		beforeLimit := it.LimitPrice
		beforeAmount := it.TargetAmount
		beforeRiskCode := it.RiskCode
		beforeRiskMsg := it.RiskMessage

		action, binding, shadow := materializeMorningItemVolume(plan, it, snap, limits, remainingCash, committedExtra, baseGross, equity, opts.Force)
		if shadow != nil {
			out.QuantityShadows = append(out.QuantityShadows, *shadow)
		}

		// Hard rules: never mutate price/budget/risk audit via this path.
		it.LimitPrice = beforeLimit
		it.TargetAmount = beforeAmount
		it.RiskCode = beforeRiskCode
		it.RiskMessage = beforeRiskMsg

		switch action {
		case morningPosActionSized:
			out.SizedCount++
			changed = true
			notional := float64(it.TargetVolume) * it.LimitPrice
			remainingCash -= notional
			committedExtra += notional
		case morningPosActionSizeSkip:
			out.SizeSkipCount++
			changed = true
		case morningPosActionIdempotent:
			out.IdempotentSkip++
			// Still reserve notional so subsequent items see committed exposure/cash.
			notional := float64(it.TargetVolume) * it.LimitPrice
			remainingCash -= notional
			committedExtra += notional
		default:
			out.SkippedOther++
		}

		if action == morningPosActionSized || action == morningPosActionSizeSkip {
			if err := repo.UpdateItemMorningTargetVolume(it); err != nil {
				return out, err
			}
			logger.SugaredLogger.Infof(
				"MaterializeMorningTargetVolumes item planId=%d code=%s action=%s binding=%s volume=%d limit=%.4f",
				plan.ID, it.StockCode, action, binding, it.TargetVolume, it.LimitPrice,
			)
		}
	}

	if changed && plan.PricingStage != morningPricingStage {
		// Confirm morning_materialized only when already past price stage naming; do not
		// invent a new stage enum in MVP (design option A).
		if plan.PricingStage == afterClosePricingStage || plan.PricingStage == "" {
			// Price materialization should have set stage; if volume-only path runs after
			// priced items exist, still promote to morning_materialized.
			if err := repo.UpdatePlanPricingStage(plan.ID, morningPricingStage); err != nil {
				return out, err
			}
			plan.PricingStage = morningPricingStage
			out.PricingStage = morningPricingStage
		}
	} else if changed {
		out.PricingStage = plan.PricingStage
	}

	logger.SugaredLogger.Infof(
		"MaterializeMorningTargetVolumes planId=%d sized=%d sizeSkip=%d idempotent=%d other=%d stage=%s",
		plan.ID, out.SizedCount, out.SizeSkipCount, out.IdempotentSkip, out.SkippedOther, out.PricingStage,
	)
	return out, nil
}

const (
	morningPosActionNone       = ""
	morningPosActionSized      = "sized"
	morningPosActionSizeSkip   = "size_skip"
	morningPosActionIdempotent = "idempotent"
	morningPosActionSkip       = "skip"
)

func materializeMorningItemVolume(
	plan *models.TradePlan,
	it *models.TradePlanItem,
	snap *MorningAccountSnapshot,
	limits *MorningRiskLimits,
	remainingCash, committedExtra, baseGross, equity float64,
	force bool,
) (action string, binding string, shadow *QuantityShadowObservation) {
	if it == nil {
		return morningPosActionNone, "", nil
	}
	if strings.TrimSpace(it.IntentStatus) != morningIntentStatusPriced {
		return morningPosActionSkip, "", nil
	}
	side := strings.ToLower(strings.TrimSpace(it.Side))
	if side != "" && side != "buy" {
		return morningPosActionSkip, "", nil
	}
	if it.LimitPrice <= 0 {
		return morningPosActionSkip, "", nil
	}
	minLot := morningBuyMinLot(it.StockCode)
	if !force && it.TargetVolume >= minLot {
		return morningPosActionIdempotent, morningBindIdempotent, nil
	}

	code := strings.ToLower(strings.TrimSpace(it.StockCode))

	// Position conflict (QG-I1 aligned): already hold → size_skip.
	if snap != nil && snap.PositionVolumes != nil {
		if vol := snap.PositionVolumes[code]; vol != 0 {
			it.IntentStatus = morningIntentStatusSizeSkip
			it.TargetVolume = 0
			return morningPosActionSizeSkip, morningBindPositionConflict, nil
		}
	}

	budget := it.TargetAmount
	if budget <= 0 && plan != nil {
		budget = plan.AmountPerStock
	}
	if budget <= 0 {
		it.IntentStatus = morningIntentStatusSizeSkip
		it.TargetVolume = 0
		return morningPosActionSizeSkip, morningBindNoBudget, nil
	}

	nameMV := 0.0
	if snap != nil && snap.NameMarketValue != nil {
		nameMV = snap.NameMarketValue[code]
	}
	singleCap := limits.MaxSingleNamePct * equity
	grossCap := limits.MaxGrossExposurePct * equity
	headroomName := singleCap - nameMV
	if headroomName < 0 {
		headroomName = 0
	}
	headroomGross := grossCap - baseGross - committedExtra
	if headroomGross < 0 {
		headroomGross = 0
	}
	headroomCash := remainingCash
	if headroomCash < 0 {
		headroomCash = 0
	}

	effective, binding := minPositiveConstraint(budget, headroomCash, headroomName, headroomGross)
	if effective <= 0 {
		it.IntentStatus = morningIntentStatusSizeSkip
		it.TargetVolume = 0
		return morningPosActionSizeSkip, binding, nil
	}

	// Phase12-M2.2.5: shadow compare always (Flag stays off in production; does not drive volume).
	obs := ObserveQuantityPolicyShadow(it.StockCode, effective, it.LimitPrice)
	logQuantityPolicyShadow(obs)
	shadow = &obs

	vol := calcMorningTargetVolume(it.StockCode, effective, it.LimitPrice)
	if vol < minLot {
		it.IntentStatus = morningIntentStatusSizeSkip
		it.TargetVolume = 0
		return morningPosActionSizeSkip, morningBindLot, shadow
	}

	it.IntentStatus = morningIntentStatusPriced
	it.TargetVolume = vol
	return morningPosActionSized, binding, shadow
}

func minPositiveConstraint(budget, cash, singleName, gross float64) (effective float64, binding string) {
	effective = budget
	binding = morningBindBudget
	if cash < effective {
		effective = cash
		binding = morningBindCash
	}
	if singleName < effective {
		effective = singleName
		binding = morningBindSingleName
	}
	if gross < effective {
		effective = gross
		binding = morningBindGross
	}
	return effective, binding
}

// calcMorningLotVolume floors to A-share lot (100). Uses limit_price (Order Spec), not live quote.
// Legacy path — used when EnableQuantityPolicy is false.
func calcMorningLotVolume(effectiveAmount, limitPrice float64) int64 {
	if effectiveAmount <= 0 || limitPrice <= 0 {
		return 0
	}
	return int64(math.Floor(effectiveAmount/limitPrice/float64(morningLotSize)) * float64(morningLotSize))
}

// calcMorningTargetVolume selects legacy /100 or QuantityPolicy (Flag).
//
//	Flag OFF → calcMorningLotVolume (unchanged)
//	Flag ON  → raw = floor(amount/price) → NormalizeBuyQuantity → target_volume
func calcMorningTargetVolume(stockCode string, effectiveAmount, limitPrice float64) int64 {
	if !tradingrule.EnableQuantityPolicy() {
		return calcMorningLotVolume(effectiveAmount, limitPrice)
	}
	if effectiveAmount <= 0 || limitPrice <= 0 {
		return 0
	}
	rawQty := int64(math.Floor(effectiveAmount / limitPrice))
	meta := morningQuantityMeta(stockCode)
	return tradingrule.NormalizeBuyQuantity(meta, rawQty).NormalizedQty
}

// morningBuyMinLot is the idempotent / size_skip threshold for buy volume.
func morningBuyMinLot(stockCode string) int64 {
	if !tradingrule.EnableQuantityPolicy() {
		return morningLotSize
	}
	return tradingrule.PolicyFromInstrumentMeta(morningQuantityMeta(stockCode)).MinBuyQty()
}

// morningQuantityMeta resolves M1 template meta from code (no DB; fail-closed MAIN).
func morningQuantityMeta(stockCode string) instrument.QuantityMeta {
	id, err := instrument.Classify(stockCode)
	if err != nil {
		return instrument.TemplateQuantityMeta(instrument.SecurityUnknown, instrument.BoardUNKNOWN)
	}
	meta := instrument.TemplateQuantityMeta(id.SecurityType, id.MarketSegment)
	meta.Symbol = id.Symbol
	meta.IdentitySource = id.Source
	return meta
}

func normalizeMorningRiskLimits(l *MorningRiskLimits) {
	if l == nil {
		return
	}
	if l.MaxGrossExposurePct <= 0 {
		l.MaxGrossExposurePct = 0.85
	}
	if l.MaxSingleNamePct <= 0 {
		l.MaxSingleNamePct = 0.20
	}
}

func loadMorningRiskLimitsDefault() MorningRiskLimits {
	// Phase6.5-B: exposure caps via TradingConfig Provider (legacy_paper_config; behavior unchanged).
	rv := tradingconfig.Default().Risk()
	return MorningRiskLimits{
		MaxGrossExposurePct: rv.MaxGrossExposurePct,
		MaxSingleNamePct:    rv.MaxSingleNamePct,
	}
}

func loadMorningAccountSnapshotDefault() *MorningAccountSnapshot {
	out := &MorningAccountSnapshot{
		NameMarketValue: map[string]float64{},
		PositionVolumes: map[string]int64{},
	}
	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	if err != nil || snap == nil || !snap.Found {
		logger.SugaredLogger.Warnf("morning position materialize: portfolio snapshot unavailable: %v", err)
		return out
	}
	out.Cash = snap.Cash
	out.Equity = snap.TotalEquity
	out.LongMarketValue = snap.MarketValue
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" || p.Volume <= 0 {
			continue
		}
		out.NameMarketValue[code] = p.MarketValue
		out.PositionVolumes[code] = p.Volume
	}
	if out.Equity <= 0 {
		out.Equity = out.Cash + out.LongMarketValue
	}
	return out
}
