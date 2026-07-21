package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/risk"
)

// BuildDraftTradePlanFromCandidatePool projects a ready CandidatePool into a new
// append-only TradePlan draft for the same TradeDate (typically T+1).
//
// Persist rules (Phase6-A3.1):
//   - Status = draft (never ready)
//   - EnableExecute = false
//   - SourceSession = after_close
//   - PlanVersion = MAX(plan_version)+1 for TradeDate
//   - PoolID + TradeDate association
//
// Does not invoke the 9:20 daily pipeline, Approve, Freeze, or Execution.
func BuildDraftTradePlanFromCandidatePool(pool *models.CandidatePool) (*models.TradePlan, error) {
	if pool == nil {
		return nil, fmt.Errorf("candidate pool is nil")
	}
	if pool.ID == 0 {
		return nil, fmt.Errorf("candidate pool id is required")
	}
	tradeDate := strings.TrimSpace(pool.TradeDate)
	if tradeDate == "" {
		return nil, fmt.Errorf("candidate pool trade date is required")
	}
	if _, err := time.Parse("2006-01-02", tradeDate); err != nil {
		return nil, fmt.Errorf("invalid candidate pool trade date %q: %w", tradeDate, err)
	}
	if pool.Status != models.CandidatePoolStatusReady {
		return nil, fmt.Errorf("candidate pool %d status=%s, want ready", pool.ID, pool.Status)
	}
	if len(pool.Items) == 0 {
		return nil, fmt.Errorf("candidate pool %d has no items", pool.ID)
	}

	cfg := data.GetPaperOpenBuyConfig()
	amount := cfg.OpenBuyAmountPerStock
	if amount <= 0 {
		amount = 100_000
	}
	maxNames := defaultMaxPlanNames

	filtered, err := FilterPoolForTradePlan(pool, amount, maxNames)
	if err != nil {
		return nil, err
	}
	if filtered == nil {
		return nil, fmt.Errorf("risk filter result is nil")
	}

	repo := data.NewTradePlanRepo()
	version, err := repo.NextPlanVersion(tradeDate)
	if err != nil {
		return nil, err
	}

	plan := &models.TradePlan{
		TradeDate:         tradeDate,
		PoolID:            pool.ID,
		GeneratedAt:       time.Now(),
		Status:            models.TradePlanStatusDraft,
		Side:              "buy",
		AmountPerStock:    amount,
		MaxNames:          maxNames,
		EnableExecute:     false,
		Message:           fmt.Sprintf("draft from pool=%d filter=%s accepted=%d rejected=%d", pool.ID, filtered.RiskStatus, filtered.AcceptedCount, filtered.FilteredCount),
		PlanVersion:       version,
		FreezeAt:          nil,
		SourceSession:     models.TradePlanSourceAfterClose,
		RiskStatus:        filtered.RiskStatus,
		MarketLevel:       filtered.MarketLevel,
		RiskFilteredCount: filtered.FilteredCount,
		RiskAcceptedCount: filtered.AcceptedCount,
		RiskSummary:       filtered.RiskSummary,
		RiskSnapshotJSON:  filtered.RiskSnapshotJSON,
	}

	planItems := draftPlanItemsFromFilter(tradeDate, filtered)
	if err := repo.CreatePlanWithItems(plan, planItems); err != nil {
		return nil, err
	}

	logger.SugaredLogger.Infof(
		"BuildDraftTradePlanFromCandidatePool date=%s planId=%d poolId=%d version=%d status=%s enableExecute=%v items=%d",
		tradeDate, plan.ID, plan.PoolID, plan.PlanVersion, plan.Status, plan.EnableExecute, len(plan.Items),
	)
	return plan, nil
}

func draftPlanItemsFromFilter(tradeDate string, filtered *risk.PlanFilterResult) []models.TradePlanItem {
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
	return planItems
}
