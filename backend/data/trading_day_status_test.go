package data

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func resetTradingDayStatusTestHooks(t *testing.T) {
	t.Helper()
	ResetTradingCronRegistryForTest()
	SetTradingDaySchemaStatusProvider(nil)
	t.Cleanup(func() {
		ResetTradingCronRegistryForTest()
		SetTradingDaySchemaStatusProvider(nil)
	})
}

func TestBuildTradingDayStatus_FrozenReady(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	resetTradingDayStatusTestHooks(t)
	ReportTradingCronRegistered(TradingCronKeyAfterClose)
	ReportTradingCronRegistered(TradingCronKeyMorning)
	SetTradingDaySchemaStatusProvider(func() TradingDaySchemaView {
		return TradingDaySchemaView{RegistryVersion: 3, ValidationStatus: db.SchemaValidationReady}
	})

	repo := NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Hour)
	tradeDate := "2026-07-27"

	poolRepo := NewCandidatePoolRepo()
	pool := &models.CandidatePool{
		TradeDate: tradeDate, GeneratedAt: now, Status: models.CandidatePoolStatusReady, ItemCount: 1,
	}
	require.NoError(t, poolRepo.CreatePoolWithItems(pool, nil))

	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: pool.ID,
		Status: models.TradePlanStatusReady, PlanVersion: 2,
		ApprovedAt: &freezeAt, ApprovedBy: "approver",
		FreezeAt: &freezeAt, FreezeBy: "ops",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
		RiskSummary:   "all accepted",
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sh600519", Side: "buy", Priority: 1, TargetAmount: 10000, Status: models.TradePlanItemPending},
	}))

	view, err := BuildTradingDayStatus(tradeDate)
	require.NoError(t, err)
	require.Equal(t, tradeDate, view.Date)
	require.True(t, view.Plan.Exists)
	require.Equal(t, models.TradePlanStatusReady, view.Plan.Status)
	require.Equal(t, 2, view.Plan.PlanVersion)
	require.Equal(t, models.TradePlanSourceAfterClose, view.Plan.SourceSession)
	require.True(t, view.Freeze.IsFrozen)
	require.Equal(t, "ops", view.Freeze.FreezeBy)
	require.NotEmpty(t, view.Freeze.FreezeAt)
	require.True(t, view.Risk.Passed)
	require.Equal(t, risk.PlanRiskStatusPassed, view.Risk.Status)
	require.Equal(t, MorningModeAdoptFrozen, view.Morning.Mode)
	require.True(t, view.ExecutionReady.Ready)
	require.Equal(t, ExecutionGuardWouldPass, view.ExecutionReady.GuardStatus)
	require.Equal(t, AfterCloseStatusCompleted, view.AfterClose.Status)
	require.Equal(t, plan.ID, view.AfterClose.PlanID)
	require.Equal(t, pool.ID, view.AfterClose.PoolID)
	require.Equal(t, db.SchemaValidationReady, view.Schema.ValidationStatus)
	require.Equal(t, 3, view.Schema.RegistryVersion)
	require.True(t, view.Cron.AfterCloseRegistered)
	require.True(t, view.Cron.MorningRegistered)
}

func TestBuildTradingDayStatus_Draft(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	resetTradingDayStatusTestHooks(t)
	SetTradingDaySchemaStatusProvider(func() TradingDaySchemaView {
		return TradingDaySchemaView{RegistryVersion: 3, ValidationStatus: db.SchemaValidationReady}
	})

	repo := NewTradePlanRepo()
	now := time.Now()
	tradeDate := "2026-07-28"
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 9,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, nil))

	view, err := BuildTradingDayStatus(tradeDate)
	require.NoError(t, err)
	require.True(t, view.Plan.Exists)
	require.Equal(t, models.TradePlanStatusDraft, view.Plan.Status)
	require.False(t, view.Freeze.IsFrozen)
	require.Equal(t, MorningModeBuildMorning, view.Morning.Mode)
	require.False(t, view.ExecutionReady.Ready)
	require.Equal(t, ExecutionGuardWouldBlockNotFrozen, view.ExecutionReady.GuardStatus)
	require.Equal(t, models.ReasonPlanNotFrozen, view.ExecutionReady.Reason)
	require.True(t, view.Risk.Passed)
	require.Equal(t, AfterCloseStatusCompleted, view.AfterClose.Status)
}

func TestBuildTradingDayStatus_NakedReady(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	resetTradingDayStatusTestHooks(t)
	SetTradingDaySchemaStatusProvider(func() TradingDaySchemaView {
		return TradingDaySchemaView{RegistryVersion: 3, ValidationStatus: db.SchemaValidationReady}
	})

	repo := NewTradePlanRepo()
	now := time.Now()
	tradeDate := "2026-07-29"
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 3,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		SourceSession: models.TradePlanSourceMorningRebuild,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, nil))

	view, err := BuildTradingDayStatus(tradeDate)
	require.NoError(t, err)
	require.True(t, view.Plan.Exists)
	require.Equal(t, models.TradePlanStatusReady, view.Plan.Status)
	require.False(t, view.Freeze.IsFrozen, "naked ready must not be frozen")
	require.Equal(t, MorningModeBuildMorning, view.Morning.Mode)
	require.False(t, view.ExecutionReady.Ready, "naked ready must NOT be execution ready (Guard)")
	require.Equal(t, ExecutionGuardWouldBlockNotFrozen, view.ExecutionReady.GuardStatus)
	require.Equal(t, models.ReasonPlanNotFrozen, view.ExecutionReady.Reason)
}

func TestBuildTradingDayStatus_NoPlan(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	resetTradingDayStatusTestHooks(t)
	SetTradingDaySchemaStatusProvider(func() TradingDaySchemaView {
		return TradingDaySchemaView{RegistryVersion: 3, ValidationStatus: db.SchemaValidationReady}
	})

	view, err := BuildTradingDayStatus("2026-08-01")
	require.NoError(t, err)
	require.False(t, view.Plan.Exists)
	require.Equal(t, RiskStatusNA, view.Risk.Status)
	require.False(t, view.Freeze.IsFrozen)
	require.Equal(t, MorningModeBuildMorning, view.Morning.Mode)
	require.False(t, view.ExecutionReady.Ready)
	require.Equal(t, ExecutionGuardNA, view.ExecutionReady.GuardStatus)
	// Default after_close switch is off → SKIPPED_DISABLED when no persisted pool/plan.
	require.Equal(t, AfterCloseStatusSkippedDisabled, view.AfterClose.Status)
}

func TestBuildTradingDayStatus_SchemaBlocked(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	resetTradingDayStatusTestHooks(t)
	SetTradingDaySchemaStatusProvider(func() TradingDaySchemaView {
		return TradingDaySchemaView{RegistryVersion: 2, ValidationStatus: db.SchemaValidationBlocked}
	})

	repo := NewTradePlanRepo()
	now := time.Now()
	freezeAt := now.Add(-time.Minute)
	tradeDate := "2026-08-02"
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusReady, PlanVersion: 1,
		ApprovedAt: &freezeAt, ApprovedBy: "approver",
		FreezeAt: &freezeAt, FreezeBy: "ops",
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, nil))

	view, err := BuildTradingDayStatus(tradeDate)
	require.NoError(t, err)
	require.Equal(t, db.SchemaValidationBlocked, view.Schema.ValidationStatus)
	require.Equal(t, 2, view.Schema.RegistryVersion)
	// Guard-aligned ready is independent of schema in this slice (schema exposed separately).
	require.True(t, view.ExecutionReady.Ready)
	require.Equal(t, ExecutionGuardWouldPass, view.ExecutionReady.GuardStatus)
}
