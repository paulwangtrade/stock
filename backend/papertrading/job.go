package papertrading

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"go-stock/backend/models"

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
	// FillSession is set by Gateway after Session Policy (A/B). Empty → open/market_open.
	FillSession ExecutionSession
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

// beforeTryBeginForTest runs after run ledger create and before TryBeginExecute (tests only).
var beforeTryBeginForTest func(planID uint)

// SetBeforeTryBeginForTest installs a pre-Begin hook (nil clears). Tests only.
func SetBeforeTryBeginForTest(fn func(planID uint)) {
	beforeTryBeginForTest = fn
}

// paperTradingJob runs the Frozen Plan → PaperBroker pipeline with flag/day/idempotency guards.
// Phase10-C.7-A: owns TradePlan lifecycle Begin/Finish (TryBeginExecute / FinishPlanCAS).
// It never touches Real Execution or production paper_* tables.
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

	planRepo := data.NewTradePlanRepo()
	planID := req.PlanID
	var plan *models.TradePlan
	if planID == 0 {
		var err error
		plan, err = planRepo.GetFrozenByTradeDate(tradeDate)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// C.7: prior Track-B run may have moved plan off ready; soft-idempotent skip.
				if pid, ok := successfulRunPlanID(tradeDate); ok {
					out.PlanID = pid
					out.Status = RunStatusSkippedAlreadyRun
					out.Message = "already completed for trade_date+plan_id"
					logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s plan_id=%d", out.Status, tradeDate, pid)
					return out, nil
				}
				out.Status = RunStatusSkippedNoFrozenPlan
				out.Message = "no frozen plan for trade date"
				logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s", out.Status, tradeDate)
				return out, nil
			}
			return nil, fmt.Errorf("papertrading: load frozen plan: %w", err)
		}
		planID = plan.ID
	} else {
		var err error
		plan, err = planRepo.GetByID(planID)
		if err != nil {
			return nil, fmt.Errorf("papertrading: load plan %d: %w", planID, err)
		}
	}
	out.PlanID = planID

	// Soft idempotency: successful run already recorded for this plan/day.
	// Checked before RequireFrozenReady so C.7 terminal plans still soft-skip.
	if hasSuccessfulRun(tradeDate, planID) {
		out.Status = RunStatusSkippedAlreadyRun
		out.Message = "already completed for trade_date+plan_id"
		logger.SugaredLogger.Infof("PaperTradingJob skipped status=%s trade_date=%s plan_id=%d", out.Status, tradeDate, planID)
		return out, nil
	}

	if guard := models.RequireFrozenReadyTradePlan(plan); !guard.Allowed {
		return nil, fmt.Errorf("papertrading: execution blocked reason=%s %s", guard.Reason, guard.Message)
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

	if beforeTryBeginForTest != nil {
		beforeTryBeginForTest(planID)
	}

	// Phase10-C.7-A: ready → executing before Broker.
	okBegin, beginErr := planRepo.TryBeginExecute(planID)
	if beginErr != nil {
		now := time.Now()
		run.Status = RunStatusFailed
		run.Message = beginErr.Error()
		run.FinishedAt = &now
		_ = db.Dao.Model(run).Updates(map[string]any{
			"status":      run.Status,
			"message":     truncateMsg(run.Message, 500),
			"finished_at": now,
		}).Error
		out.Status = RunStatusFailed
		out.Message = beginErr.Error()
		return out, beginErr
	}
	if !okBegin {
		now := time.Now()
		run.Status = RunStatusSkippedPlanLifecycle
		run.Message = fmt.Sprintf("TryBeginExecute CAS miss planId=%d (not ready)", planID)
		run.FinishedAt = &now
		_ = db.Dao.Model(run).Updates(map[string]any{
			"status":      run.Status,
			"message":     truncateMsg(run.Message, 500),
			"finished_at": now,
		}).Error
		out.Status = RunStatusSkippedPlanLifecycle
		out.Message = run.Message
		logger.SugaredLogger.Warnf("PaperTradingJob skipped status=%s plan_id=%d", out.Status, planID)
		return out, nil
	}

	price := req.Price
	if price == nil {
		price = DefaultOpenPriceProvider()
	}
	broker := NewPaperBrokerForSession(price, req.FillSession)
	brokerRes, err := broker.RunForPlan(planID)
	now := time.Now()
	run.FinishedAt = &now

	if err != nil {
		finishTrackBPlanOnBrokerError(planRepo, planID, err)
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

	finishTrackBPlanAfterBroker(planRepo, planID, brokerRes)

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

func finishTrackBPlanOnBrokerError(repo *data.TradePlanRepo, planID uint, brokerErr error) {
	msg := fmt.Sprintf("track-b broker error planId=%d: %v", planID, brokerErr)
	ok, err := repo.FinishPlanCAS(planID, models.TradePlanStatusExecuting, models.TradePlanStatusFailed, msg)
	if err != nil {
		logger.SugaredLogger.Errorf("PaperTradingJob FinishPlanCAS failed plan_id=%d: %v", planID, err)
		return
	}
	if !ok {
		logger.SugaredLogger.Warnf("PaperTradingJob FinishPlanCAS miss plan_id=%d to=failed", planID)
	}
}

func finishTrackBPlanAfterBroker(repo *data.TradePlanRepo, planID uint, brokerRes *RunResult) {
	toStatus, msg := aggregateTrackBTerminalStatus(repo, planID, brokerRes)
	ok, err := repo.FinishPlanCAS(planID, models.TradePlanStatusExecuting, toStatus, msg)
	if err != nil {
		logger.SugaredLogger.Errorf("PaperTradingJob FinishPlanCAS failed plan_id=%d to=%s: %v", planID, toStatus, err)
		return
	}
	if !ok {
		logger.SugaredLogger.Warnf("PaperTradingJob FinishPlanCAS miss plan_id=%d to=%s", planID, toStatus)
	}
}

// aggregateTrackBTerminalStatus mirrors Track-A / C.7 design:
// attempted = N - S (skipped excluded); F==0 → failed; F < attempted → partial; else done.
func aggregateTrackBTerminalStatus(repo *data.TradePlanRepo, planID uint, brokerRes *RunResult) (string, string) {
	plan, err := repo.GetByID(planID)
	if err != nil || plan == nil {
		filled := 0
		if brokerRes != nil {
			filled = brokerRes.FilledCount
		}
		if filled == 0 {
			return models.TradePlanStatusFailed, fmt.Sprintf("track-b ok=0/? planId=%d (reload failed)", planID)
		}
		return models.TradePlanStatusPartial, fmt.Sprintf("track-b ok=%d/? planId=%d (reload failed)", filled, planID)
	}

	var filled, skipped, errored, pending int
	for _, it := range plan.Items {
		side := strings.ToLower(strings.TrimSpace(it.Side))
		if side != "buy" && side != "sell" {
			continue
		}
		switch it.Status {
		case models.TradePlanItemFilled:
			filled++
		case models.TradePlanItemSkipped:
			skipped++
		case models.TradePlanItemError:
			errored++
		default:
			pending++
		}
	}
	attempted := filled + errored + pending // N - S
	reject := 0
	if brokerRes != nil {
		reject = brokerRes.RejectCount
	}
	msg := fmt.Sprintf("track-b ok=%d/%d planId=%d filled=%d skipped=%d rejected=%d pending=%d",
		filled, attempted, planID, filled, skipped, reject, pending)

	if filled == 0 {
		return models.TradePlanStatusFailed, msg
	}
	if filled < attempted {
		return models.TradePlanStatusPartial, msg
	}
	return models.TradePlanStatusDone, msg
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

// successfulRunPlanID returns a plan_id that already has a successful run for tradeDate.
func successfulRunPlanID(tradeDate string) (uint, bool) {
	var run PaperSimRun
	err := db.Dao.Model(&PaperSimRun{}).
		Where("trade_date = ? AND status IN ?",
			tradeDate,
			[]string{RunStatusCompleted, RunStatusCompletedWithRejects}).
		Order("id DESC").
		First(&run).Error
	if err != nil || run.PlanID == 0 {
		return 0, false
	}
	return run.PlanID, true
}

func truncateMsg(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
