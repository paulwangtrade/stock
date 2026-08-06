package papertrading

import (
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
)

// ExecutionRequest is the Phase10-C.2 unified entry payload for track-B PaperTrading
// (paper_sim_*). Manual API, cron, and future UI must use RunExecution — not call
// the internal job directly.
type ExecutionRequest struct {
	TradeDate string // empty → today local YYYY-MM-DD
	PlanID    uint   // 0 → resolve Frozen by TradeDate
	Trigger   string // cron | manual (UI uses manual)
	Actor     string // required for manual
	Price     PriceProvider
	// SkipWeekdayCheck allows tests to run on weekends. Does NOT skip Session Policy.
	SkipWeekdayCheck bool
	// Now overrides the clock for Session Policy (tests). Zero → time.Now().
	// There is intentionally no SkipSessionPolicy flag.
	Now time.Time
}

// ExecutionResult is the gateway outcome including Session Policy metadata (C.2-B.1).
type ExecutionResult struct {
	JobResult
	// Entry identifies the unified gateway (observability / tests).
	Entry string `json:"entry"`
	// Session / Decision / Reason come from Session Policy (always set on exit).
	Session  ExecutionSession `json:"session,omitempty"`
	Decision string           `json:"decision,omitempty"`
	Reason   string           `json:"reason,omitempty"`
}

const ExecutionEntryGateway = "execution_gateway"

var (
	executionNowMu sync.RWMutex
	executionNowFn func() time.Time
)

func executionNow() time.Time {
	executionNowMu.RLock()
	fn := executionNowFn
	executionNowMu.RUnlock()
	if fn != nil {
		return fn()
	}
	return time.Now()
}

// SetExecutionNowForTest overrides the Gateway clock when ExecutionRequest.Now is zero.
// Pass nil to restore. There is no production SkipSessionPolicy.
func SetExecutionNowForTest(fn func() time.Time) {
	executionNowMu.Lock()
	defer executionNowMu.Unlock()
	executionNowFn = fn
}

// RunExecution is the sole production entry for Frozen TradePlan → paper_sim_*.
// Order: Session Policy → paperTradingJob. Session Policy cannot be bypassed.
func RunExecution(req ExecutionRequest) (*ExecutionResult, error) {
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = TriggerCron
	}
	now := req.Now
	if now.IsZero() {
		now = executionNow()
	}

	logger.SugaredLogger.Infof(
		"ExecutionGateway enter entry=%s trigger=%s actor=%s trade_date=%s plan_id=%d now=%s",
		ExecutionEntryGateway, trigger, strings.TrimSpace(req.Actor), strings.TrimSpace(req.TradeDate), req.PlanID,
		now.Format(time.RFC3339),
	)

	pol := EvaluateSessionPolicy(now)
	out := &ExecutionResult{
		Entry:    ExecutionEntryGateway,
		Session:  pol.Session,
		Decision: pol.Decision,
		Reason:   pol.Reason,
	}

	if !pol.Allow {
		out.JobResult = JobResult{
			Enabled:   IsEnabled(),
			TradeDate: strings.TrimSpace(req.TradeDate),
			PlanID:    req.PlanID,
			Trigger:   trigger,
			Actor:     strings.TrimSpace(req.Actor),
			Status:    RunStatusSkippedOutsideSession,
			Message:   pol.Reason,
		}
		if out.Actor == "" && trigger != TriggerManual {
			out.Actor = "cron"
		}
		if out.TradeDate == "" {
			out.TradeDate = now.Format("2006-01-02")
		}
		logger.SugaredLogger.Infof(
			"ExecutionGateway reject entry=%s session=%s decision=%s reason=%s",
			ExecutionEntryGateway, pol.Session, pol.Decision, pol.Reason,
		)
		return out, nil
	}

	jobRes, err := paperTradingJob(JobRequest{
		TradeDate:        req.TradeDate,
		PlanID:           req.PlanID,
		Trigger:          trigger,
		Actor:            req.Actor,
		Price:            req.Price,
		SkipWeekdayCheck: req.SkipWeekdayCheck,
	})
	if jobRes != nil {
		out.JobResult = *jobRes
	}
	logger.SugaredLogger.Infof(
		"ExecutionGateway exit entry=%s session=%s decision=%s status=%s plan_id=%d filled=%d",
		ExecutionEntryGateway, pol.Session, pol.Decision, out.Status, out.PlanID, out.FilledCount,
	)
	return out, err
}
