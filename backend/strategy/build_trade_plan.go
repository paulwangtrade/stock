package strategy

import (
	"fmt"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/risk"
)

// BuildTradePlan 仅落库 RiskFilter 结果，不做任何风控判断。
func BuildTradePlan(tradeDate string, pool *models.CandidatePool, filtered *risk.PlanFilterResult) (*models.TradePlan, error) {
	tradeDate = normalizeTradeDate(tradeDate)
	if pool == nil {
		return nil, fmt.Errorf("candidate pool is nil")
	}
	if filtered == nil {
		return nil, fmt.Errorf("risk filter result is nil")
	}

	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	maxNames := defaultMaxPlanNames

	plan := &models.TradePlan{
		TradeDate:         tradeDate,
		PoolID:            pool.ID,
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusReady,
		Side:              "buy",
		AmountPerStock:    amount,
		MaxNames:          maxNames,
		EnableExecute:     cfg.EnablePaperOpenBuy,
		Message:           fmt.Sprintf("from pool=%d filter=%s accepted=%d rejected=%d", pool.ID, filtered.RiskStatus, filtered.AcceptedCount, filtered.FilteredCount),
		RiskStatus:        filtered.RiskStatus,
		MarketLevel:       filtered.MarketLevel,
		RiskFilteredCount: filtered.FilteredCount,
		RiskAcceptedCount: filtered.AcceptedCount,
		RiskSummary:       filtered.RiskSummary,
		RiskSnapshotJSON:  filtered.RiskSnapshotJSON,
	}

	planItems := make([]models.TradePlanItem, 0, len(filtered.Items))
	for _, it := range filtered.Items {
		c := it.Candidate
		status := it.Status
		if status == "" {
			if it.Allowed {
				status = models.TradePlanItemPending
			} else {
				status = models.TradePlanItemSkipped
			}
		}
		planItems = append(planItems, models.TradePlanItem{
			TradeDate:       tradeDate,
			StockCode:       c.StockCode,
			StockName:       c.StockName,
			Side:            "buy",
			Priority:        it.Priority,
			TargetAmount:    c.TargetAmount,
			Score:           c.Score,
			Reason:          c.Reason,
			StrategyName:    c.StrategyName,
			StrategyVersion: c.StrategyVersion,
			Status:          status,
			RiskCode:        string(it.RiskCode),
			RiskMessage:     it.RiskMessage,
		})
	}

	if err := data.NewTradePlanRepo().CreatePlanWithItems(plan, planItems); err != nil {
		return nil, err
	}
	logger.SugaredLogger.Infof("BuildTradePlan date=%s planId=%d poolId=%d items=%d accepted=%d rejected=%d risk=%s",
		tradeDate, plan.ID, plan.PoolID, len(plan.Items), plan.RiskAcceptedCount, plan.RiskFilteredCount, plan.RiskStatus)
	return plan, nil
}

// BuildTradePlanForDate 调试入口：读最新池 → PlanFilter → BuildTradePlan。
func BuildTradePlanForDate(tradeDate string) (*models.TradePlan, error) {
	tradeDate = normalizeTradeDate(tradeDate)
	poolRepo := data.NewCandidatePoolRepo()
	pool, err := poolRepo.GetLatestByTradeDate(tradeDate)
	if err != nil {
		return nil, fmt.Errorf("no candidate pool for %s: %w", tradeDate, err)
	}
	if pool.Status != models.CandidatePoolStatusReady || len(pool.Items) == 0 {
		return nil, fmt.Errorf("candidate pool %d not ready or empty", pool.ID)
	}
	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	filtered, err := FilterPoolForTradePlan(pool, amount, defaultMaxPlanNames)
	if err != nil {
		return nil, err
	}
	return BuildTradePlan(tradeDate, pool, filtered)
}

// RunDailyCandidateAndPlan 9:20：CandidatePool → risk.PlanFilter → BuildTradePlan。
func RunDailyCandidateAndPlan(tradeDate string) (*models.CandidatePool, *models.TradePlan, error) {
	tradeDate = normalizeTradeDate(tradeDate)
	pool, err := BuildCandidatePool(tradeDate)
	if err != nil {
		logger.SugaredLogger.Errorf("[PaperPlan] BuildCandidatePool failed: %v", err)
		return nil, nil, err
	}
	if pool.Status != models.CandidatePoolStatusReady || pool.ItemCount == 0 {
		logger.SugaredLogger.Warnf("[PaperPlan] candidate pool empty date=%s msg=%s", tradeDate, pool.Message)
		return pool, nil, fmt.Errorf("candidate pool empty: %s", pool.Message)
	}

	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	filtered, err := FilterPoolForTradePlan(pool, amount, defaultMaxPlanNames)
	if err != nil {
		logger.SugaredLogger.Errorf("[PaperPlan] PlanFilter failed: %v", err)
		return pool, nil, err
	}

	plan, err := BuildTradePlan(tradeDate, pool, filtered)
	if err != nil {
		logger.SugaredLogger.Errorf("[PaperPlan] BuildTradePlan failed: %v", err)
		return pool, nil, err
	}

	pending := make([]string, 0, len(plan.Items))
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			pending = append(pending, it.StockCode)
		}
	}
	strategyName, strategyVer := "", ""
	if len(plan.Items) > 0 {
		strategyName = plan.Items[0].StrategyName
		strategyVer = plan.Items[0].StrategyVersion
	}
	logger.SugaredLogger.Infof("[PaperPlan] source=%s strategy=%s version=%s pending=%d stocks=%v planId=%d risk=%s",
		pool.Source, strategyName, strategyVer, len(pending), pending, plan.ID, plan.RiskStatus)
	return pool, plan, nil
}
