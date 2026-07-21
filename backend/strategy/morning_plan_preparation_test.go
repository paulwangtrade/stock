package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestRunMorningPlanPreparation_AdoptFrozen(t *testing.T) {
	freezeAt := time.Now()
	frozen := &models.TradePlan{
		ID:          42,
		TradeDate:   "2026-07-22",
		Status:      models.TradePlanStatusReady,
		PlanVersion: 3,
		FreezeAt:    &freezeAt,
		PoolID:      7,
		Side:        "buy",
	}

	prevFrozen := getFrozenTradePlanFn
	prevDaily := runDailyCandidateAndPlanFn
	t.Cleanup(func() {
		getFrozenTradePlanFn = prevFrozen
		runDailyCandidateAndPlanFn = prevDaily
	})

	var dailyCalls atomic.Int32
	getFrozenTradePlanFn = func(tradeDate string) (*models.TradePlan, error) {
		require.Equal(t, "2026-07-22", tradeDate)
		return frozen, nil
	}
	runDailyCandidateAndPlanFn = func(string) (*models.CandidatePool, *models.TradePlan, error) {
		dailyCalls.Add(1)
		return nil, nil, fmt.Errorf("legacy must not run when frozen exists")
	}

	pool, plan, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeAdoptFrozen, mode)
	require.Equal(t, uint(42), plan.ID)
	require.True(t, plan.IsFrozen())
	require.Equal(t, int32(0), dailyCalls.Load())
	_ = pool // optional; may be nil if PoolID not in test DB
}

func TestRunMorningPlanPreparation_FallbackLegacy(t *testing.T) {
	prevFrozen := getFrozenTradePlanFn
	prevDaily := runDailyCandidateAndPlanFn
	t.Cleanup(func() {
		getFrozenTradePlanFn = prevFrozen
		runDailyCandidateAndPlanFn = prevDaily
	})

	legacyPool := &models.CandidatePool{ID: 9, TradeDate: "2026-07-22", Status: models.CandidatePoolStatusReady}
	legacyPlan := &models.TradePlan{ID: 99, TradeDate: "2026-07-22", Status: models.TradePlanStatusReady}

	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return nil, fmt.Errorf("not found")
	}
	runDailyCandidateAndPlanFn = func(tradeDate string) (*models.CandidatePool, *models.TradePlan, error) {
		require.Equal(t, "2026-07-22", tradeDate)
		return legacyPool, legacyPlan, nil
	}

	pool, plan, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeBuildMorning, mode)
	require.Equal(t, uint(9), pool.ID)
	require.Equal(t, uint(99), plan.ID)
	require.False(t, plan.IsFrozen())
}

func TestRunMorningPlanPreparation_FrozenSkipsCandidatePoolCreate(t *testing.T) {
	freezeAt := time.Now()
	prevFrozen := getFrozenTradePlanFn
	prevDaily := runDailyCandidateAndPlanFn
	t.Cleanup(func() {
		getFrozenTradePlanFn = prevFrozen
		runDailyCandidateAndPlanFn = prevDaily
	})

	var poolCreates atomic.Int32
	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return &models.TradePlan{
			ID: 1, TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
			PlanVersion: 1, FreezeAt: &freezeAt,
		}, nil
	}
	runDailyCandidateAndPlanFn = func(string) (*models.CandidatePool, *models.TradePlan, error) {
		// Real RunDaily would create a pool; count any legacy invocation as a create.
		poolCreates.Add(1)
		return &models.CandidatePool{ID: 1}, &models.TradePlan{ID: 1}, nil
	}

	_, _, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeAdoptFrozen, mode)
	require.Equal(t, int32(0), poolCreates.Load(), "CandidatePool create count must be 0 when adopting frozen")
}

func TestRunMorningPlanPreparation_SourceBoundaryNoExecution(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("morning_plan_preparation.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunPaperOpenPrepare(",
		"TradingPreflight",
		"ApproveTradePlan(",
		"FreezeTradePlan(",
		"CreatePlanWithItems(",
		"BuildTradePlan(",
		"BuildCandidatePool(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("morning preparation must not reference %s", token)
		}
	}
	require.Contains(t, src, "GetFrozenTradePlan")
	require.Contains(t, src, "RunDailyCandidateAndPlan")
	require.NotContains(t, src, "InitPaperOpenBuyJobs")
}
