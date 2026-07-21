package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// ApproveTradePlan approves a Draft TradePlan after a passing Risk Proposal.
// It writes ApprovedAt / ApprovedBy / ApprovalReason and keeps Status=draft.
// It does not Freeze, promote to ready, or touch Execution / cron / Trading Gate.
func ApproveTradePlan(plan *models.TradePlan, approvedBy, approvalReason string) (*models.TradePlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("trade plan is nil")
	}
	if plan.ID == 0 {
		return nil, fmt.Errorf("trade plan id is required")
	}
	if !plan.IsDraft() {
		return nil, fmt.Errorf("trade plan %d status=%s, want draft", plan.ID, plan.Status)
	}
	if len(plan.Items) == 0 {
		return nil, fmt.Errorf("trade plan %d has no items", plan.ID)
	}

	approvedBy = strings.TrimSpace(approvedBy)
	if approvedBy == "" {
		approvedBy = "system"
	}
	approvalReason = strings.TrimSpace(approvalReason)

	riskResult, err := EvaluateDraftTradePlanRisk(plan)
	if err != nil {
		return nil, fmt.Errorf("risk proposal evaluation failed: %w", err)
	}
	if riskResult == nil || !riskResult.Passed {
		reason := "risk proposal did not pass"
		if riskResult != nil && len(riskResult.RiskReasons) > 0 {
			reason = reason + ": " + strings.Join(riskResult.RiskReasons, "; ")
		} else if riskResult != nil && riskResult.PlanFilterResult != nil {
			reason = reason + ": status=" + riskResult.PlanFilterResult.RiskStatus
		}
		return nil, fmt.Errorf("approve denied for plan %d: %s", plan.ID, reason)
	}

	at := time.Now()
	ok, err := data.NewTradePlanRepo().ApproveDraft(plan.ID, approvedBy, approvalReason, at)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("approve CAS miss: plan %d is not draft", plan.ID)
	}

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	if err != nil {
		return nil, err
	}
	if got.Status != models.TradePlanStatusDraft {
		return nil, fmt.Errorf("approve invariant broken: plan %d status=%s", got.ID, got.Status)
	}

	logger.SugaredLogger.Infof(
		"ApproveTradePlan planId=%d version=%d status=%s approvedBy=%s reason=%q",
		got.ID, got.PlanVersion, got.Status, got.ApprovedBy, got.ApprovalReason,
	)
	return got, nil
}
