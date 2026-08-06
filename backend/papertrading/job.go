package papertrading

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JobRequest configures a PaperTradingJob invocation.
type JobRequest struct {
	TradeDate string // empty → today local YYYY-MM-DD
	PlanID    uint   // 0 → resolve Frozen by TradeDate
	Trigger   string // cron | manual
	Actor     string // required for manual
	Price     PriceProvider
	// SkipWeekdayCheck allows tests to run on weekends.
	SkipWeekdayCheck bool
}

// JobResult is the observable outcome of PaperTradingJob.
type JobResult struct {
	ExecutionID    string     `json:"executionId"`
	Enabled        bool       `json:"enabled"`
	TradeDate      string     `json:"tradeDate"`
	PlanID         uint       `json:"planId"`
	Trigger        string     `json:"trigger"`
	Actor          string     `json:"actor"`
	Status         string     `json:"status"`
	Message        string     `json:"message"`
	OrdersTotal    int        `json:"ordersTotal"`
	FilledCount    int        `json:"filledCount"`
	RejectCount    int        `json:"rejectCount"`
	SkippedAlready int        `json:"skippedAlready"`
	AccountID      uint       `json:"accountId"`
	Broker         *RunResult `json:"broker,omitempty"`
}

// paperTradingJob runs the Frozen Plan → PaperBroker pipeline with flag/day/idempotency guards.
// It never touches Real Execution, trade_plan lifecycle writes, or production paper_* tables.
//
// Production / API / cron / UI MUST call RunExecution (Execution Gateway). Do not invoke
// this function from outside the papertrading package.
func paperTradingJob(req JobRequest) (*JobResult, error) {
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = TriggerCron
	}
	actor := strings.TrimSpace(req.Actor)
	if trigger == TriggerManual && actor == "" {
		return nil, fmt.Errorf("papertrading: actor is required for manual trigger")
	}
	if actor == "" {
		actor = "cron"
	}

	tradeDate := strings.TrimSpace(req.TradeDate)
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}

	execID := uuid.NewString()
	out := &JobResult{
		ExecutionID: execID,
		Enabled:     IsEnabled(),
		TradeDate:   tradeDate,
		Trigger:     trigger,
		Actor:       actor,
	}

	if !IsEnabled() {
		out.Status = RunStatusSkippedDisabled
		out.Message = "enablePaperTrading=false"
		logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s trigger=%s actor=%s",
			out.Status, tradeDate, trigger, actor)
		return out, nil
	}

	if !req.SkipWeekdayCheck && !data.IsWeekdayLocal(time.Now()) {
		out.Status = RunStatusSkippedNonTradingDay
		out.Message = "non trading day"
		logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s", out.Status, tradeDate)
		return out, nil
	}

	if db.Dao == nil {
		return nil, fmt.Errorf("papertrading: db not initialized")
	}
	if err := EnsureSchema(db.Dao); err != nil {
		return nil, err
	}

	planID := req.PlanID
	if planID == 0 {
		plan, err := data.NewTradePlanRepo().GetFrozenByTradeDate(tradeDate)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				out.Status = RunStatusSkippedNoFrozenPlan
				out.Message = "no frozen plan for trade date"
				logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s", out.Status, tradeDate)
				return out, nil
			}
			return nil, fmt.Errorf("papertrading: load frozen plan: %w", err)
		}
		planID = plan.ID
	}
	out.PlanID = planID

	// Soft idempotency: successful run already recorded for this plan/day.
	if hasSuccessfulRun(tradeDate, planID) {
		out.Status = RunStatusSkippedAlreadyRun
		out.Message = "already completed for trade_date+plan_id"
		logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s plan_id=%d", out.Status, tradeDate, planID)
		return out, nil
	}

	run := &PaperSimRun{
		ExecutionID: execID,
		TradeDate:   tradeDate,
		PlanID:      planID,
		Trigger:     trigger,
		Actor:       actor,
		Status:      RunStatusRunning,
		StartedAt:   time.Now(),
	}
	if err := db.Dao.Create(run).Error; err != nil {
		return nil, fmt.Errorf("papertrading: create run ledger: %w", err)
	}

	price := req.Price
	if price == nil {
		price = DefaultOpenPriceProvider()
	}
	broker := NewPaperBroker(price)
	brokerRes, err := broker.RunForPlan(planID)
	now := time.Now()
	run.FinishedAt = &now

	if err != nil {
		run.Status = RunStatusFailed
		run.Message = err.Error()
		_ = db.Dao.Model(run).Updates(map[string]any{
			"status":      run.Status,
			"message":     truncateMsg(run.Message, 500),
			"finished_at": now,
		}).Error
		out.Status = RunStatusFailed
		out.Message = err.Error()
		logger.SugaredLogger.Errorf("PaperTradingJob failed execution_id=%s plan_id=%d err=%v", execID, planID, err)
		return out, err
	}

	out.Broker = brokerRes
	out.OrdersTotal = brokerRes.OrdersTotal
	out.FilledCount = brokerRes.FilledCount
	out.RejectCount = brokerRes.RejectCount
	out.SkippedAlready = brokerRes.SkippedAlready
	out.AccountID = brokerRes.AccountID

	status := RunStatusCompleted
	if brokerRes.RejectCount > 0 {
		status = RunStatusCompletedWithRejects
	}
	// Pure re-hit of already-processed items still counts as completed (idempotent).
	if brokerRes.OrdersTotal == 0 && brokerRes.SkippedAlready > 0 {
		status = RunStatusCompleted
	}
	run.Status = status
	run.Message = brokerRes.Message
	run.OrdersTotal = brokerRes.OrdersTotal
	run.FilledCount = brokerRes.FilledCount
	run.RejectCount = brokerRes.RejectCount
	run.SkippedAlready = brokerRes.SkippedAlready
	run.AccountID = brokerRes.AccountID
	_ = db.Dao.Model(run).Updates(map[string]any{
		"status":          status,
		"message":         truncateMsg(brokerRes.Message, 500),
		"orders_total":    brokerRes.OrdersTotal,
		"filled_count":    brokerRes.FilledCount,
		"reject_count":    brokerRes.RejectCount,
		"skipped_already": brokerRes.SkippedAlready,
		"account_id":      brokerRes.AccountID,
		"finished_at":     now,
	}).Error

	out.Status = status
	out.Message = brokerRes.Message
	logger.SugaredLogger.Infof(
		"PaperTradingJob done execution_id=%s trade_date=%s plan_id=%d trigger=%s actor=%s status=%s %s",
		execID, tradeDate, planID, trigger, actor, status, brokerRes.Message,
	)
	return out, nil
}

func hasSuccessfulRun(tradeDate string, planID uint) bool {
	var n int64
	err := db.Dao.Model(&PaperSimRun{}).
		Where("trade_date = ? AND plan_id = ? AND status IN ?",
			tradeDate, planID,
			[]string{RunStatusCompleted, RunStatusCompletedWithRejects}).
		Count(&n).Error
	return err == nil && n > 0
}

func truncateMsg(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
