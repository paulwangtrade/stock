package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// TradePlanExecutingTimeout executing 超过该时长视为 stale（不新增 DB 状态，仅 reconcile 观测）。
const TradePlanExecutingTimeout = 15 * time.Minute

// ReconcileResult 单计划对账结果。
type ReconcileResult struct {
	PlanID         uint   `json:"planId"`
	PreviousStatus string `json:"previousStatus"`
	TerminalStatus string `json:"terminalStatus"`
	Message        string `json:"message"`
	Applied        bool   `json:"applied"`
	SkippedReason  string `json:"skippedReason,omitempty"`
}

// SuggestReconcileTerminalStatus 根据 item/order 聚合建议终态（不重新下单）。
func SuggestReconcileTerminalStatus(summary *TradePlanExecutionSummary) (status, message string) {
	if summary == nil {
		return models.TradePlanStatusFailed, "reconcile: empty summary"
	}
	if summary.EffectiveFilled == 0 {
		return models.TradePlanStatusFailed,
			fmt.Sprintf("reconcile: no fills (pending=%d errors=%d)", summary.StillPending, summary.ErrorCount)
	}
	if summary.StillPending > 0 || summary.ErrorCount > 0 || summary.OrderPendingCount > 0 {
		return models.TradePlanStatusPartial,
			fmt.Sprintf("reconcile: partial filled=%d pending=%d errors=%d orderPending=%d",
				summary.EffectiveFilled, summary.StillPending, summary.ErrorCount, summary.OrderPendingCount)
	}
	return models.TradePlanStatusDone,
		fmt.Sprintf("reconcile: done filled=%d", summary.EffectiveFilled)
}

// ReconcileTradePlan 对单个 executing 计划 CAS 迁移至建议终态。
func ReconcileTradePlan(planID uint) (ReconcileResult, error) {
	repo := NewTradePlanRepo()
	plan, err := repo.GetByID(planID)
	if err != nil {
		return ReconcileResult{}, err
	}
	res := ReconcileResult{PlanID: planID, PreviousStatus: plan.Status}
	if plan.Status != models.TradePlanStatusExecuting {
		res.SkippedReason = "not executing"
		return res, nil
	}
	summary, err := repo.GetPlanExecutionSummary(planID)
	if err != nil {
		return res, err
	}
	toStatus, msg := SuggestReconcileTerminalStatus(summary)
	res.TerminalStatus = toStatus
	res.Message = msg
	ok, err := repo.FinishPlanCAS(planID, models.TradePlanStatusExecuting, toStatus, msg)
	if err != nil {
		return res, err
	}
	res.Applied = ok
	if !ok {
		res.SkippedReason = "CAS miss (status changed)"
	}
	return res, nil
}

// ReconcileStaleTradePlans 扫描 executing 超时计划并对账（不自动重新下单）。
func ReconcileStaleTradePlans(timeout time.Duration) ([]ReconcileResult, error) {
	if timeout <= 0 {
		timeout = TradePlanExecutingTimeout
	}
	cutoff := time.Now().Add(-timeout)
	repo := NewTradePlanRepo()
	plans, err := repo.ListExecutingPlans(cutoff)
	if err != nil {
		return nil, err
	}
	out := make([]ReconcileResult, 0, len(plans))
	for _, p := range plans {
		r, rerr := ReconcileTradePlan(p.ID)
		if rerr != nil {
			logger.SugaredLogger.Errorf("reconcile plan_id=%d: %v", p.ID, rerr)
			continue
		}
		if r.Applied {
			logger.SugaredLogger.Infof("reconcile applied plan_id=%d %s -> %s: %s",
				p.ID, r.PreviousStatus, r.TerminalStatus, r.Message)
		}
		out = append(out, r)
	}
	return out, nil
}

// IsReconcileRecommended executing 且 executed_at 早于 cutoff。
func IsReconcileRecommended(executedAt *time.Time, now time.Time, timeout time.Duration) bool {
	if executedAt == nil || executedAt.IsZero() {
		return false
	}
	if timeout <= 0 {
		timeout = TradePlanExecutingTimeout
	}
	return now.Sub(*executedAt) >= timeout
}

// ReconcileRecommendedReason 观测用阻断/提示文案。
func ReconcileRecommendedReason(executedAt *time.Time, now time.Time, timeout time.Duration) string {
	if !IsReconcileRecommended(executedAt, now, timeout) {
		return ""
	}
	ago := now.Sub(*executedAt).Truncate(time.Second)
	return fmt.Sprintf("executing stale %s (threshold %s)", ago, strings.TrimSpace(timeout.String()))
}
