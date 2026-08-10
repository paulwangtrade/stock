package approvegate

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"
)

const (
	RuleApproveGate = "AG-GATE"
	RuleApproveMeta = "AG-META"
	RuleApproveRisk = "AG-RISK"

	CodeNotDraft       = "NOT_DRAFT"
	CodeNoItems        = "NO_ITEMS"
	CodeRiskNotPassed  = "RISK_NOT_PASSED"
	CodeReadinessBlock = "READINESS_NOT_READY"
	CodeRiskEvalError  = "RISK_EVAL_ERROR"
	CodeAlreadyApproved = "ALREADY_APPROVED"
)

// ApproveEligibilityResult is the read-only Approve Gate eligibility report.
type ApproveEligibilityResult struct {
	PlanID          uint                                       `json:"plan_id"`
	Eligible        bool                                       `json:"eligible"`
	AlreadyApproved bool                                       `json:"already_approved"`
	Blockers        []readiness.Finding                        `json:"blockers"`
	Warnings        []readiness.Finding                        `json:"warnings"`
	RiskResult      *strategy.RiskProposalResult                 `json:"risk_result,omitempty"`
	ReadinessResult *readiness.ExecutionIntentReadinessResult    `json:"readiness_result,omitempty"`
	CheckedAt       time.Time                                  `json:"checked_at"`
}

// EligibilityOptions configures plan loading and injectable evaluators (tests).
type EligibilityOptions struct {
	ReadinessOpts *readiness.Options
	LoadPlan      func(planID uint) (*models.TradePlan, error)
	EvaluateRisk  func(plan *models.TradePlan) (*strategy.RiskProposalResult, error)
	EvaluateReadiness func(plan *models.TradePlan, opts *readiness.Options) readiness.ExecutionIntentReadinessResult
}

// CheckApproveEligibility loads a TradePlan and decides whether Approve Gate would allow approval.
// Eligible iff structural checks pass, Risk.Passed, and Readiness.Ready (len blockers == 0).
// It does not write approval fields, change status, or touch Execution / Freeze.
func CheckApproveEligibility(planID uint, opts *EligibilityOptions) (*ApproveEligibilityResult, error) {
	if planID == 0 {
		return nil, fmt.Errorf("plan id is required")
	}
	if opts == nil {
		opts = &EligibilityOptions{}
	}

	plan, err := loadPlan(planID, opts.LoadPlan)
	if err != nil {
		return nil, err
	}

	out := &ApproveEligibilityResult{
		PlanID:          plan.ID,
		AlreadyApproved: isApproved(plan),
		Blockers:        make([]readiness.Finding, 0),
		Warnings:        make([]readiness.Finding, 0),
		CheckedAt:       time.Now(),
	}

	if out.AlreadyApproved {
		out.Warnings = append(out.Warnings, readiness.Finding{
			RuleCode: RuleApproveMeta,
			Code:     CodeAlreadyApproved,
			Severity: readiness.SeverityWarn,
			Message:  "plan already has approval metadata; repeat approve will not overwrite approved_at",
		})
	}

	appendStructuralBlockers(plan, &out.Blockers)

	riskResult, riskBlockers := evaluateRisk(plan, opts.EvaluateRisk)
	out.RiskResult = riskResult
	out.Blockers = append(out.Blockers, riskBlockers...)

	rd := evaluateReadiness(plan, opts)
	out.ReadinessResult = &rd
	out.Warnings = append(out.Warnings, rd.Warnings...)
	if !rd.Ready {
		out.Blockers = append(out.Blockers, readinessBlockersFromResult(&rd)...)
	}

	out.Eligible = len(out.Blockers) == 0 &&
		riskResult != nil && riskResult.Passed &&
		rd.Ready
	return out, nil
}

func loadPlan(planID uint, loader func(uint) (*models.TradePlan, error)) (*models.TradePlan, error) {
	if loader != nil {
		return loader(planID)
	}
	return data.NewTradePlanRepo().GetByID(planID)
}

func evaluateRisk(
	plan *models.TradePlan,
	eval func(*models.TradePlan) (*strategy.RiskProposalResult, error),
) (*strategy.RiskProposalResult, []readiness.Finding) {
	if eval != nil {
		res, err := eval(plan)
		if err != nil {
			return nil, []readiness.Finding{{
				RuleCode: RuleApproveRisk,
				Code:     CodeRiskEvalError,
				Severity: readiness.SeverityBlock,
				Message:  err.Error(),
			}}
		}
		return res, riskBlockers(res)
	}

	res, err := strategy.EvaluateDraftTradePlanRisk(plan)
	if err != nil {
		return nil, []readiness.Finding{{
			RuleCode: RuleApproveRisk,
			Code:     CodeRiskEvalError,
			Severity: readiness.SeverityBlock,
			Message:  err.Error(),
		}}
	}
	return res, riskBlockers(res)
}

func evaluateReadiness(plan *models.TradePlan, opts *EligibilityOptions) readiness.ExecutionIntentReadinessResult {
	if opts != nil && opts.EvaluateReadiness != nil {
		return opts.EvaluateReadiness(plan, opts.ReadinessOpts)
	}
	return readiness.EvaluateExecutionIntentReadiness(plan, opts.ReadinessOpts)
}

func appendStructuralBlockers(plan *models.TradePlan, blockers *[]readiness.Finding) {
	if plan == nil {
		*blockers = append(*blockers, readiness.Finding{
			RuleCode: RuleApproveMeta,
			Code:     "PLAN_NIL",
			Severity: readiness.SeverityBlock,
			Message:  "trade plan is nil",
		})
		return
	}
	if !plan.IsDraft() {
		*blockers = append(*blockers, readiness.Finding{
			RuleCode: RuleApproveMeta,
			Code:     CodeNotDraft,
			Severity: readiness.SeverityBlock,
			Message:  fmt.Sprintf("trade plan %d status=%s, want draft", plan.ID, plan.Status),
		})
	}
	if len(plan.Items) == 0 {
		*blockers = append(*blockers, readiness.Finding{
			RuleCode: RuleApproveMeta,
			Code:     CodeNoItems,
			Severity: readiness.SeverityBlock,
			Message:  fmt.Sprintf("trade plan %d has no items", plan.ID),
		})
	}
}

func riskBlockers(res *strategy.RiskProposalResult) []readiness.Finding {
	if res == nil || res.Passed {
		return nil
	}
	out := make([]readiness.Finding, 0, len(res.RiskReasons)+1)
	for _, reason := range res.RiskReasons {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			continue
		}
		out = append(out, readiness.Finding{
			RuleCode: RuleApproveRisk,
			Code:     CodeRiskNotPassed,
			Severity: readiness.SeverityBlock,
			Message:  reason,
		})
	}
	if len(out) == 0 {
		out = append(out, readiness.Finding{
			RuleCode: RuleApproveRisk,
			Code:     CodeRiskNotPassed,
			Severity: readiness.SeverityBlock,
			Message:  "risk proposal did not pass",
		})
	}
	return out
}

func readinessBlockersFromResult(rd *readiness.ExecutionIntentReadinessResult) []readiness.Finding {
	if rd == nil || rd.Ready || len(rd.Blockers) == 0 {
		return nil
	}
	out := make([]readiness.Finding, 0, len(rd.Blockers))
	for _, b := range rd.Blockers {
		out = append(out, readiness.Finding{
			RuleCode: b.RuleCode,
			Code:     b.Code,
			Severity: b.Severity,
			Message:  b.Message,
			Evidence: b.Evidence,
		})
	}
	if len(out) == 0 {
		out = append(out, readiness.Finding{
			RuleCode: RuleApproveGate,
			Code:     CodeReadinessBlock,
			Severity: readiness.SeverityBlock,
			Message:  "readiness not ready",
		})
	}
	return out
}

func isApproved(plan *models.TradePlan) bool {
	return plan != nil && plan.ApprovedAt != nil && !plan.ApprovedAt.IsZero()
}
