package main

import (
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/strategy"
)

// GetPaperOpenBuyConfig 返回开盘执行开关配置。
func (a *App) GetPaperOpenBuyConfig() data.PaperOpenBuyConfig {
	return data.GetPaperOpenBuyConfig()
}

// SetPaperOpenBuyConfig 保存开盘执行配置。
func (a *App) SetPaperOpenBuyConfig(cfg data.PaperOpenBuyConfig) data.PaperOpenBuyConfig {
	if err := data.SavePaperOpenBuyConfig(cfg); err != nil {
		logger.SugaredLogger.Errorf("SetPaperOpenBuyConfig: %v", err)
	}
	return data.GetPaperOpenBuyConfig()
}

// BuildCandidatePool TEMP/调试：生成当日候选池。
func (a *App) BuildCandidatePool(tradeDate string) (*models.CandidatePool, error) {
	return strategy.BuildCandidatePool(tradeDate)
}

// BuildTradePlan TEMP/调试：基于当日最新池 → PlanFilter → 交易计划。
func (a *App) BuildTradePlan(tradeDate string) (*models.TradePlan, error) {
	return strategy.BuildTradePlanForDate(tradeDate)
}

// RunDailyCandidateAndPlan TEMP/调试或 9:20：池 + 计划。
func (a *App) RunDailyCandidateAndPlan(tradeDate string) map[string]any {
	pool, plan, err := strategy.RunDailyCandidateAndPlan(tradeDate)
	out := map[string]any{"ok": err == nil}
	if pool != nil {
		out["pool"] = pool
	}
	if plan != nil {
		out["plan"] = plan
	}
	if err != nil {
		out["error"] = err.Error()
	}
	return out
}

// GetTodayTradePlan 查询当日最新交易计划。
func (a *App) GetTodayTradePlan() (*models.TradePlan, error) {
	return data.GetTodayTradePlan()
}

// GetTodayTradeAnalysis TEMP/观测：当日 CandidatePool→TradePlan→Risk→Order 链路。
func (a *App) GetTodayTradeAnalysis() (*data.TradePlanAnalysis, error) {
	return data.GetTodayTradeAnalysis()
}

// GetTradePlanAnalysis TEMP/观测：按日期查询交易链。
func (a *App) GetTradePlanAnalysis(date string) (*data.TradePlanAnalysis, error) {
	return data.NewTradeAnalysisRepo().GetTradePlanAnalysis(date)
}

// GetStrategyPerformance TEMP/观测：策略×版本×信号 聚合（pnl 预留）。
func (a *App) GetStrategyPerformance() ([]data.StrategyPerformanceRow, error) {
	return data.GetStrategyPerformance()
}

// GetPaperOrderHealth TEMP/观测：订单健康报告（只读，OBS-1）。
func (a *App) GetPaperOrderHealth(date string) (*data.PaperOrderHealthReport, error) {
	return data.GetPaperOrderHealth(date)
}

// GetPaperOpenBuyStatus TEMP：诊断明日/当日是否具备 9:30 执行条件。
func (a *App) GetPaperOpenBuyStatus() data.PaperOpenBuyStatus {
	return data.GetPaperOpenBuyStatus()
}

// GetDailyTradingStatus Phase1.5：当日自动交易链路只读观测。
func (a *App) GetDailyTradingStatus() data.DailyTradingStatus {
	return data.GetDailyTradingStatus()
}

// RunPaperOpenPrepare Execution Adapter：检查 ready 计划。
func (a *App) RunPaperOpenPrepare() data.PaperOpenBuyResult {
	return data.RunPaperOpenPrepare()
}

// RunPaperOpenBuyOnce 手动执行（忽略 enable）；cron 走 requireEnabled=true。
func (a *App) RunPaperOpenBuyOnce() data.PaperOpenBuyResult {
	return data.RunPaperOpenBuyOnce(false)
}

func skipIfNonWeekday(job string) bool {
	if data.IsWeekdayLocal(time.Now()) {
		return false
	}
	logger.SugaredLogger.Infof("Skip paper open buy: non trading day (%s)", job)
	return true
}

func runTradePlanReconcileJob() {
	results, err := data.ReconcileStaleTradePlans(data.TradePlanExecutingTimeout)
	if err != nil {
		logger.SugaredLogger.Errorf("ReconcileStaleTradePlans: %v", err)
		return
	}
	if len(results) == 0 {
		return
	}
	applied := 0
	for _, r := range results {
		if r.Applied {
			applied++
		}
	}
	logger.SugaredLogger.Infof("trade plan reconcile: scanned=%d applied=%d", len(results), applied)
}

func (a *App) registerTradePlanReconcileCron(key, spec, label string) bool {
	if _, exists := a.getCronEntry(key); exists {
		return true
	}
	id, err := a.cron.AddFunc(spec, func() {
		defer PanicHandler()
		if skipIfNonWeekday(label) {
			return
		}
		runTradePlanReconcileJob()
	})
	if err != nil {
		logger.SugaredLogger.Errorf("InitPaperOpenBuyJobs reconcile %s: %s", key, err.Error())
		return false
	}
	a.setCronEntry(key, id)
	return true
}

// InitPaperOpenBuyJobs 注册 9:20 产计划 / 9:25 检查 / 9:30 执行，并输出 Daily Check。
func (a *App) InitPaperOpenBuyJobs() {
	if a.cron == nil {
		return
	}
	preflight := TradingPreflightCheck()
	if !preflight.Ready {
		logger.SugaredLogger.Errorf("TRADING_START_BLOCKED status=%s reason=%s",
			preflight.Status, preflight.Reason)
		return
	}
	const dailyKey = "paper_daily_plan"
	const prepareKey = "paper_open_prepare"
	const buyKey = "paper_open_buy"
	const reconcileKey = "paper_trade_plan_reconcile"
	const reconcileAMKey = "paper_trade_plan_reconcile_am"
	const reconcilePMKey = "paper_trade_plan_reconcile_pm"
	dailyOK, prepareOK, buyOK := false, false, false

	if _, exists := a.getCronEntry(dailyKey); !exists {
		id, err := a.cron.AddFunc("0 20 9 * * 1-5", func() {
			defer PanicHandler()
			if skipIfNonWeekday("9:20") {
				return
			}
			_, _, err := strategy.RunDailyCandidateAndPlan("")
			if err != nil {
				logger.SugaredLogger.Errorf("RunDailyCandidateAndPlan: %v", err)
			}
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperOpenBuyJobs daily: %s", err.Error())
		} else {
			a.setCronEntry(dailyKey, id)
			dailyOK = true
		}
	} else {
		dailyOK = true
	}

	if _, exists := a.getCronEntry(prepareKey); !exists {
		id, err := a.cron.AddFunc("0 25 9 * * 1-5", func() {
			defer PanicHandler()
			if skipIfNonWeekday("9:25") {
				return
			}
			data.RunPaperOpenPrepare()
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperOpenBuyJobs prepare: %s", err.Error())
		} else {
			a.setCronEntry(prepareKey, id)
			prepareOK = true
		}
	} else {
		prepareOK = true
	}

	if _, exists := a.getCronEntry(buyKey); !exists {
		id, err := a.cron.AddFunc("0 30 9 * * 1-5", func() {
			defer PanicHandler()
			if skipIfNonWeekday("9:30") {
				return
			}
			data.RunPaperOpenBuyOnce(true)
		})
		if err != nil {
			logger.SugaredLogger.Errorf("InitPaperOpenBuyJobs buy: %s", err.Error())
		} else {
			a.setCronEntry(buyKey, id)
			buyOK = true
		}
	} else {
		buyOK = true
	}

	reconcileOK := a.registerTradePlanReconcileCron(reconcileKey, "0 35 9 * * 1-5", "9:35 reconcile") &&
		a.registerTradePlanReconcileCron(reconcileAMKey, "0 0/10 10-11 * * 1-5", "am reconcile") &&
		a.registerTradePlanReconcileCron(reconcilePMKey, "0 0/10 13-14 * * 1-5", "pm reconcile")

	if dailyOK && prepareOK && buyOK && reconcileOK {
		logger.SugaredLogger.Infof("paper open buy cron registered (9:20 plan / 9:25 prepare / 9:30 execute / reconcile 9:35+intraday)")
	} else if dailyOK && prepareOK && buyOK {
		logger.SugaredLogger.Infof("paper open buy cron registered (9:20 plan / 9:25 prepare / 9:30 execute)")
	}
	if !data.PaperOpenBuyConfigFileExists() {
		logger.SugaredLogger.Warnf("paper open buy config missing")
	}
	cfg := data.GetPaperOpenBuyConfig()
	logger.SugaredLogger.Infof("paper open buy config: enable=%v amount=%.0f fallbackWhitelist=%v",
		cfg.EnablePaperOpenBuy, cfg.OpenBuyAmountPerStock, cfg.AllowWhitelistFallback)

	// 启动自检（不阻断）
	data.RunPaperTradingDailyCheck()
}
