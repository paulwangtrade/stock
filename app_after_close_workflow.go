package main

import (
	"go-stock/backend/job"
	"go-stock/backend/logger"
	"go-stock/backend/marketstate"
	"go-stock/backend/strategy"
	"go-stock/backend/tradingconfig"
)

const (
	afterClosePlanCronKey  = "after_close_plan_workflow"
	afterClosePlanCronSpec = "0 30 15 * * 1-5"
)

// afterCloseWorkflowRunner is the production Workflow entry; tests may replace it.
var afterCloseWorkflowRunner = strategy.RunAfterClosePlanWorkflow

// InitAfterClosePlanJobs registers the after-close Candidate→Draft→Risk cron.
// It does not Approve, Freeze, Execute, or alter the 9:20 pipeline / Trading Gate.
func (a *App) InitAfterClosePlanJobs() {
	if a.cron == nil {
		return
	}
	preflight := TradingPreflightCheck()
	if !preflight.Ready {
		logger.SugaredLogger.Errorf("TRADING_START_BLOCKED after_close status=%s reason=%s",
			preflight.Status, preflight.Reason)
		return
	}

	tradingconfig.LogInitialized()
	afterCloseEnabled := tradingconfig.Default().AfterCloseEnabled()
	if _, exists := a.getCronEntry(afterClosePlanCronKey); exists {
		logger.SugaredLogger.Infof("after-close plan cron already registered key=%s enable=%v source=%s",
			afterClosePlanCronKey, afterCloseEnabled, tradingconfig.SourceLegacyAfterClose)
		return
	}

	id, err := a.cron.AddFunc(afterClosePlanCronSpec, func() {
		defer PanicHandler()
		job.Default().Observe(job.JobAfterCloseWorkflow, func() {
			if skipIfNonWeekday("15:30 after_close") {
				return
			}
			runAfterClosePlanWorkflowJob()
		})()
	})
	if err != nil {
		logger.SugaredLogger.Errorf("InitAfterClosePlanJobs: %s", err.Error())
		return
	}
	a.setCronEntryObserved(afterClosePlanCronKey, afterClosePlanCronSpec, id)
	logger.SugaredLogger.Infof(
		"after-close plan cron registered key=%s spec=%s enable=%v source=%s",
		afterClosePlanCronKey, afterClosePlanCronSpec, afterCloseEnabled, tradingconfig.SourceLegacyAfterClose,
	)
}

func runAfterClosePlanWorkflowJob() {
	// Phase11-B: plan generation gate (time-only; no strategy/order).
	if !marketstate.CanGeneratePlan() {
		logger.SugaredLogger.Infof("AfterCloseWorkflow cron skipped market_state=%s canGeneratePlan=false",
			marketstate.GetCurrentMarketState())
		return
	}
	// Phase6.5-A: after_close switch via TradingConfig Provider (legacy_after_close; behavior unchanged).
	if !tradingconfig.Default().AfterCloseEnabled() {
		logger.SugaredLogger.Infof("AfterCloseWorkflow cron skipped after_close_plan_enabled=false source=%s",
			tradingconfig.SourceLegacyAfterClose)
		return
	}

	logger.SugaredLogger.Infof("AfterCloseWorkflow cron started sourceDate=\"\" trigger=cron")
	res, err := afterCloseWorkflowRunner("")
	if err != nil {
		failedStep := ""
		if res != nil {
			failedStep = res.FailedStep
		}
		logger.SugaredLogger.Errorf(
			"AfterCloseWorkflow cron failed failedStep=%s err=%v",
			failedStep, err,
		)
		if res != nil {
			logger.SugaredLogger.Infof(
				"AfterCloseWorkflow cron completed ok=false source=%s tradeDate=%s poolId=%d planId=%d version=%d riskPassed=%v failedStep=%s message=%s",
				res.SourceDate, res.TradeDate, res.CandidatePoolID, res.TradePlanID, res.PlanVersion, res.RiskPassed, res.FailedStep, res.Message,
			)
		}
		return
	}
	if res == nil {
		logger.SugaredLogger.Errorf("AfterCloseWorkflow cron failed: result is nil")
		return
	}

	if res.RiskPassed {
		logger.SugaredLogger.Infof(
			"AfterCloseWorkflow cron risk result planId=%d version=%d riskPassed=true",
			res.TradePlanID, res.PlanVersion,
		)
	} else {
		logger.SugaredLogger.Warnf(
			"AfterCloseWorkflow cron risk result planId=%d version=%d riskPassed=false",
			res.TradePlanID, res.PlanVersion,
		)
	}

	logger.SugaredLogger.Infof(
		"AfterCloseWorkflow cron completed ok=%v source=%s tradeDate=%s poolId=%d planId=%d version=%d riskPassed=%v failedStep=%s message=%s",
		res.OK, res.SourceDate, res.TradeDate, res.CandidatePoolID, res.TradePlanID, res.PlanVersion, res.RiskPassed, res.FailedStep, res.Message,
	)
}
