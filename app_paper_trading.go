package main

import (
	"go-stock/backend/logger"
	"go-stock/backend/papertrading"
)

const (
	paperTradingOpenCronKey    = "paper_trading_open"
	paperTradingOpenCronSpec   = "0 31 9 * * 1-5"
	paperTradingSettleCronKey  = "paper_trading_settle"
	paperTradingSettleCronSpec = "0 5 15 * * 1-5"
)

// InitPaperTradingJobs registers independent Paper Trading MVP crons (09:31 open / 15:05 settle).
// Default enablePaperTrading=false → jobs no-op. Does not touch Real Execution or production paper_*.
func (a *App) InitPaperTradingJobs() {
	if a.cron == nil {
		return
	}
	preflight := TradingPreflightCheck()
	if !preflight.Ready {
		logger.SugaredLogger.Errorf("TRADING_START_BLOCKED paper_trading status=%s reason=%s",
			preflight.Status, preflight.Reason)
		return
	}

	enabled := papertrading.IsEnabled()

	if _, exists := a.getCronEntry(paperTradingOpenCronKey); !exists {
		id, err := a.cron.AddFunc(paperTradingOpenCronSpec, func() {
			defer PanicHandler()
			if skipIfNonWeekday("09:31 paper_trading") {
				return
			}
			runPaperTradingOpenJob()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperTradingJobs open: %s", err.Error())
		} else {
			a.setCronEntry(paperTradingOpenCronKey, id)
			logger.SugaredLogger.Infof(
				"paper trading open cron registered key=%s spec=%s enablePaperTrading=%v",
				paperTradingOpenCronKey, paperTradingOpenCronSpec, enabled,
			)
		}
	}

	if _, exists := a.getCronEntry(paperTradingSettleCronKey); !exists {
		id, err := a.cron.AddFunc(paperTradingSettleCronSpec, func() {
			defer PanicHandler()
			if skipIfNonWeekday("15:05 paper_trading_settle") {
				return
			}
			runPaperTradingSettleJob()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperTradingJobs settle: %s", err.Error())
		} else {
			a.setCronEntry(paperTradingSettleCronKey, id)
			logger.SugaredLogger.Infof(
				"paper trading settle cron registered key=%s spec=%s enablePaperTrading=%v",
				paperTradingSettleCronKey, paperTradingSettleCronSpec, enabled,
			)
		}
	}
}

func runPaperTradingOpenJob() {
	// Phase10-C.2-A: cron uses the same Execution Gateway as manual / future UI.
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		Trigger: papertrading.TriggerCron,
		Actor:   "cron",
		Price:   papertrading.DefaultOpenPriceProvider(),
	})
	if err != nil {
		logger.SugaredLogger.Errorf("ExecutionGateway cron failed: %v", err)
		return
	}
	if res != nil {
		logger.SugaredLogger.Infof("ExecutionGateway cron entry=%s status=%s plan_id=%d message=%s",
			res.Entry, res.Status, res.PlanID, res.Message)
	}
}

func runPaperTradingSettleJob() {
	res, err := papertrading.SettlementJob("", papertrading.DefaultOpenPriceProvider(), true)
	if err != nil {
		logger.SugaredLogger.Errorf("PaperSettlementJob cron failed: %v", err)
		return
	}
	if res != nil {
		logger.SugaredLogger.Infof("PaperSettlementJob cron message=%s equity=%.2f locked=%d",
			res.Message, res.Equity, res.LockedVolumeTotal)
	}
}
