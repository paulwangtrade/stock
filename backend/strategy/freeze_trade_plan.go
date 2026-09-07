package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/tradingconfig"
)

// FreezeTradePlan promotes an approved Draft TradePlan to ready and writes freeze
// audit fields (FreezeAt / FreezeBy / FreezeReason). It does not introduce a
// frozen status string and does not touch Execution / cron / Trading Gate / 9:20.
func FreezeTradePlan(plan *models.TradePlan, freezeBy, freezeReason string) (*models.TradePlan, error) {
	if plan == nil {
		return nil, fmt.Errorf("trade plan is nil")
	}
	return FreezeTradePlanAt(plan.ID, freezeBy, freezeReason, time.Now())
}

// FreezeTradePlanAt is the clock-injectable freeze entry for Simulator and tests.
// Production FreezeTradePlan keeps wall-clock semantics (time.Now()).
// Zero freezeTime falls back to time.Now().
func FreezeTradePlanAt(planID uint, freezeBy, freezeReason string, freezeTime time.Time) (*models.TradePlan, error) {
	if planID == 0 {
		return nil, fmt.Errorf("trade plan id is required")
	}

	repo := data.NewTradePlanRepo()
	current, err := repo.GetByID(planID)
	if err != nil {
		return nil, err
	}

	// Idempotent: already frozen → return as-is.
	if current.IsFrozen() {
		logger.SugaredLogger.Infof("FreezeTradePlan planId=%d already frozen (idempotent)", current.ID)
		return current, nil
	}

	if !current.IsDraft() {
		return nil, fmt.Errorf("trade plan %d status=%s, want draft", current.ID, current.Status)
	}
	if current.ApprovedAt == nil || current.ApprovedAt.IsZero() {
		return nil, fmt.Errorf("freeze denied for plan %d: ApprovedAt is nil", current.ID)
	}

	freezeBy = strings.TrimSpace(freezeBy)
	if freezeBy == "" {
		freezeBy = "system"
	}
	freezeReason = strings.TrimSpace(freezeReason)

	at := freezeTime
	if at.IsZero() {
		at = time.Now()
	}
	ok, err := repo.PromoteDraftToFrozen(current.ID, freezeBy, freezeReason, at, tradingconfig.Default().EnablePaperOpenBuy())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("freeze CAS miss: plan %d", current.ID)
	}

	got, err := repo.GetByID(current.ID)
	if err != nil {
		return nil, err
	}
	if !got.IsFrozen() {
		return nil, fmt.Errorf("freeze invariant broken: plan %d IsFrozen=false status=%s", got.ID, got.Status)
	}

	logger.SugaredLogger.Infof(
		"FreezeTradePlan planId=%d version=%d status=%s freezeBy=%s reason=%q",
		got.ID, got.PlanVersion, got.Status, got.FreezeBy, got.FreezeReason,
	)
	return got, nil
}
