package strategy

import (
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// Morning plan preparation modes (observability only; not persisted Status).
const (
	MorningPlanModeAdoptFrozen  = "adopt_frozen"
	MorningPlanModeBuildMorning = "build_morning"
)

type getFrozenTradePlanFunc func(tradeDate string) (*models.TradePlan, error)
type runDailyCandidateAndPlanFunc func(tradeDate string) (*models.CandidatePool, *models.TradePlan, error)

// Injectable for tests; production defaults to real implementations.
var getFrozenTradePlanFn getFrozenTradePlanFunc = GetFrozenTradePlan
var runDailyCandidateAndPlanFn runDailyCandidateAndPlanFunc = RunDailyCandidateAndPlan

// RunMorningPlanPreparation prefers an existing Frozen TradePlan for tradeDate.
// If found, it adopts that plan without building a new CandidatePool/TradePlan.
// Otherwise it falls back to RunDailyCandidateAndPlan.
//
// It does not modify Execution, Trading Gate, cron registration, or plan Status schema.
func RunMorningPlanPreparation(tradeDate string) (pool *models.CandidatePool, plan *models.TradePlan, mode string, err error) {
	tradeDate = normalizeTradeDate(tradeDate)

	frozen, ferr := getFrozenTradePlanFn(tradeDate)
	if ferr == nil && frozen != nil && frozen.IsFrozen() {
		mode = MorningPlanModeAdoptFrozen
		plan = frozen
		if frozen.PoolID != 0 {
			if p, perr := data.NewCandidatePoolRepo().GetByID(frozen.PoolID); perr == nil {
				pool = p
			}
		}
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
			"RunMorningPlanPreparation mode=%s date=%s fallback Build failed: %v (frozenLookup=%v)",
			mode, tradeDate, err, ferr,
		)
		return pool, plan, mode, err
	}
	logger.SugaredLogger.Infof(
		"RunMorningPlanPreparation mode=%s date=%s planId=%d poolId=%d",
		mode, tradeDate, planIDOrZero(plan), poolIDOrZero(pool),
	)
	return pool, plan, mode, nil
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
