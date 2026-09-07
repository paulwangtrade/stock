package morningpreparation

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/strategy"
)

// JobOptions configures MorningPlanPreparationJob (no execution / fill).
type JobOptions struct {
	TradeDate          string
	Now                time.Time
	AttemptMaterialize bool
	DeadlineCheckpoint bool
}

// JobResult is the outcome of MorningPlanPreparationJob.
type JobResult struct {
	Observation          MorningReadinessObservation
	MaterializeAttempted bool
	MaterializeSuccess   bool
	Message              string
}

type planLookupFn func(tradeDate string) (*models.TradePlan, error)
type materializeFn func(planID uint) (*strategy.MorningIntentMaterializeResult, error)

var (
	lookupPlanFn     planLookupFn  = defaultLookupPlan
	runMaterializeFn materializeFn = defaultMaterialize
)

// RunMorningPlanPreparationJob checks today's plan, optionally materializes draft, emits observation.
// Does not Approve, Freeze, order, fill, or call RunExecution.
func RunMorningPlanPreparationJob(opts JobOptions) JobResult {
	tradeDate := strings.TrimSpace(opts.TradeDate)
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	if tradeDate == "" {
		tradeDate = now.Format("2006-01-02")
	}

	plan, _ := lookupPlanFn(tradeDate)
	matAttempted := false
	matSuccess := false
	msg := "observation only"

	if opts.AttemptMaterialize && plan != nil && shouldAttemptMaterialize(plan) {
		matAttempted = true
		res, err := runMaterializeFn(plan.ID)
		if err != nil {
			msg = "materialize error: " + err.Error()
			logger.SugaredLogger.Warnf("MorningPlanPreparationJob materialize planId=%d err=%v", plan.ID, err)
		} else if res != nil {
			matSuccess = res.Success
			msg = res.Message
			logger.SugaredLogger.Infof(
				"MorningPlanPreparationJob materialize planId=%d success=%t ready=%t items=%d stage=%s",
				plan.ID, res.Success, res.ReadinessReady, res.MaterializedItems, res.PricingStage,
			)
		}
		if reloaded, err := data.NewTradePlanRepo().GetByID(plan.ID); err == nil && reloaded != nil {
			plan = reloaded
		}
	}

	obs := EvaluateMorningReadiness(MorningReadinessInput{
		TradeDate:            tradeDate,
		Now:                  now,
		Plan:                 plan,
		MaterializeAttempted: matAttempted,
		MaterializeSuccess:   matSuccess,
		DeadlineCheckpoint:   opts.DeadlineCheckpoint,
	})

	logger.SugaredLogger.Infof(
		"MorningPlanPreparationJob trade_date=%s plan_id=%d status=%s materialization=%s freeze=%s deadline=%s reason=%s checkpoint=%t materialize_attempted=%t",
		obs.TradeDate, obs.PlanID, obs.Status, obs.MaterializationStatus, obs.FreezeStatus,
		obs.DeadlineStatus, obs.Reason, opts.DeadlineCheckpoint, matAttempted,
	)

	return JobResult{
		Observation:          obs,
		MaterializeAttempted: matAttempted,
		MaterializeSuccess:   matSuccess,
		Message:              msg,
	}
}

func shouldAttemptMaterialize(plan *models.TradePlan) bool {
	if plan == nil || plan.IsFrozen() {
		return false
	}
	if isSellIsolatedFromBuyMaterialize(plan) {
		return false
	}
	if plan.Status != models.TradePlanStatusDraft {
		return false
	}
	return !isMorningMaterialized(plan)
}

// LookupMorningMaterializePlan is the 09:26 buy-materialize target picker.
// It never returns side=sell. If the shared latest row is sell, it falls back
// to the newest same-day buy draft. Does not support sell materialize.
func LookupMorningMaterializePlan(tradeDate string) (*models.TradePlan, error) {
	return defaultLookupPlan(tradeDate)
}

func defaultLookupPlan(tradeDate string) (*models.TradePlan, error) {
	repo := data.NewTradePlanRepo()
	if frozen, err := repo.GetFrozenByTradeDate(tradeDate); err == nil && frozen != nil && isMorningBuySide(frozen) {
		return frozen, nil
	}
	latest, err := repo.GetLatestByTradeDate(tradeDate)
	if err != nil || latest == nil {
		return nil, err
	}
	if isMorningBuySide(latest) {
		return latest, nil
	}
	// Latest is sell (or other non-buy): keep searching for a buy draft; never return sell.
	buy, berr := repo.GetLatestBuyDraftByTradeDate(tradeDate)
	if berr != nil || buy == nil {
		return nil, berr
	}
	return buy, nil
}

func defaultMaterialize(planID uint) (*strategy.MorningIntentMaterializeResult, error) {
	openFn := func(stockCode string) (float64, bool) {
		code := strings.TrimSpace(stockCode)
		if code == "" {
			return 0, false
		}
		q, ok := papertrading.RealtimeOpenPriceProvider{}.OpenQuote(code, "")
		if !ok || q.Open <= 0 {
			return 0, false
		}
		return q.Open, true
	}
	return strategy.RunMorningIntentMaterialize(planID, &strategy.MorningIntentMaterializeOpts{
		OpenPriceFn: openFn,
	})
}
