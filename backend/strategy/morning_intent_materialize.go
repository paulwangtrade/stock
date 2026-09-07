package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/readiness"
	"go-stock/backend/tradingrule"
)

// MorningIntentMaterializeOpts configures the Phase10-A.1 orchestrator.
// OpenPriceFn should come from an existing provider (e.g. realtime open); never fabricate.
type MorningIntentMaterializeOpts struct {
	OpenPriceFn MorningOpenPriceFunc
	VolumeOpts  *MorningPositionMaterializeOpts

	// Optional injectables for tests.
	LoadPlan          func(planID uint) (*models.TradePlan, error)
	LimitPrices       func(planID uint, openPriceFn MorningOpenPriceFunc) (*MorningPriceMaterializeResult, error)
	TargetVolumes     func(planID uint, opts *MorningPositionMaterializeOpts) (*MorningPositionMaterializeResult, error)
	EvaluateReadiness func(plan *models.TradePlan) readiness.ExecutionIntentReadinessResult
}

// MorningIntentMaterializeResult is the orchestrated morning materialize outcome.
type MorningIntentMaterializeResult struct {
	Success            bool                              `json:"success"`
	PlanID             uint                              `json:"plan_id"`
	MaterializedItems  int                               `json:"materialized_items"`
	ReadinessReady     bool                              `json:"readiness_ready"`
	Blockers           []readiness.Finding               `json:"blockers"`
	Message            string                            `json:"message,omitempty"`
	FailedStep         string                            `json:"failed_step,omitempty"`
	PricingStage       string                            `json:"pricing_stage,omitempty"`
	Price              *MorningPriceMaterializeResult    `json:"price,omitempty"`
	Volume             *MorningPositionMaterializeResult `json:"volume,omitempty"`
	NoOp               bool                              `json:"no_op,omitempty"`
	NoOpReason         string                            `json:"no_op_reason,omitempty"`
}

// RunMorningIntentMaterialize orchestrates LimitPrices → TargetVolumes → Readiness recheck.
// Does not Approve / Freeze / Execution. Idempotent via underlying Materialize* behavior.
func RunMorningIntentMaterialize(planID uint, opts *MorningIntentMaterializeOpts) (*MorningIntentMaterializeResult, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	if opts == nil {
		opts = &MorningIntentMaterializeOpts{}
	}

	out := &MorningIntentMaterializeResult{
		PlanID:   planID,
		Blockers: make([]readiness.Finding, 0),
	}

	load := opts.LoadPlan
	if load == nil {
		load = func(id uint) (*models.TradePlan, error) {
			return data.NewTradePlanRepo().GetByID(id)
		}
	}
	plan, err := load(planID)
	if err != nil {
		out.FailedStep = "precheck"
		out.Message = err.Error()
		return out, err
	}
	if plan == nil {
		out.FailedStep = "precheck"
		out.Message = fmt.Sprintf("trade plan %d not found", planID)
		return out, fmt.Errorf("%s", out.Message)
	}
	out.PlanID = plan.ID
	out.PricingStage = plan.PricingStage

	if reason := morningIntentMaterializeDenyReason(plan); reason != "" {
		out.FailedStep = "precheck"
		out.Message = reason
		out.NoOp = true
		out.NoOpReason = reason
		return out, nil
	}

	limitFn := opts.LimitPrices
	if limitFn == nil {
		limitFn = MaterializeMorningLimitPrices
	}
	volFn := opts.TargetVolumes
	if volFn == nil {
		volFn = MaterializeMorningTargetVolumes
	}
	evalReady := opts.EvaluateReadiness
	if evalReady == nil {
		evalReady = func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
			return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: false})
		}
	}

	openFn := opts.OpenPriceFn
	if openFn == nil {
		openFn = morningOpenPriceFn // may be nil → pending_open (never fabricate)
	}

	priceRes, err := limitFn(plan.ID, openFn)
	if err != nil {
		out.FailedStep = "price"
		out.Message = err.Error()
		out.Price = priceRes
		return out, err
	}
	out.Price = priceRes
	if priceRes != nil && priceRes.NoOp {
		out.NoOp = true
		out.NoOpReason = priceRes.NoOpReason
		out.FailedStep = "price"
		out.Message = "limit price materialize no-op: " + priceRes.NoOpReason
		// Still attempt readiness on current plan for diagnostics.
		return finalizeMorningIntentMaterialize(out, plan.ID, load, evalReady), nil
	}

	volOpts := opts.VolumeOpts
	if volOpts == nil {
		volOpts = &MorningPositionMaterializeOpts{}
	}
	volRes, err := volFn(plan.ID, volOpts)
	if err != nil {
		out.FailedStep = "volume"
		out.Message = err.Error()
		out.Volume = volRes
		return out, err
	}
	out.Volume = volRes
	if volRes != nil && volRes.NoOp {
		out.NoOp = true
		out.NoOpReason = volRes.NoOpReason
		out.FailedStep = "volume"
		out.Message = "target volume materialize no-op: " + volRes.NoOpReason
		return finalizeMorningIntentMaterialize(out, plan.ID, load, evalReady), nil
	}

	out.Success = true
	out.Message = "morning intent materialize completed"
	logger.SugaredLogger.Infof(
		"RunMorningIntentMaterialize planId=%d priced=%d sized=%d pendingOpen=%d",
		plan.ID,
		priceCount(priceRes),
		sizedCount(volRes),
		pendingOpenCount(priceRes),
	)
	return finalizeMorningIntentMaterialize(out, plan.ID, load, evalReady), nil
}

func morningIntentMaterializeDenyReason(plan *models.TradePlan) string {
	if plan == nil {
		return "plan is nil"
	}
	if plan.IsFrozen() {
		return "plan is frozen"
	}
	if plan.Status != models.TradePlanStatusDraft {
		return "plan is not draft"
	}
	if plan.PricingPolicyVersion < 1 {
		return "legacy plan has no execution intent"
	}
	stage := strings.TrimSpace(plan.PricingStage)
	switch stage {
	case afterClosePricingStage, morningPricingStage:
		return ""
	default:
		return fmt.Sprintf("pricing_stage %q not allowed (want %s or %s)",
			stage, afterClosePricingStage, morningPricingStage)
	}
}

func finalizeMorningIntentMaterialize(
	out *MorningIntentMaterializeResult,
	planID uint,
	load func(uint) (*models.TradePlan, error),
	eval func(*models.TradePlan) readiness.ExecutionIntentReadinessResult,
) *MorningIntentMaterializeResult {
	plan, err := load(planID)
	if err != nil || plan == nil {
		if out.Message == "" {
			out.Message = "materialize finished but reload plan failed"
		}
		if err != nil && out.FailedStep == "" {
			out.FailedStep = "readiness"
			out.Message = err.Error()
		}
		return out
	}
	out.PricingStage = plan.PricingStage
	out.MaterializedItems = countFullyMaterializedItems(plan)
	rd := eval(plan)
	out.ReadinessReady = rd.Ready
	out.Blockers = rd.Blockers
	if out.Blockers == nil {
		out.Blockers = make([]readiness.Finding, 0)
	}
	return out
}

func countFullyMaterializedItems(plan *models.TradePlan) int {
	if plan == nil {
		return 0
	}
	n := 0
	for _, it := range plan.Items {
		side := strings.ToLower(strings.TrimSpace(it.Side))
		if side != "" && side != "buy" {
			continue
		}
		if strings.TrimSpace(it.IntentStatus) == morningIntentStatusPriced &&
			it.LimitPrice > 0 && intentVolumeMeetsLot(it.StockCode, it.TargetVolume) {
			n++
		}
	}
	return n
}

// intentVolumeMeetsLot: Flag OFF → morningLotSize (100); Flag ON → QuantityPolicy Validate.
// Does not change calcMorningLotVolume / materialize sizing.
func intentVolumeMeetsLot(stockCode string, volume int64) bool {
	if !tradingrule.EnableQuantityPolicy() {
		return volume >= morningLotSize
	}
	meta := tradingrule.MetaFromStockCode(stockCode)
	return tradingrule.ValidateBuyQuantity(meta, volume).Accepted
}

func priceCount(r *MorningPriceMaterializeResult) int {
	if r == nil {
		return 0
	}
	return r.PricedCount
}

func sizedCount(r *MorningPositionMaterializeResult) int {
	if r == nil {
		return 0
	}
	return r.SizedCount
}

func pendingOpenCount(r *MorningPriceMaterializeResult) int {
	if r == nil {
		return 0
	}
	return r.PendingOpenCount
}
