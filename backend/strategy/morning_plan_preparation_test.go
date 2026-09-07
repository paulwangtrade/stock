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

func restoreMorningPrepFns(t *testing.T) {
	t.Helper()
	prevFrozen := getFrozenTradePlanFn
	prevDraft := getLatestBuyDraftFn
	prevDaily := runDailyCandidateAndPlanFn
	t.Cleanup(func() {
		getFrozenTradePlanFn = prevFrozen
		getLatestBuyDraftFn = prevDraft
		runDailyCandidateAndPlanFn = prevDaily
	})
}

func stubNoFrozenNoDraft(t *testing.T) {
	t.Helper()
	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return nil, fmt.Errorf("not found")
	}
	getLatestBuyDraftFn = func(string) (*models.TradePlan, error) {
		return nil, fmt.Errorf("not found")
	}
}

func TestRunMorningPlanPreparation_CaseB_AdoptFrozen(t *testing.T) {
	restoreMorningPrepFns(t)
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

	var dailyCalls atomic.Int32
	getFrozenTradePlanFn = func(tradeDate string) (*models.TradePlan, error) {
		require.Equal(t, "2026-07-22", tradeDate)
		return frozen, nil
	}
	getLatestBuyDraftFn = func(string) (*models.TradePlan, error) {
		t.Fatal("draft lookup must not run when frozen exists")
		return nil, nil
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
	_ = pool
}

func TestRunMorningPlanPreparation_CaseA_BuildMorningDraft(t *testing.T) {
	restoreMorningPrepFns(t)
	stubNoFrozenNoDraft(t)

	legacyPool := &models.CandidatePool{ID: 9, TradeDate: "2026-07-22", Status: models.CandidatePoolStatusReady}
	created := &models.TradePlan{
		ID: 99, TradeDate: "2026-07-22", Status: models.TradePlanStatusDraft,
		EnableExecute: false, Side: "buy",
	}
	var dailyCalls atomic.Int32
	runDailyCandidateAndPlanFn = func(tradeDate string) (*models.CandidatePool, *models.TradePlan, error) {
		require.Equal(t, "2026-07-22", tradeDate)
		dailyCalls.Add(1)
		return legacyPool, created, nil
	}

	pool, plan, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeBuildMorning, mode)
	require.Equal(t, uint(9), pool.ID)
	require.Equal(t, uint(99), plan.ID)
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.EnableExecute)
	require.Nil(t, plan.FreezeAt)
	require.False(t, plan.IsFrozen())
	require.Equal(t, int32(1), dailyCalls.Load())
}

func TestRunMorningPlanPreparation_CaseC_AdoptBuyDraft(t *testing.T) {
	restoreMorningPrepFns(t)

	existing := &models.TradePlan{
		ID: 55, TradeDate: "2026-07-22", Status: models.TradePlanStatusDraft,
		PlanVersion: 1, PoolID: 8, Side: "buy", EnableExecute: false,
	}
	var dailyCalls atomic.Int32
	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return nil, fmt.Errorf("not found")
	}
	getLatestBuyDraftFn = func(tradeDate string) (*models.TradePlan, error) {
		require.Equal(t, "2026-07-22", tradeDate)
		return existing, nil
	}
	runDailyCandidateAndPlanFn = func(string) (*models.CandidatePool, *models.TradePlan, error) {
		dailyCalls.Add(1)
		return nil, nil, fmt.Errorf("must not create when buy draft exists")
	}

	pool, plan, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeAdoptDraft, mode)
	require.Equal(t, uint(55), plan.ID)
	require.True(t, plan.IsDraft())
	require.Equal(t, int32(0), dailyCalls.Load())
	_ = pool
}

func TestRunMorningPlanPreparation_SellDraftDoesNotAdopt(t *testing.T) {
	restoreMorningPrepFns(t)

	sell := &models.TradePlan{
		ID: 70, TradeDate: "2026-07-22", Status: models.TradePlanStatusDraft, Side: "sell",
	}
	created := &models.TradePlan{ID: 71, TradeDate: "2026-07-22", Status: models.TradePlanStatusDraft, Side: "buy"}
	var dailyCalls atomic.Int32
	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return nil, fmt.Errorf("not found")
	}
	getLatestBuyDraftFn = func(string) (*models.TradePlan, error) {
		return sell, nil // defense: even if repo leaked sell, isAdoptableBuyDraft rejects
	}
	runDailyCandidateAndPlanFn = func(string) (*models.CandidatePool, *models.TradePlan, error) {
		dailyCalls.Add(1)
		return &models.CandidatePool{ID: 1}, created, nil
	}

	_, plan, mode, err := RunMorningPlanPreparation("2026-07-22")
	require.NoError(t, err)
	require.Equal(t, MorningPlanModeBuildMorning, mode)
	require.Equal(t, uint(71), plan.ID)
	require.Equal(t, int32(1), dailyCalls.Load())
}

func TestRunMorningPlanPreparation_FrozenSkipsCandidatePoolCreate(t *testing.T) {
	restoreMorningPrepFns(t)
	freezeAt := time.Now()
	var poolCreates atomic.Int32
	getFrozenTradePlanFn = func(string) (*models.TradePlan, error) {
		return &models.TradePlan{
			ID: 1, TradeDate: "2026-07-22", Status: models.TradePlanStatusReady,
			PlanVersion: 1, FreezeAt: &freezeAt,
		}, nil
	}
	getLatestBuyDraftFn = func(string) (*models.TradePlan, error) {
		t.Fatal("draft lookup must not run when frozen exists")
		return nil, nil
	}
	runDailyCandidateAndPlanFn = func(string) (*models.CandidatePool, *models.TradePlan, error) {
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
		"BuildDraftTradePlanFromCandidatePool(",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("morning preparation must not reference %s", token)
		}
	}
	require.Contains(t, src, "GetFrozenTradePlan")
	require.Contains(t, src, "GetLatestBuyDraftByTradeDate")
	require.Contains(t, src, "RunDailyCandidateAndPlan")
	require.Contains(t, src, "MorningPlanModeAdoptDraft")
	require.NotContains(t, src, "InitPaperOpenBuyJobs")
}
