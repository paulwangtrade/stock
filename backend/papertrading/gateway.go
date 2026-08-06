package papertrading

import (
	"strings"

	"go-stock/backend/logger"
)

// ExecutionRequest is the Phase10-C.2 unified entry payload for track-B PaperTrading
// (paper_sim_*). Manual API, cron, and future UI must use RunExecution — not call
// the internal job directly.
//
// C.2-A: passthrough to the existing job (no A/B/C session policy yet).
type ExecutionRequest struct {
	TradeDate string // empty → today local YYYY-MM-DD
	PlanID    uint   // 0 → resolve Frozen by TradeDate
	Trigger   string // cron | manual (UI uses manual)
	Actor     string // required for manual
	Price     PriceProvider
	// SkipWeekdayCheck allows tests to run on weekends. Does NOT skip session
	// policy (session policy lands in C.2-B).
	SkipWeekdayCheck bool
}

// ExecutionResult is the gateway outcome. C.2-A embeds JobResult; later slices may
// attach session / price_mode / decision without breaking callers.
type ExecutionResult struct {
	JobResult
	// Entry identifies the unified gateway (observability / tests).
	Entry string `json:"entry"`
}

const ExecutionEntryGateway = "execution_gateway"

// RunExecution is the sole production entry for Frozen TradePlan → paper_sim_*.
// It never writes production paper_* tables.
func RunExecution(req ExecutionRequest) (*ExecutionResult, error) {
	trigger := strings.TrimSpace(req.Trigger)
	if trigger == "" {
		trigger = TriggerCron
	}
	logger.SugaredLogger.Infof(
		"ExecutionGateway enter entry=%s trigger=%s actor=%s trade_date=%s plan_id=%d",
		ExecutionEntryGateway, trigger, strings.TrimSpace(req.Actor), strings.TrimSpace(req.TradeDate), req.PlanID,
	)

	jobRes, err := paperTradingJob(JobRequest{
		TradeDate:        req.TradeDate,
		PlanID:           req.PlanID,
		Trigger:          trigger,
		Actor:            req.Actor,
		Price:            req.Price,
		SkipWeekdayCheck: req.SkipWeekdayCheck,
	})
	out := &ExecutionResult{Entry: ExecutionEntryGateway}
	if jobRes != nil {
		out.JobResult = *jobRes
		logger.SugaredLogger.Infof(
			"ExecutionGateway exit entry=%s status=%s plan_id=%d filled=%d",
			ExecutionEntryGateway, jobRes.Status, jobRes.PlanID, jobRes.FilledCount,
		)
	}
	return out, err
}
