package approvegate

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/readiness"
)

const (
	CodeOK             = "OK"
	CodeDenied         = "APPROVE_DENIED"
	CodeCASMiss        = "CAS_MISS"
	CodeInvalidPlanID  = "INVALID_PLAN_ID"
	CodeApproveWriteOK = "APPROVED"
)

// ApproveWriteResult is the outcome of ApproveTradePlanByID (write layer).
// Business denies return nil error with OK=false; infra failures return error.
type ApproveWriteResult struct {
	OK              bool                    `json:"ok"`
	AlreadyApproved bool                    `json:"already_approved"`
	Code            string                  `json:"code"`
	Message         string                  `json:"message,omitempty"`
	Plan            *models.TradePlan       `json:"plan,omitempty"`
	Blockers        []readiness.Finding     `json:"blockers,omitempty"`
	Eligibility     *ApproveEligibilityResult `json:"eligibility,omitempty"`
}

// ApproveOptions configures eligibility injection for ApproveTradePlanByID tests.
type ApproveOptions struct {
	Eligibility *EligibilityOptions
	Now         func() time.Time
}

// ApproveTradePlanByID runs Approve Gate then CAS-writes approval metadata.
//
// Flow:
//  1. CheckApproveEligibility
//  2. AlreadyApproved → return without overwrite
//  3. Blockers / !Eligible → deny, no write
//  4. Eligible → ApproveDraftGate CAS (approved_at/by/source); status stays draft
//
// Does not modify Freeze, Execution, Strategy selection, Cron, items, Intent, or Spec.
func ApproveTradePlanByID(planID uint, actor, source string, opts *ApproveOptions) (*ApproveWriteResult, error) {
	if planID == 0 {
		return &ApproveWriteResult{
			OK:      false,
			Code:    CodeInvalidPlanID,
			Message: "plan id is required",
		}, fmt.Errorf("plan id is required")
	}
	if opts == nil {
		opts = &ApproveOptions{}
	}

	actor = strings.TrimSpace(actor)
	if actor == "" {
		actor = "system"
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = "system"
	}

	elig, err := CheckApproveEligibility(planID, opts.Eligibility)
	if err != nil {
		return nil, err
	}

	if elig.AlreadyApproved {
		plan, loadErr := loadPlanAfterWrite(planID, opts.Eligibility)
		if loadErr != nil {
			return nil, loadErr
		}
		return &ApproveWriteResult{
			OK:              false,
			AlreadyApproved: true,
			Code:            CodeAlreadyApproved,
			Message:         "plan already approved; approved_at not overwritten",
			Plan:            plan,
			Eligibility:     elig,
			Blockers:        nil,
		}, nil
	}

	if !elig.Eligible || len(elig.Blockers) > 0 {
		return &ApproveWriteResult{
			OK:          false,
			Code:        CodeDenied,
			Message:     "approve gate denied: blockers present",
			Blockers:    elig.Blockers,
			Eligibility: elig,
		}, nil
	}

	at := time.Now()
	if opts.Now != nil {
		at = opts.Now()
	}

	ok, err := data.NewTradePlanRepo().ApproveDraftGate(planID, actor, source, at)
	if err != nil {
		return nil, err
	}
	if !ok {
		plan, loadErr := loadPlanAfterWrite(planID, opts.Eligibility)
		if loadErr != nil {
			return &ApproveWriteResult{
				OK:          false,
				Code:        CodeCASMiss,
				Message:     fmt.Sprintf("approve CAS miss: plan %d", planID),
				Eligibility: elig,
			}, nil
		}
		if isApproved(plan) {
			return &ApproveWriteResult{
				OK:              false,
				AlreadyApproved: true,
				Code:            CodeAlreadyApproved,
				Message:         "approve CAS miss: plan already approved",
				Plan:            plan,
				Eligibility:     elig,
			}, nil
		}
		return &ApproveWriteResult{
			OK:          false,
			Code:        CodeCASMiss,
			Message:     fmt.Sprintf("approve CAS miss: plan %d", planID),
			Plan:        plan,
			Eligibility: elig,
		}, nil
	}

	got, err := loadPlanAfterWrite(planID, opts.Eligibility)
	if err != nil {
		return nil, err
	}
	if got.Status != models.TradePlanStatusDraft {
		return nil, fmt.Errorf("approve invariant broken: plan %d status=%s", got.ID, got.Status)
	}

	logger.SugaredLogger.Infof(
		"ApproveTradePlanByID planId=%d version=%d status=%s approvedBy=%s source=%s",
		got.ID, got.PlanVersion, got.Status, got.ApprovedBy, got.ApprovedSource,
	)

	return &ApproveWriteResult{
		OK:          true,
		Code:        CodeApproveWriteOK,
		Message:     "approved",
		Plan:        got,
		Eligibility: elig,
	}, nil
}

func loadPlanAfterWrite(planID uint, eligOpts *EligibilityOptions) (*models.TradePlan, error) {
	if eligOpts != nil && eligOpts.LoadPlan != nil {
		return eligOpts.LoadPlan(planID)
	}
	return data.NewTradePlanRepo().GetByID(planID)
}
