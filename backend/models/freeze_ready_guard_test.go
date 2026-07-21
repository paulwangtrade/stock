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
		ID:        1,
		Status:    TradePlanStatusReady,
		FreezeAt:  &now,
		TradeDate: "2026-07-22",
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

func TestRequireFrozenReadyTradePlan_SourceBoundaryNotWired(t *testing.T) {
	// A6.1: independent guard only — must not be called from Execution adapters yet.
	require.Equal(t, "PLAN_NOT_FROZEN", ReasonPlanNotFrozen)
	for _, rel := range []string{
		filepath.Join("..", "data", "paper_open_buy.go"),
		filepath.Join("..", "..", "app_paper_open_buy.go"),
		filepath.Join("..", "..", "app_trading_preflight.go"),
	} {
		b, err := os.ReadFile(rel)
		require.NoError(t, err, rel)
		require.False(t, strings.Contains(string(b), "RequireFrozenReadyTradePlan"),
			"%s must not wire A6.1 guard yet", rel)
	}
}
