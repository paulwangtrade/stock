package main

import (
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/strategy"
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

	if _, exists := a.getCronEntry(afterClosePlanCronKey); exists {
		logger.SugaredLogger.Infof("after-close plan cron already registered key=%s enable=%v",
			afterClosePlanCronKey, data.IsAfterClosePlanEnabled())
		return
	}

	id, err := a.cron.AddFunc(afterClosePlanCronSpec, func() {
		defer PanicHandler()
		if skipIfNonWeekday("15:30 after_close") {
			return
		}
		runAfterClosePlanWorkflowJob()
	})
	if err != nil {
		logger.SugaredLogger.Errorf("InitAfterClosePlanJobs: %s", err.Error())
		return
	}
	a.setCronEntry(afterClosePlanCronKey, id)
	logger.SugaredLogger.Infof(
		"after-close plan cron registered key=%s spec=%s enable=%v",
		afterClosePlanCronKey, afterClosePlanCronSpec, data.IsAfterClosePlanEnabled(),
	)
}

func runAfterClosePlanWorkflowJob() {
	if !data.IsAfterClosePlanEnabled() {
		logger.SugaredLogger.Infof("AfterCloseWorkflow cron skipped after_close_plan_enabled=false")
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
