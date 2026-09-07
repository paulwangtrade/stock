package main

import (
	"time"

	"go-stock/backend/data"
	"go-stock/backend/job"
	"go-stock/backend/logger"
	"go-stock/backend/morningpreparation"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/tradingwindow"
)

const (
	paperTradingOpenCronKey     = "paper_trading_open"
	paperTradingOpenCronSpec    = "0 31 9 * * 1-5"
	paperTradingSessionBCronKey = "paper_trading_session_b"
	// Phase10-C.4-A: 15:10 local — inside Session B [15:00,15:30); after settle@15:05.
	paperTradingSessionBCronSpec = "0 10 15 * * 1-5"
	paperTradingSettleCronKey    = "paper_trading_settle"
	paperTradingSettleCronSpec   = "0 5 15 * * 1-5"
	// Phase10-C.6-C / Phase11-K: 09:20 T+1 unlock (align Materialize window; before 09:31 fill).
	paperTradingT1UnlockCronKey  = "paper_trading_t1_unlock"
	paperTradingT1UnlockCronSpec = "0 20 9 * * 1-5"
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
				job.Default().Observe(job.JobPaperTradingOpen, func() {
					if skipIfNonWeekday("09:31 paper_trading") {
						return
					}
					runPaperTradingOpenJob()
				})()
			})
			if err != nil {
				logger.SugaredLogger.Errorf("InitPaperTradingJobs open: %s", err.Error())
			} else {
				a.setCronEntryObserved(paperTradingOpenCronKey, paperTradingOpenCronSpec, id)
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
				job.Default().Observe(job.JobPaperTradingSessionB, func() {
					if skipIfNonWeekday("15:10 paper_trading_session_b") {
						return
					}
					runPaperTradingSessionBJob()
				})()
			})
			if err != nil {
				logger.SugaredLogger.Errorf("InitPaperTradingJobs session_b: %s", err.Error())
			} else {
				a.setCronEntryObserved(paperTradingSessionBCronKey, paperTradingSessionBCronSpec, id)
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
			job.Default().Observe(job.JobPaperTradingSettle, func() {
				if skipIfNonWeekday("15:05 paper_trading_settle") {
					return
				}
				runPaperTradingSettleJob()
			})()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperTradingJobs settle: %s", err.Error())
		} else {
			a.setCronEntryObserved(paperTradingSettleCronKey, paperTradingSettleCronSpec, id)
			logger.SugaredLogger.Infof(
				"paper trading settle cron registered key=%s spec=%s enablePaperTrading=%v",
				paperTradingSettleCronKey, paperTradingSettleCronSpec, enabled,
			)
		}
	}

	// Phase11-K: T+1 unlock @09:20 via PositionState Morning Settlement (delegates PositionUnlockJob).
	if _, exists := a.getCronEntry(paperTradingT1UnlockCronKey); !exists {
		id, err := a.cron.AddFunc(paperTradingT1UnlockCronSpec, func() {
			defer PanicHandler()
			job.Default().Observe(job.JobPaperTradingT1Unlock, func() {
				if skipIfNonWeekday("09:20 paper_trading_t1_unlock") {
					return
				}
				runPaperTradingT1UnlockJob()
			})()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperTradingJobs t1_unlock: %s", err.Error())
		} else {
			a.setCronEntryObserved(paperTradingT1UnlockCronKey, paperTradingT1UnlockCronSpec, id)
			logger.SugaredLogger.Infof(
				"paper trading t1_unlock cron registered key=%s spec=%s enablePaperTrading=%v",
				paperTradingT1UnlockCronKey, paperTradingT1UnlockCronSpec, enabled,
			)
		}
	}
}

func runPaperTradingOpenJob() {
	logPlanWindowObservationBeforeOpenCron()
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

// runPaperTradingT1UnlockJob is Phase11-K 09:20 morning settlement (T+1 unlock + PositionState recompute).
// Does not call Gateway / fill / TradingEvent emit.
func runPaperTradingT1UnlockJob() {
	res, err := positionstate.RunMorningSettlement(time.Now())
	if err != nil {
		logger.SugaredLogger.Errorf("PositionStateMorningSettlement failed: %v", err)
		return
	}
	if res != nil {
		logger.SugaredLogger.Infof(
			"PositionStateMorningSettlement trade_date=%s unlocked=%d volume=%d states=%d skipped=%v msg=%s",
			res.TradeDate, res.PositionsUnlocked, res.UnlockVolumeTotal, len(res.PositionStates), res.UnlockSkipped, res.UnlockMessage,
		)
	}
}

func logPlanWindowObservationBeforeOpenCron() {
	tradeDate := time.Now().Format("2006-01-02")
	now := time.Now()
	repo := data.NewTradePlanRepo()
	plan, err := repo.GetFrozenByTradeDate(tradeDate)
	if err != nil || plan == nil {
		plan, _ = repo.GetLatestByTradeDate(tradeDate)
	}
	var eval tradingwindow.PlanWindowResult
	if plan == nil {
		eval = tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
			TradeDate:   tradeDate,
			CurrentTime: now,
			PlanStatus:  "",
			IsFrozen:    false,
		})
	} else {
		eval = tradingwindow.EvaluatePlanWindow(tradingwindow.PlanWindowInput{
			TradeDate:   plan.TradeDate,
			CurrentTime: now,
			PlanStatus:  plan.Status,
			IsFrozen:    plan.IsFrozen(),
			FrozenTime:  plan.FreezeAt,
		})
	}
	morning := morningpreparation.EvaluateMorningReadiness(morningpreparation.MorningReadinessInput{
		TradeDate: tradeDate,
		Now:       now,
		Plan:      plan,
	})
	logger.SugaredLogger.Infof(
		"PlanWindowPolicy cron observation trade_date=%s window_status=%s reason=%s morning_status=%s morning_reason=%s blocks_auto=%v blocks_manual=%v",
		tradeDate, eval.Status, eval.Reason, morning.Status, morning.Reason,
		eval.BlocksAutoExecution, tradingwindow.BlocksManualRunExecution(eval),
	)
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
