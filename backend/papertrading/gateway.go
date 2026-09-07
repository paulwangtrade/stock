package papertrading

import (
	"strings"
	"sync"
	"time"

	"go-stock/backend/logger"
	"go-stock/backend/tradingevent"
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
	// PriceMode is set after Session allow (realtime | close | none).
	PriceMode string `json:"priceMode,omitempty"`
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
// Order: Session Policy → SelectFillProvider → paperTradingJob.
// Session Policy cannot be bypassed.
func RunExecution(req ExecutionRequest) (*ExecutionResult, error) {
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = TriggerCron
	}
	now := req.Now
	if now.IsZero() {
		now = executionNow()
	}

	tradeDate := strings.TrimSpace(req.TradeDate)
	if tradeDate == "" {
		tradeDate = now.Format("2006-01-02")
	}

	logger.SugaredLogger.Infof(
		"ExecutionGateway enter entry=%s trigger=%s actor=%s trade_date=%s plan_id=%d now=%s",
		ExecutionEntryGateway, trigger, strings.TrimSpace(req.Actor), tradeDate, req.PlanID,
		now.Format(time.RFC3339),
	)

	pol := EvaluateSessionPolicy(now)
	out := &ExecutionResult{
		Entry:    ExecutionEntryGateway,
		Session:  pol.Session,
		Decision: pol.Decision,
		Reason:   pol.Reason,
	}

	// Observation v0: STARTED after session resolve; does not affect allow/deny.
	tradingevent.EmitExecution(
		tradingevent.EventExecutionStarted, tradeDate, string(pol.Session), tradingevent.StatusPass, "GATEWAY_ENTER",
		req.PlanID, now, "",
	)

	if !pol.Allow {
		out.PriceMode = PriceModeNone
		out.JobResult = JobResult{
			Enabled:   IsEnabled(),
			TradeDate: tradeDate,
			PlanID:    req.PlanID,
			Trigger:   trigger,
			Actor:     strings.TrimSpace(req.Actor),
			Status:    RunStatusSkippedOutsideSession,
			Message:   pol.Reason,
		}
		if out.Actor == "" && trigger != TriggerManual {
			out.Actor = "cron"
		}
		logger.SugaredLogger.Infof(
			"ExecutionGateway reject entry=%s session=%s decision=%s reason=%s",
			ExecutionEntryGateway, pol.Session, pol.Decision, pol.Reason,
		)
		tradingevent.EmitExecution(
			tradingevent.EventExecutionSkipped, out.TradeDate, string(pol.Session), tradingevent.StatusSkip, pol.Reason,
			out.PlanID, now, "",
		)
		return out, nil
	}

	price := SelectFillProvider(pol.Session, req.Price)
	out.PriceMode = priceModeForSession(pol.Session)

	jobRes, err := paperTradingJob(JobRequest{
		TradeDate:        req.TradeDate,
		PlanID:           req.PlanID,
		Trigger:          trigger,
		Actor:            req.Actor,
		Price:            price,
		FillSession:      pol.Session,
		SkipWeekdayCheck: req.SkipWeekdayCheck,
	})
	if jobRes != nil {
		out.JobResult = *jobRes
	}
	// Preserve request identity for observation when job returns nil/partial.
	if out.TradeDate == "" {
		out.TradeDate = tradeDate
	}
	if out.PlanID == 0 {
		out.PlanID = req.PlanID
	}
	logger.SugaredLogger.Infof(
		"ExecutionGateway exit entry=%s session=%s decision=%s price_mode=%s status=%s plan_id=%d filled=%d",
		ExecutionEntryGateway, pol.Session, pol.Decision, out.PriceMode, out.Status, out.PlanID, out.FilledCount,
	)
	emitExecutionExit(out, err, now)
	return out, err
}

// emitExecutionExit maps Gateway exit to EXECUTION_* (observation only).
func emitExecutionExit(out *ExecutionResult, err error, now time.Time) {
	if out == nil {
		return
	}
	planID := out.PlanID
	tradeDate := out.TradeDate
	if tradeDate == "" {
		tradeDate = now.Format("2006-01-02")
	}
	session := string(out.Session)
	execID := strings.TrimSpace(out.ExecutionID)

	if err != nil {
		tradingevent.EmitExecution(
			tradingevent.EventExecutionFailed, tradeDate, session, tradingevent.StatusFail, err.Error(), planID, now, execID,
		)
		return
	}
	reason := strings.TrimSpace(out.Reason)
	if reason == "" {
		reason = strings.TrimSpace(out.Message)
	}
	switch out.Status {
	case RunStatusFailed:
		if reason == "" {
			reason = out.Status
		}
		tradingevent.EmitExecution(
			tradingevent.EventExecutionFailed, tradeDate, session, tradingevent.StatusFail, reason, planID, now, execID,
		)
	case RunStatusSkippedOutsideSession, RunStatusSkippedDisabled, RunStatusSkippedNonTradingDay,
		RunStatusSkippedNoFrozenPlan, RunStatusSkippedAlreadyRun, RunStatusSkippedPlanLifecycle:
		if reason == "" {
			reason = out.Status
		}
		tradingevent.EmitExecution(
			tradingevent.EventExecutionSkipped, tradeDate, session, tradingevent.StatusSkip, reason, planID, now, execID,
		)
	default:
		if reason == "" {
			reason = out.Status
		}
		tradingevent.EmitExecution(
			tradingevent.EventExecutionCompleted, tradeDate, session, tradingevent.StatusPass, reason, planID, now, execID,
		)
	}
}
