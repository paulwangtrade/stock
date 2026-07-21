package data

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// DailyTradingStatus Phase1.5：当日自动交易链路只读观测（不改交易逻辑）。
type DailyTradingStatus struct {
	TradeDate          string `json:"tradeDate"`
	IsWeekday          bool   `json:"isWeekday"`
	EnablePaperOpenBuy bool   `json:"enablePaperOpenBuy"`

	Candidate DailyCandidateStatus `json:"candidate"`
	Plan      DailyPlanStatus      `json:"plan"`
	Risk      DailyRiskStatus      `json:"risk"`
	Execution DailyExecutionStatus `json:"execution"`
	Paper     DailyPaperStatus     `json:"paper"`

	BlockReason  string   `json:"blockReason"`
	BlockReasons []string `json:"blockReasons"`
	Message      string   `json:"message"`
}

// DailyCandidateStatus 候选池观测。
type DailyCandidateStatus struct {
	PoolID  uint   `json:"poolId,omitempty"`
	Status  string `json:"status"` // EMPTY | READY
	Count   int    `json:"count"`
	Source  string `json:"source,omitempty"`
	Message string `json:"message,omitempty"`
}

// DailyPlanStatus 交易计划观测。
type DailyPlanStatus struct {
	PlanID                uint   `json:"planId,omitempty"`
	Status                string `json:"status"` // EMPTY | ready | executing | done | ...
	ItemCount             int    `json:"itemCount"`
	PendingCount          int    `json:"pendingCount"`
	FilledCount           int    `json:"filledCount"`
	SkippedCount          int    `json:"skippedCount"`
	ErrorCount            int    `json:"errorCount"`
	Message               string `json:"message,omitempty"`
	ExecutingSince        string `json:"executingSince,omitempty"`
	ReconcileRecommended  bool   `json:"reconcileRecommended"`
	ReconcileRecommendedReason string `json:"reconcileRecommendedReason,omitempty"`
}

// DailyRiskStatus PlanFilter 结果观测。
type DailyRiskStatus struct {
	Status        string `json:"status"` // N/A | pass | partial | reject | ...
	AcceptedCount int    `json:"acceptedCount"`
	FilteredCount int    `json:"filteredCount"`
	Summary       string `json:"summary,omitempty"`
	MarketLevel   int    `json:"marketLevel"`
}

// DailyExecutionStatus 执行编排观测。
type DailyExecutionStatus struct {
	Phase              string `json:"phase"` // not_started | ready | executing | done | partial | failed | skipped
	Ready              bool   `json:"ready"`
	ExecutorConfigured bool   `json:"executorConfigured"`
	Message            string `json:"message,omitempty"`
}

// DailyPaperStatus 纸面成交观测。
type DailyPaperStatus struct {
	OrderCount       int     `json:"orderCount"`
	FillCount        int     `json:"fillCount"`
	FilledOrderCount int     `json:"filledOrderCount"`
	HasAccount       bool    `json:"hasAccount"`
	AccountID        uint    `json:"accountId,omitempty"`
	AccountCash      float64 `json:"accountCash,omitempty"`
}

// GetDailyTradingStatus 返回当前交易日自动交易链路状态与阻断原因（只读）。
func GetDailyTradingStatus() DailyTradingStatus {
	return GetDailyTradingStatusForDate(todayTradeDateLocal())
}

// GetDailyTradingStatusForDate 按交易日查询（空日期默认当天）。
func GetDailyTradingStatusForDate(tradeDate string) DailyTradingStatus {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		tradeDate = todayTradeDateLocal()
	}

	cfg := GetPaperOpenBuyConfig()
	now := time.Now()
	st := DailyTradingStatus{
		TradeDate:          tradeDate,
		IsWeekday:          IsWeekdayLocal(now),
		EnablePaperOpenBuy: cfg.EnablePaperOpenBuy,
		Candidate:          DailyCandidateStatus{Status: "EMPTY"},
		Plan:               DailyPlanStatus{Status: "EMPTY"},
		Risk:               DailyRiskStatus{Status: "N/A"},
		Execution: DailyExecutionStatus{
			Phase:              "not_started",
			ExecutorConfigured: getPlanItemExecutor() != nil,
		},
	}

	pool, poolErr := NewCandidatePoolRepo().GetLatestByTradeDate(tradeDate)
	if poolErr == nil && pool != nil {
		st.Candidate.PoolID = pool.ID
		st.Candidate.Count = pool.ItemCount
		st.Candidate.Source = pool.Source
		st.Candidate.Message = pool.Message
		if pool.Status == models.CandidatePoolStatusReady && pool.ItemCount > 0 {
			st.Candidate.Status = "READY"
		} else {
			st.Candidate.Status = "EMPTY"
		}
	}

	plan, planErr := NewTradePlanRepo().GetLatestByTradeDate(tradeDate)
	if planErr == nil && plan != nil {
		st.Plan.PlanID = plan.ID
		st.Plan.Status = strings.ToLower(strings.TrimSpace(plan.Status))
		st.Plan.Message = plan.Message
		for _, it := range plan.Items {
			st.Plan.ItemCount++
			switch it.Status {
			case models.TradePlanItemPending, "":
				st.Plan.PendingCount++
			case models.TradePlanItemFilled:
				st.Plan.FilledCount++
			case models.TradePlanItemSkipped:
				st.Plan.SkippedCount++
			case models.TradePlanItemError:
				st.Plan.ErrorCount++
			}
		}

		st.Risk.Status = strings.TrimSpace(plan.RiskStatus)
		if st.Risk.Status == "" {
			st.Risk.Status = "N/A"
		}
		st.Risk.AcceptedCount = plan.RiskAcceptedCount
		st.Risk.FilteredCount = plan.RiskFilteredCount
		st.Risk.Summary = plan.RiskSummary
		st.Risk.MarketLevel = plan.MarketLevel

		if plan.Status == models.TradePlanStatusExecuting && plan.ExecutedAt != nil && !plan.ExecutedAt.IsZero() {
			st.Plan.ExecutingSince = plan.ExecutedAt.Format(time.RFC3339)
			st.Plan.ReconcileRecommended = IsReconcileRecommended(plan.ExecutedAt, now, TradePlanExecutingTimeout)
			if st.Plan.ReconcileRecommended {
				st.Plan.ReconcileRecommendedReason = ReconcileRecommendedReason(plan.ExecutedAt, now, TradePlanExecutingTimeout)
			}
		}

		st.Execution.Phase = mapPlanToExecutionPhase(plan, st.Plan)
		st.Execution.Ready = cfg.EnablePaperOpenBuy &&
			st.Candidate.Status == "READY" &&
			plan.Status == models.TradePlanStatusReady &&
			st.Plan.PendingCount > 0 &&
			st.Execution.ExecutorConfigured
	}

	st.Paper = loadDailyPaperStatus(tradeDate, plan)
	st.BlockReasons = deriveDailyBlockReasons(st)
	st.BlockReason = firstBlockReason(st.BlockReasons)
	st.Message = buildDailyStatusMessage(st)
	return st
}

func mapPlanToExecutionPhase(plan *models.TradePlan, counts DailyPlanStatus) string {
	if plan == nil {
		return "not_started"
	}
	switch plan.Status {
	case models.TradePlanStatusReady:
		if counts.PendingCount > 0 {
			return "ready"
		}
		if counts.FilledCount > 0 {
			return "done"
		}
		return "skipped"
	case models.TradePlanStatusExecuting:
		return "executing"
	case models.TradePlanStatusDone:
		return "done"
	case models.TradePlanStatusPartial:
		return "partial"
	case models.TradePlanStatusFailed:
		return "failed"
	case models.TradePlanStatusSkipped:
		return "skipped"
	default:
		return strings.ToLower(strings.TrimSpace(plan.Status))
	}
}

func loadDailyPaperStatus(tradeDate string, plan *models.TradePlan) DailyPaperStatus {
	out := DailyPaperStatus{}
	if db.Dao == nil {
		return out
	}

	orderIDs := map[uint]struct{}{}
	if plan != nil {
		for _, it := range plan.Items {
			if it.OrderID > 0 {
				orderIDs[it.OrderID] = struct{}{}
			}
		}
	}

	var orders []PaperOrder
	q := db.Dao.Model(&PaperOrder{})
	if len(orderIDs) > 0 {
		ids := make([]uint, 0, len(orderIDs))
		for id := range orderIDs {
			ids = append(ids, id)
		}
		q = q.Where("id IN ?", ids)
	} else {
		start, end := tradeDateBounds(tradeDate)
		q = q.Where("created_at >= ? AND created_at < ?", start, end)
	}
	_ = q.Find(&orders).Error

	out.OrderCount = len(orders)
	for _, o := range orders {
		if o.Status == PaperOrderStatusFilled {
			out.FilledOrderCount++
		}
	}

	fillQ := db.Dao.Model(&PaperFill{})
	if len(orderIDs) > 0 {
		ids := make([]uint, 0, len(orderIDs))
		for id := range orderIDs {
			ids = append(ids, id)
		}
		fillQ = fillQ.Where("order_id IN ?", ids)
	} else {
		start, end := tradeDateBounds(tradeDate)
		fillQ = fillQ.Where("filled_at >= ? AND filled_at < ?", start, end)
	}
	var fillCount int64
	_ = fillQ.Count(&fillCount).Error
	out.FillCount = int(fillCount)

	var acc PaperAccount
	if err := db.Dao.Order("id ASC").First(&acc).Error; err == nil {
		out.HasAccount = true
		out.AccountID = acc.ID
		out.AccountCash = acc.Cash
	}
	return out
}

func tradeDateBounds(tradeDate string) (time.Time, time.Time) {
	day, err := time.ParseInLocation("2006-01-02", tradeDate, time.Local)
	if err != nil {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		return start, start.Add(24 * time.Hour)
	}
	return day, day.Add(24 * time.Hour)
}

func deriveDailyBlockReasons(st DailyTradingStatus) []string {
	var reasons []string
	add := func(code string) {
		for _, r := range reasons {
			if r == code {
				return
			}
		}
		reasons = append(reasons, code)
	}

	if !st.IsWeekday {
		add("non_weekday")
	}
	if !st.EnablePaperOpenBuy {
		add("enable_paper_open_buy_disabled")
	}
	if st.Candidate.Status != "READY" || st.Candidate.Count == 0 {
		add("candidate_pool_empty")
	}
	if st.Plan.Status == "EMPTY" || st.Plan.PlanID == 0 {
		add("trade_plan_missing")
	} else if st.Plan.Status != models.TradePlanStatusReady {
		switch st.Plan.Status {
		case models.TradePlanStatusExecuting:
			if st.Plan.PendingCount > 0 {
				add("plan_stuck_executing")
			}
			if st.Plan.ReconcileRecommended {
				add("plan_reconcile_recommended")
			}
		case models.TradePlanStatusDone, models.TradePlanStatusPartial:
			if st.Plan.FilledCount == 0 && st.Paper.FillCount == 0 {
				add("execution_finished_without_fills")
			}
		case models.TradePlanStatusFailed:
			add("execution_failed")
		case models.TradePlanStatusSkipped:
			add("execution_skipped")
		default:
			add("trade_plan_not_ready")
		}
	}
	if st.Plan.PlanID > 0 && st.Plan.PendingCount == 0 && st.Plan.FilledCount == 0 && st.Plan.ItemCount > 0 {
		add("all_plan_items_filtered_or_skipped")
	}
	if !st.Execution.ExecutorConfigured {
		add("plan_item_executor_not_configured")
	}
	if st.Execution.Phase == "done" && st.Paper.FillCount > 0 {
		// 已成功执行，不再视为阻断
		return filterExecutionBlockReasons(reasons)
	}
	return reasons
}

func filterExecutionBlockReasons(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		switch r {
		case "candidate_pool_empty", "trade_plan_missing", "trade_plan_not_ready",
			"all_plan_items_filtered_or_skipped", "enable_paper_open_buy_disabled":
			continue
		default:
			out = append(out, r)
		}
	}
	return out
}

func firstBlockReason(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	return reasons[0]
}

func buildDailyStatusMessage(st DailyTradingStatus) string {
	if st.BlockReason != "" {
		return fmt.Sprintf("blocked: %s", st.BlockReason)
	}
	if st.Execution.Ready {
		return "executionReady=true"
	}
	if st.Execution.Phase == "done" || st.Execution.Phase == "partial" {
		return fmt.Sprintf("execution=%s fills=%d orders=%d", st.Execution.Phase, st.Paper.FillCount, st.Paper.OrderCount)
	}
	return "observation only"
}
