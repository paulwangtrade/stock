// Phase10-H.2：注册 PlanExecutionSummaryReader，避免 data↔papertrading import cycle。
package papertrading

import (
	"go-stock/backend/data"
)

func init() {
	data.PlanExecutionSummaryReader = func(planID uint) (*data.TradePlanExecutionSummary, error) {
		view, err := DefaultExecutionReadService().BuildPlanExecutionSummary(planID)
		if err != nil {
			return nil, err
		}
		if view == nil {
			return nil, nil
		}
		return &data.TradePlanExecutionSummary{
			PlanID:             view.PlanID,
			ItemCount:          view.ItemCount,
			PendingCount:       view.PendingCount,
			FilledCount:        view.FilledCount,
			SkippedCount:       view.SkippedCount,
			ErrorCount:         view.ErrorCount,
			EffectiveFilled:    view.EffectiveFilled,
			StillPending:       view.StillPending,
			OrderFilledCount:   view.OrderFilledCount,
			OrderRejectedCount: view.OrderRejectedCount,
			OrderPendingCount:  view.OrderPendingCount,
		}, nil
	}
}
