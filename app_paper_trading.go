package main

import (
	"go-stock/backend/logger"
	"go-stock/backend/papertrading"
)

const (
	paperTradingOpenCronKey     = "paper_trading_open"
	paperTradingOpenCronSpec    = "0 31 9 * * 1-5"
	paperTradingSessionBCronKey = "paper_trading_session_b"
	// Phase10-C.4-A: 15:10 local — inside Session B [15:00,15:30); after settle@15:05.
	paperTradingSessionBCronSpec = "0 10 15 * * 1-5"
	paperTradingSettleCronKey    = "paper_trading_settle"
	paperTradingSettleCronSpec   = "0 5 15 * * 1-5"
)

// InitPaperTradingJobs registers Paper Trading MVP crons.
// Fill cron is exclusive via Config.FillMode (default A):
//   - A: 09:31 Session A open → RunExecution
//   - B: 15:10 Session B close → RunExecution (observation sampling)
// Settle @15:05 always registers (mark-to-market; not Fill).
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
	fillMode := papertrading.EffectiveFillMode()
	registerOpen, registerSessionB := papertrading.FillCronExclusive(fillMode)

	if registerOpen {
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
					"paper trading open cron registered key=%s spec=%s fillMode=%s enablePaperTrading=%v",
					paperTradingOpenCronKey, paperTradingOpenCronSpec, fillMode, enabled,
				)
			}
		}
	}

	if registerSessionB {
		if _, exists := a.getCronEntry(paperTradingSessionBCronKey); !exists {
			id, err := a.cron.AddFunc(paperTradingSessionBCronSpec, func() {
				defer PanicHandler()
				if skipIfNonWeekday("15:10 paper_trading_session_b") {
					return
				}
				runPaperTradingSessionBJob()
			})
			if err != nil {
				logger.SugaredLogger.Errorf("InitPaperTradingJobs session_b: %s", err.Error())
			} else {
				a.setCronEntry(paperTradingSessionBCronKey, id)
				logger.SugaredLogger.Infof(
					"paper trading session_b cron registered key=%s spec=%s fillMode=%s enablePaperTrading=%v",
					paperTradingSessionBCronKey, paperTradingSessionBCronSpec, fillMode, enabled,
				)
			}
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
		logger.SugaredLogger.Infof("ExecutionGateway cron entry=%s session=%s price_mode=%s status=%s plan_id=%d message=%s",
			res.Entry, res.Session, res.PriceMode, res.Status, res.PlanID, res.Message)
	}
}

func runPaperTradingSessionBJob() {
	// Phase10-C.4-A: exclusive FillMode=B sampling. Gateway selects CloseFill when clock ∈ Session B.
	res, err := papertrading.RunExecution(papertrading.ExecutionRequest{
		Trigger: papertrading.TriggerCron,
		Actor:   papertrading.ActorCronSessionB,
		Price:   papertrading.DefaultOpenPriceProvider(), // overridden by SelectFillProvider on B
	})
	if err != nil {
		logger.SugaredLogger.Errorf("ExecutionGateway session_b cron failed: %v", err)
		return
	}
	if res != nil {
		logger.SugaredLogger.Infof(
			"ExecutionGateway session_b cron entry=%s session=%s price_mode=%s status=%s plan_id=%d message=%s",
			res.Entry, res.Session, res.PriceMode, res.Status, res.PlanID, res.Message,
		)
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
