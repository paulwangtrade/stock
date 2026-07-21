package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
)

// GetFrozenTradePlan is the Phase6-A5.1 Frozen Plan Consumer entry.
// It returns the frozen ready TradePlan for tradeDate (Status=ready, FreezeAt≠nil).
//
// It does not create CandidatePool/TradePlan, does not mutate plan status,
// does not call Execution, and does not alter the 9:20 pipeline.
func GetFrozenTradePlan(tradeDate string) (*models.TradePlan, error) {
	tradeDate = strings.TrimSpace(tradeDate)
	if tradeDate == "" {
		return nil, fmt.Errorf("trade date is required")
	}

	plan, err := data.NewTradePlanRepo().GetFrozenByTradeDate(tradeDate)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("frozen trade plan not found for %s", tradeDate)
	}
	// Defense in depth: zero-valued freeze_at must not count as frozen.
	if !plan.IsFrozen() {
		return nil, fmt.Errorf("frozen trade plan not found for %s (ready without freeze)", tradeDate)
	}

	logger.SugaredLogger.Infof(
		"GetFrozenTradePlan date=%s planId=%d version=%d sourceSession=%s items=%d",
		tradeDate, plan.ID, plan.PlanVersion, plan.SourceSession, len(plan.Items),
	)
	return plan, nil
}
