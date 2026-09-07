package readiness

import (
	"fmt"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"
)

// LoadAndEvaluate loads plan + paper_sim context from DB and evaluates readiness.
func LoadAndEvaluate(planID uint) (ExecutionReadiness, error) {
	if planID == 0 {
		return ExecutionReadiness{}, fmt.Errorf("plan id is required")
	}
	repo := data.NewTradePlanRepo()
	plan, err := repo.GetByID(planID)
	if err != nil {
		return ExecutionReadiness{}, err
	}
	acc, err := papertrading.GetDefaultAccount()
	if err != nil {
		return ExecutionReadiness{}, err
	}
	positions, err := papertrading.GetPositions(acc.ID)
	if err != nil {
		return ExecutionReadiness{}, err
	}
	return Evaluate(int(plan.ID), mapPlanItems(plan.Items), mapAccount(acc), mapPositions(positions)), nil
}

func mapPlanItems(items []models.TradePlanItem) []PlanItem {
	out := make([]PlanItem, 0, len(items))
	for _, it := range items {
		out = append(out, PlanItem{
			StockCode:    it.StockCode,
			StockName:    it.StockName,
			Side:         it.Side,
			TargetAmount: it.TargetAmount,
			TargetVolume: it.TargetVolume,
			LimitPrice:   it.LimitPrice,
			RefPrice:     it.RefPrice,
			OpenRefPrice: it.OpenRefPrice,
			Status:       it.Status,
			IntentStatus: it.IntentStatus,
		})
	}
	return out
}

func mapAccount(acc *papertrading.PaperSimAccount) AccountSnapshot {
	if acc == nil {
		return AccountSnapshot{}
	}
	return AccountSnapshot{
		Cash:   acc.Cash,
		Equity: acc.Equity,
	}
}

func mapPositions(positions []papertrading.PaperSimPosition) []PositionSnapshot {
	out := make([]PositionSnapshot, 0, len(positions))
	for _, p := range positions {
		out = append(out, PositionSnapshot{
			StockCode:       p.StockCode,
			StockName:       p.StockName,
			TotalVolume:     p.TotalVolume,
			AvailableVolume: p.AvailableVolume,
			AvgCost:         p.AvgCost,
			MarkPrice:       p.MarkPrice,
		})
	}
	return out
}
