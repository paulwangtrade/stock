package projection

import (
	"strings"

	"go-stock/backend/models"
)

func mapDecisionStatus(item *models.TradePlanItem, inPool bool) string {
	if item == nil {
		if inPool {
			return DecisionStatusWatch
		}
		return DecisionStatusUnknown
	}
	status := strings.ToLower(strings.TrimSpace(item.Status))
	switch status {
	case models.TradePlanItemPending:
		return DecisionStatusBuyCandidate
	case models.TradePlanItemFilled:
		return DecisionStatusBuyCandidate
	case models.TradePlanItemSkipped:
		code := strings.TrimSpace(item.RiskCode)
		if code != "" && !strings.EqualFold(code, "approved") {
			return DecisionStatusReject
		}
		if inPool {
			return DecisionStatusWatch
		}
		return DecisionStatusNotInPlan
	default:
		if inPool {
			return DecisionStatusWatch
		}
		return DecisionStatusUnknown
	}
}

func mapCandidateStatus(inPool bool, item *models.TradePlanItem) string {
	if item == nil {
		if inPool {
			return CandidateStatusInPool
		}
		return CandidateStatusNotInPool
	}
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case models.TradePlanItemPending:
		return CandidateStatusPlanPending
	case models.TradePlanItemFilled:
		return CandidateStatusPlanFilled
	case models.TradePlanItemSkipped:
		return CandidateStatusPlanSkipped
	default:
		if inPool {
			return CandidateStatusInPool
		}
		return CandidateStatusNotInPool
	}
}
