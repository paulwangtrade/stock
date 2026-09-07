package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequireFrozenReadyTradePlan_FrozenReadyPass(t *testing.T) {
	now := time.Now()
	plan := &TradePlan{
		ID:         1,
		Status:     TradePlanStatusReady,
		FreezeAt:   &now,
		ApprovedAt: &now,
		TradeDate:  "2026-07-22",
	}
	got := RequireFrozenReadyTradePlan(plan)
	require.True(t, got.Allowed)
	require.Empty(t, got.Reason)
	require.True(t, plan.IsFrozen())
}

func TestRequireFrozenReadyTradePlan_NakedReadyBlock(t *testing.T) {
	plan := &TradePlan{
		ID:        2,
		Status:    TradePlanStatusReady,
		FreezeAt:  nil,
		TradeDate: "2026-07-22",
	}
	got := RequireFrozenReadyTradePlan(plan)
	require.False(t, got.Allowed)
	require.Equal(t, ReasonPlanNotFrozen, got.Reason)
	require.Equal(t, "PLAN_NOT_FROZEN", got.Reason)
	require.False(t, plan.IsFrozen())
	require.Contains(t, got.Message, "INV-P-RDY-01")
}

func TestRequireFrozenReadyTradePlan_ReadyFreezeWithoutApproveBlock(t *testing.T) {
	now := time.Now()
	plan := &TradePlan{
		ID:        5,
		Status:    TradePlanStatusReady,
		FreezeAt:  &now,
		ApprovedAt: nil,
		TradeDate: "2026-07-22",
	}
	require.True(t, plan.IsFrozen(), "IsFrozen ignores ApprovedAt")
	got := RequireFrozenReadyTradePlan(plan)
	require.False(t, got.Allowed)
	require.Equal(t, ReasonPlanNotApproved, got.Reason)
	require.Contains(t, got.Message, "ApprovedAt")
}

func TestRequireFrozenReadyTradePlan_DraftBlock(t *testing.T) {
	plan := &TradePlan{
		ID:     3,
		Status: TradePlanStatusDraft,
	}
	got := RequireFrozenReadyTradePlan(plan)
	require.False(t, got.Allowed)
	require.Equal(t, ReasonPlanNotFrozen, got.Reason)
}

func TestRequireFrozenReadyTradePlan_FailedBlock(t *testing.T) {
	plan := &TradePlan{
		ID:     4,
		Status: TradePlanStatusFailed,
	}
	got := RequireFrozenReadyTradePlan(plan)
	require.False(t, got.Allowed)
	require.Equal(t, ReasonPlanNotFrozen, got.Reason)
}

func TestRequireFrozenReadyTradePlan_SourceBoundaryWiring(t *testing.T) {
	require.Equal(t, "PLAN_NOT_FROZEN", ReasonPlanNotFrozen)
	require.Equal(t, "PLAN_NOT_APPROVED", ReasonPlanNotApproved)
	// A6.2 wires Guard in data adapter; Phase10-D.0.1 also wires papertrading broker/job.
	adapter, err := os.ReadFile(filepath.Join("..", "data", "paper_open_buy.go"))
	require.NoError(t, err)
	require.Contains(t, string(adapter), "RequireFrozenReadyTradePlan",
		"A6.2 must wire Guard in RunPaperOpenPrepare/Buy")

	broker, err := os.ReadFile(filepath.Join("..", "papertrading", "broker.go"))
	require.NoError(t, err)
	require.Contains(t, string(broker), "RequireFrozenReadyTradePlan",
		"Phase10-D.0.1 must wire Guard in PaperBroker")

	for _, rel := range []string{
		filepath.Join("..", "..", "app_paper_open_buy.go"),
		filepath.Join("..", "..", "app_trading_preflight.go"),
	} {
		b, err := os.ReadFile(rel)
		require.NoError(t, err, rel)
		require.False(t, strings.Contains(string(b), "RequireFrozenReadyTradePlan"),
			"%s must not call Guard directly", rel)
	}
}
