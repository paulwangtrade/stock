package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// PaperOpenBuyStatus 开盘建仓就绪诊断（TEMP/启动检查）。
type PaperOpenBuyStatus struct {
	EnablePaperOpenBuy   bool   `json:"enablePaperOpenBuy"`
	TradeDate            string `json:"tradeDate"`
	CandidatePoolStatus  string `json:"candidatePoolStatus"`
	CandidateCount       int    `json:"candidateCount"`
	TradePlanStatus      string `json:"tradePlanStatus"`
	TradePlanCount       int    `json:"tradePlanCount"`
	ExecutionReady       bool   `json:"executionReady"`
	Message              string `json:"message"`
}

// IsWeekdayLocal 简单交易日保护：周一至周五（不含法定节假日）。
func IsWeekdayLocal(t time.Time) bool {
	wd := t.Weekday()
	return wd >= time.Monday && wd <= time.Friday
}

// GetPaperOpenBuyStatus 返回当日候选池/计划/开关就绪状态。
func GetPaperOpenBuyStatus() PaperOpenBuyStatus {
	cfg := GetPaperOpenBuyConfig()
	tradeDate := todayTradeDateLocal()
	st := PaperOpenBuyStatus{
		EnablePaperOpenBuy:  cfg.EnablePaperOpenBuy,
		TradeDate:           tradeDate,
		CandidatePoolStatus: "EMPTY",
		TradePlanStatus:     "EMPTY",
	}

	pool, perr := NewCandidatePoolRepo().GetLatestByTradeDate(tradeDate)
	if perr == nil && pool != nil {
		if pool.Status == models.CandidatePoolStatusReady && pool.ItemCount > 0 {
			st.CandidatePoolStatus = "READY"
			st.CandidateCount = pool.ItemCount
		} else {
			st.CandidatePoolStatus = "EMPTY"
			st.CandidateCount = pool.ItemCount
		}
	}

	plan, planErr := NewTradePlanRepo().GetReadyByTradeDate(tradeDate)
	if planErr == nil && plan != nil {
		st.TradePlanStatus = strings.ToUpper(plan.Status)
		st.TradePlanCount = len(plan.Items)
		if plan.Status == models.TradePlanStatusReady && len(plan.Items) > 0 {
			st.TradePlanStatus = "READY"
		}
	} else {
		// 无 ready 时仍展示最新计划状态（便于诊断）
		latest, lerr := NewTradePlanRepo().GetLatestByTradeDate(tradeDate)
		if lerr == nil && latest != nil {
			st.TradePlanStatus = strings.ToUpper(latest.Status)
			st.TradePlanCount = len(latest.Items)
		}
	}

	st.ExecutionReady = cfg.EnablePaperOpenBuy &&
		st.CandidatePoolStatus == "READY" &&
		st.TradePlanStatus == "READY" &&
		st.TradePlanCount > 0

	switch {
	case !cfg.EnablePaperOpenBuy:
		st.Message = "EnablePaperOpenBuy=false, cron 09:30 will skip"
	case st.CandidatePoolStatus != "READY":
		st.Message = "CandidatePool empty, 09:30 will skip execution"
	case st.TradePlanStatus != "READY" || st.TradePlanCount == 0:
		st.Message = "TradePlan empty, 09:30 will skip execution"
	default:
		st.Message = "executionReady=true"
	}
	return st
}

func countEnabledStockStrategies() int {
	if db.Dao == nil {
		return 0
	}
	var n int64
	_ = db.Dao.Model(&models.StockStrategy{}).Where("enable = ?", true).Count(&n).Error
	return int(n)
}

func latestStockStrategyRunSummary() string {
	api := NewStockStrategyApi()
	strat, err := api.GetFirstEnabled()
	if err != nil || strat == nil {
		return "none"
	}
	run, rerr := api.GetLatestRun(strat.ID)
	if rerr != nil || run == nil {
		return fmt.Sprintf("strategy=%q run=none", strat.Name)
	}
	return fmt.Sprintf("strategy=%q runId=%d count=%d msg=%s", strat.Name, run.ID, run.StockCount, truncateMsg(run.Message, 80))
}

func truncateMsg(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// RunPaperTradingDailyCheck 启动自检日志（不阻断启动）。
func RunPaperTradingDailyCheck() {
	cfg := GetPaperOpenBuyConfig()
	tradeDate := todayTradeDateLocal()
	st := GetPaperOpenBuyStatus()

	enabledCount := countEnabledStockStrategies()
	latestRun := latestStockStrategyRunSummary()

	poolLabel := st.CandidatePoolStatus
	planLabel := st.TradePlanStatus

	logger.SugaredLogger.Infof("================================")
	logger.SugaredLogger.Infof("Paper Trading Daily Check")
	logger.SugaredLogger.Infof("================================")
	logger.SugaredLogger.Infof("TradeDate: %s", tradeDate)
	logger.SugaredLogger.Infof("EnablePaperOpenBuy: %v", cfg.EnablePaperOpenBuy)
	logger.SugaredLogger.Infof("Enabled StockStrategy Count: %d", enabledCount)
	logger.SugaredLogger.Infof("Latest StockStrategyRun: %s", latestRun)
	logger.SugaredLogger.Infof("CandidatePool: %s", poolLabel)
	logger.SugaredLogger.Infof("CandidatePool Items: %d", st.CandidateCount)
	logger.SugaredLogger.Infof("TradePlan: %s", planLabel)
	logger.SugaredLogger.Infof("TradePlan Items: %d", st.TradePlanCount)

	if st.CandidatePoolStatus != "READY" {
		logger.SugaredLogger.Warnf("WARNING: CandidatePool empty, 09:30 will skip execution")
	}
	if st.TradePlanStatus != "READY" || st.TradePlanCount == 0 {
		logger.SugaredLogger.Warnf("WARNING: TradePlan empty, 09:30 will skip execution")
	}
	if !IsWeekdayLocal(time.Now()) {
		logger.SugaredLogger.Infof("Note: today is non-weekday; cron paper jobs will skip")
	}
	logger.SugaredLogger.Infof("ExecutionReady: %v (%s)", st.ExecutionReady, st.Message)
	logger.SugaredLogger.Infof("================================")
}
