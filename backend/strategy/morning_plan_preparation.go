package strategy

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// Morning plan preparation modes (observability only; not persisted Status).
const (
	MorningPlanModeAdoptFrozen  = "adopt_frozen"
	MorningPlanModeAdoptDraft   = "adopt_draft"
	MorningPlanModeBuildMorning = "build_morning"
)

type getFrozenTradePlanFunc func(tradeDate string) (*models.TradePlan, error)
type getLatestBuyDraftFunc func(tradeDate string) (*models.TradePlan, error)
type runDailyCandidateAndPlanFunc func(tradeDate string) (*models.CandidatePool, *models.TradePlan, error)

// Injectable for tests; production defaults to real implementations.
var getFrozenTradePlanFn getFrozenTradePlanFunc = GetFrozenTradePlan
var getLatestBuyDraftFn getLatestBuyDraftFunc = defaultGetLatestBuyDraft
var runDailyCandidateAndPlanFn runDailyCandidateAndPlanFunc = RunDailyCandidateAndPlan

func defaultGetLatestBuyDraft(tradeDate string) (*models.TradePlan, error) {
	return data.NewTradePlanRepo().GetLatestBuyDraftByTradeDate(tradeDate)
}

// RunMorningPlanPreparation prefers an existing Frozen TradePlan for tradeDate.
// If none, it adopts the latest same-day buy draft (no new CandidatePool/TradePlan).
// Otherwise it falls back to RunDailyCandidateAndPlan (Draft Builder).
//
// It does not modify Execution, Trading Gate, cron registration, or plan Status schema.
func RunMorningPlanPreparation(tradeDate string) (pool *models.CandidatePool, plan *models.TradePlan, mode string, err error) {
	tradeDate = normalizeTradeDate(tradeDate)

	frozen, ferr := getFrozenTradePlanFn(tradeDate)
	if ferr == nil && frozen != nil && frozen.IsFrozen() {
		mode = MorningPlanModeAdoptFrozen
		plan = frozen
		pool = loadPoolIfPresent(frozen.PoolID)
		logger.SugaredLogger.Infof(
			"RunMorningPlanPreparation mode=%s date=%s planId=%d version=%d poolId=%d",
			mode, tradeDate, plan.ID, plan.PlanVersion, plan.PoolID,
		)
		return pool, plan, mode, nil
	}

	draft, derr := getLatestBuyDraftFn(tradeDate)
	if derr == nil && isAdoptableBuyDraft(draft) {
		mode = MorningPlanModeAdoptDraft
		plan = draft
		pool = loadPoolIfPresent(draft.PoolID)
		logger.SugaredLogger.Infof(
			"RunMorningPlanPreparation mode=%s date=%s planId=%d version=%d poolId=%d",
			mode, tradeDate, plan.ID, plan.PlanVersion, plan.PoolID,
		)
		return pool, plan, mode, nil
	}

	pool, plan, err = runDailyCandidateAndPlanFn(tradeDate)
	mode = MorningPlanModeBuildMorning
	if err != nil {
		logger.SugaredLogger.Errorf(
			"RunMorningPlanPreparation mode=%s date=%s fallback Build failed: %v (frozenLookup=%v draftLookup=%v)",
			mode, tradeDate, err, ferr, derr,
		)
		return pool, plan, mode, err
	}
	logger.SugaredLogger.Infof(
		"RunMorningPlanPreparation mode=%s date=%s planId=%d poolId=%d",
		mode, tradeDate, planIDOrZero(plan), poolIDOrZero(pool),
	)
	return pool, plan, mode, nil
}

func isAdoptableBuyDraft(plan *models.TradePlan) bool {
	if plan == nil || !plan.IsDraft() {
		return false
	}
	side := strings.ToLower(strings.TrimSpace(plan.Side))
	return side == "" || side == "buy"
}

func loadPoolIfPresent(poolID uint) *models.CandidatePool {
	if poolID == 0 {
		return nil
	}
	p, err := data.NewCandidatePoolRepo().GetByID(poolID)
	if err != nil {
		return nil
	}
	return p
}

func planIDOrZero(plan *models.TradePlan) uint {
	if plan == nil {
		return 0
	}
	return plan.ID
}

func poolIDOrZero(pool *models.CandidatePool) uint {
	if pool == nil {
		return 0
	}
	return pool.ID
}
