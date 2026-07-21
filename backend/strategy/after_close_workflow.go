package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/tradingcalendar"
)

// AfterCloseWorkflowResult is the orchestration outcome for the after-close
// Candidate → Draft → Risk path. It never includes Approve/Freeze/Execution.
type AfterCloseWorkflowResult struct {
	SourceDate      string `json:"sourceDate"`
	TradeDate       string `json:"tradeDate"`
	CandidatePoolID uint   `json:"candidatePoolId"`
	TradePlanID     uint   `json:"tradePlanId"`
	PlanVersion     int    `json:"planVersion"`
	RiskPassed      bool   `json:"riskPassed"`
	OK              bool   `json:"ok"`
	FailedStep      string `json:"failedStep,omitempty"`
	Message         string `json:"message,omitempty"`
}

type draftBuildFunc func(*models.CandidatePool) (*models.TradePlan, error)
type riskEvalFunc func(*models.TradePlan) (*RiskProposalResult, error)

// AfterClosePlanWorkflow orchestrates after-close Candidate → Draft → Risk.
type AfterClosePlanWorkflow struct {
	Calendar   tradingcalendar.Calendar
	buildPool  candidatePoolBuildFunc
	buildDraft draftBuildFunc
	evalRisk   riskEvalFunc
}

// NewAfterClosePlanWorkflow creates a workflow using production builders.
func NewAfterClosePlanWorkflow() *AfterClosePlanWorkflow {
	return &AfterClosePlanWorkflow{Calendar: tradingcalendar.Default}
}

// RunAfterClosePlanWorkflow is the default-calendar entry point.
func RunAfterClosePlanWorkflow(sourceDate string) (*AfterCloseWorkflowResult, error) {
	return NewAfterClosePlanWorkflow().Run(sourceDate)
}

// Run executes:
//  1. NextTradingDay(sourceDate) → TradeDate
//  2. BuildCandidatePool(TradeDate) with after_close config
//  3. BuildDraftTradePlanFromCandidatePool
//  4. EvaluateDraftTradePlanRisk
//
// It does not call ApproveTradePlan, FreezeTradePlan, Execution, or the 9:20 pipeline.
func (w *AfterClosePlanWorkflow) Run(sourceDate string) (*AfterCloseWorkflowResult, error) {
	sourceDate = strings.TrimSpace(sourceDate)
	if sourceDate == "" {
		sourceDate = todayTradeDate()
	}

	out := &AfterCloseWorkflowResult{SourceDate: sourceDate}

	tradeDate, err := w.Calendar.NextTradingDayString(sourceDate)
	if err != nil {
		out.FailedStep = "calendar"
		out.Message = err.Error()
		return out, err
	}
	out.TradeDate = tradeDate

	buildPool := w.buildPool
	if buildPool == nil {
		buildPool = BuildCandidatePool
	}
	buildDraft := w.buildDraft
	if buildDraft == nil {
		buildDraft = BuildDraftTradePlanFromCandidatePool
	}
	evalRisk := w.evalRisk
	if evalRisk == nil {
		evalRisk = EvaluateDraftTradePlanRisk
	}

	pool, err := buildPool(tradeDate, WithCandidatePoolConfig(map[string]any{
		"session":     candidatePoolSessionAfterClose,
		"source_date": sourceDate,
	}))
	if err != nil {
		out.FailedStep = "candidate"
		out.Message = err.Error()
		return out, err
	}
	if pool == nil {
		out.FailedStep = "candidate"
		out.Message = "candidate pool is nil"
		return out, fmt.Errorf("candidate pool is nil")
	}
	if pool.Status != models.CandidatePoolStatusReady || pool.ItemCount == 0 && len(pool.Items) == 0 {
		out.CandidatePoolID = pool.ID
		out.FailedStep = "candidate"
		out.Message = fmt.Sprintf("candidate pool %d not ready or empty: %s", pool.ID, pool.Message)
		return out, fmt.Errorf("%s", out.Message)
	}
	out.CandidatePoolID = pool.ID

	draft, err := buildDraft(pool)
	if err != nil {
		out.FailedStep = "draft"
		out.Message = err.Error()
		return out, err
	}
	if draft == nil {
		out.FailedStep = "draft"
		out.Message = "draft trade plan is nil"
		return out, fmt.Errorf("draft trade plan is nil")
	}
	out.TradePlanID = draft.ID
	out.PlanVersion = draft.PlanVersion

	riskResult, err := evalRisk(draft)
	if err != nil {
		out.FailedStep = "risk"
		out.Message = err.Error()
		return out, err
	}
	if riskResult == nil {
		out.FailedStep = "risk"
		out.Message = "risk proposal result is nil"
		return out, fmt.Errorf("risk proposal result is nil")
	}
	out.RiskPassed = riskResult.Passed
	if !riskResult.Passed {
		out.OK = false
		out.FailedStep = "risk"
		out.Message = "risk proposal did not pass"
		logger.SugaredLogger.Warnf(
			"RunAfterClosePlanWorkflow source=%s tradeDate=%s poolId=%d planId=%d version=%d riskPassed=false",
			sourceDate, tradeDate, out.CandidatePoolID, out.TradePlanID, out.PlanVersion,
		)
		return out, nil
	}

	out.OK = true
	out.FailedStep = ""
	out.Message = "after-close workflow completed"
	logger.SugaredLogger.Infof(
		"RunAfterClosePlanWorkflow source=%s tradeDate=%s poolId=%d planId=%d version=%d riskPassed=true",
		sourceDate, tradeDate, out.CandidatePoolID, out.TradePlanID, out.PlanVersion,
	)
	return out, nil
}
