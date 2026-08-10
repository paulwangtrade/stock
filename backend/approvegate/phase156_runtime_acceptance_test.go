package approvegate

// Phase6.5.6.15.3 Runtime Acceptance Harness.
// Isolated in-memory DB (schema v5 capability via ApprovedSource column).
// Calls existing Approve / Freeze / Guard symbols only — does not modify
// Execution / Freeze / Strategy / Cron implementations.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"
	"go-stock/backend/strategy"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type itemSpecSnap struct {
	LimitPrice   float64
	TargetVolume int64
	IntentStatus string
	RefPrice     float64
	RefSource    string
	EntryRule    string
	OpenRefPrice float64
}

func setupAcceptanceDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:acc156_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureTradePlanTables())
	// schema v5 capability: approved_source must exist (isolated replica, not production).
	require.True(t, db.Dao.Migrator().HasColumn(&models.TradePlan{}, "ApprovedSource"),
		"schema v5: approved_source column required")
	require.NoError(t, data.SavePaperOpenBuyConfig(data.PaperOpenBuyConfig{
		EnablePaperOpenBuy:    false,
		OpenBuyAmountPerStock: 100_000,
		EnableRiskFilter:      false,
	}))
}

func acceptanceReadinessOpts() *readiness.Options {
	return &readiness.Options{
		MarketData: qualitygate.MarketDataSnapshot{
			SkipGapEval: true,
			IndustryByCode: map[string]string{"sz000001": "银行"},
			NameByCode:     map[string]string{"sz000001": "平安银行"},
			AnchorPriceByCode: map[string]float64{"sz000001": 10},
		},
	}
}

func acceptancePassOpts() *ApproveOptions {
	return &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  passRiskEvaluator,
			ReadinessOpts: acceptanceReadinessOpts(),
		},
	}
}

func acceptanceFailRiskOpts() *ApproveOptions {
	return &ApproveOptions{
		Eligibility: &EligibilityOptions{
			EvaluateRisk:  failRiskEvaluator,
			ReadinessOpts: acceptanceReadinessOpts(),
		},
	}
}

func snapItemSpec(plan *models.TradePlan) itemSpecSnap {
	if plan == nil || len(plan.Items) == 0 {
		return itemSpecSnap{}
	}
	it := plan.Items[0]
	return itemSpecSnap{
		LimitPrice:   it.LimitPrice,
		TargetVolume: it.TargetVolume,
		IntentStatus: it.IntentStatus,
		RefPrice:     it.RefPrice,
		RefSource:    it.RefSource,
		EntryRule:    it.EntryRule,
		OpenRefPrice: it.OpenRefPrice,
	}
}

func requireSpecUnchanged(t *testing.T, before, after itemSpecSnap) {
	t.Helper()
	require.Equal(t, before, after, "Intent/Order Spec must be unchanged across Approve/Freeze")
}

// TestRuntimeAcceptance_Phase156_S0toS4_HappyPath:
// Draft(materialized) → Risk+Readiness → Approve → Freeze → Guard.Allowed
func TestRuntimeAcceptance_Phase156_S0toS4_HappyPath(t *testing.T) {
	setupAcceptanceDB(t)

	// ----- S0: materialized Draft -----
	plan := seedMaterializedDraft(t, "2026-08-01")
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, 1, plan.PricingPolicyVersion)
	require.Equal(t, "morning_materialized", plan.PricingStage)
	require.Equal(t, readiness.IntentPriced, plan.Items[0].IntentStatus)
	require.Greater(t, plan.Items[0].LimitPrice, 0.0)
	require.GreaterOrEqual(t, plan.Items[0].TargetVolume, int64(100))
	spec0 := snapItemSpec(plan)

	// ----- S1: Risk + Readiness PASS -----
	elig, err := CheckApproveEligibility(plan.ID, acceptancePassOpts().Eligibility)
	require.NoError(t, err)
	require.True(t, elig.Eligible, "blockers=%+v", elig.Blockers)
	require.NotNil(t, elig.RiskResult)
	require.True(t, elig.RiskResult.Passed)
	require.NotNil(t, elig.ReadinessResult)
	require.True(t, elig.ReadinessResult.Ready)
	require.Empty(t, elig.Blockers)

	// ----- S2: ApproveTradePlanByID -----
	approveRes, err := ApproveTradePlanByID(plan.ID, "acceptance-actor", "acceptance", acceptancePassOpts())
	require.NoError(t, err)
	require.True(t, approveRes.OK)
	require.Equal(t, CodeApproveWriteOK, approveRes.Code)
	require.NotNil(t, approveRes.Plan)
	require.Equal(t, models.TradePlanStatusDraft, approveRes.Plan.Status)
	require.NotNil(t, approveRes.Plan.ApprovedAt)
	require.Equal(t, "acceptance-actor", approveRes.Plan.ApprovedBy)
	require.Equal(t, "acceptance", approveRes.Plan.ApprovedSource)
	require.Nil(t, approveRes.Plan.FreezeAt)
	require.False(t, approveRes.Plan.IsFrozen())

	afterApprove, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	requireSpecUnchanged(t, spec0, snapItemSpec(afterApprove))

	// Guard must still block draft (even if approved).
	guardDraft := models.RequireFrozenReadyTradePlan(afterApprove)
	require.False(t, guardDraft.Allowed)
	require.Equal(t, models.ReasonPlanNotFrozen, guardDraft.Reason)

	// ----- S3: FreezeTradePlan -----
	frozen, err := strategy.FreezeTradePlan(afterApprove, "acceptance-156", "15.3 freeze")
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, frozen.Status)
	require.NotNil(t, frozen.FreezeAt)
	require.Equal(t, "acceptance-156", frozen.FreezeBy)
	require.True(t, frozen.IsFrozen())
	require.NotNil(t, frozen.ApprovedAt)
	require.Equal(t, "acceptance-actor", frozen.ApprovedBy)
	require.Equal(t, "acceptance", frozen.ApprovedSource)

	afterFreeze, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	requireSpecUnchanged(t, spec0, snapItemSpec(afterFreeze))

	rd := readiness.EvaluateExecutionIntentReadiness(afterFreeze, acceptanceReadinessOpts())
	require.Equal(t, readiness.StageFrozenSnapshot, rd.LifecycleStage)

	// ----- S4: RequireFrozenReadyTradePlan -----
	guard := models.RequireFrozenReadyTradePlan(afterFreeze)
	require.True(t, guard.Allowed, "reason=%s msg=%s", guard.Reason, guard.Message)
	require.Empty(t, guard.Reason)
}

func TestRuntimeAcceptance_Phase156_N1_FreezeWithoutApprove(t *testing.T) {
	setupAcceptanceDB(t)
	plan := seedMaterializedDraft(t, "2026-08-01")
	require.Nil(t, plan.ApprovedAt)

	_, err := strategy.FreezeTradePlan(plan, "acceptance-156", "n1 should fail")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ApprovedAt is nil")

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Nil(t, got.FreezeAt)
	require.False(t, got.IsFrozen())
	require.False(t, models.RequireFrozenReadyTradePlan(got).Allowed)
}

func TestRuntimeAcceptance_Phase156_N2_RiskDeny(t *testing.T) {
	setupAcceptanceDB(t)
	plan := seedMaterializedDraft(t, "2026-08-01")

	res, err := ApproveTradePlanByID(plan.ID, "actor", "acceptance", acceptanceFailRiskOpts())
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Equal(t, CodeDenied, res.Code)
	require.True(t, hasCode(res.Blockers, CodeRiskNotPassed))

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)

	_, ferr := strategy.FreezeTradePlan(got, "acceptance-156", "n2")
	require.Error(t, ferr)
	require.Contains(t, ferr.Error(), "ApprovedAt is nil")
}

func TestRuntimeAcceptance_Phase156_N3_ReadinessDeny(t *testing.T) {
	setupAcceptanceDB(t)
	plan := seedUnmaterializedDraft(t, "2026-08-01")

	res, err := ApproveTradePlanByID(plan.ID, "actor", "acceptance", acceptancePassOpts())
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Equal(t, CodeDenied, res.Code)
	require.NotEmpty(t, res.Blockers)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Nil(t, got.ApprovedAt)

	_, ferr := strategy.FreezeTradePlan(got, "acceptance-156", "n3")
	require.Error(t, ferr)
}

func TestRuntimeAcceptance_Phase156_N4_DuplicateApprove(t *testing.T) {
	setupAcceptanceDB(t)
	plan := seedMaterializedDraft(t, "2026-08-01")

	first, err := ApproveTradePlanByID(plan.ID, "alice", "acceptance", acceptancePassOpts())
	require.NoError(t, err)
	require.True(t, first.OK)
	firstAt := *first.Plan.ApprovedAt

	second, err := ApproveTradePlanByID(plan.ID, "bob", "http_api", acceptancePassOpts())
	require.NoError(t, err)
	require.False(t, second.OK)
	require.True(t, second.AlreadyApproved)
	require.Equal(t, CodeAlreadyApproved, second.Code)
	require.Equal(t, "alice", second.Plan.ApprovedBy)
	require.Equal(t, "acceptance", second.Plan.ApprovedSource)
	require.True(t, second.Plan.ApprovedAt.Equal(firstAt))

	// Still freezable once with original approval metadata.
	frozen, err := strategy.FreezeTradePlan(second.Plan, "acceptance-156", "n4 freeze")
	require.NoError(t, err)
	require.True(t, frozen.IsFrozen())
	require.Equal(t, "alice", frozen.ApprovedBy)
	require.Equal(t, "acceptance", frozen.ApprovedSource)
	require.True(t, models.RequireFrozenReadyTradePlan(frozen).Allowed)
}

func TestRuntimeAcceptance_Phase156_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("phase156_runtime_acceptance_test.go"))
	require.NoError(t, err)
	src := string(raw)
	// Harness may *call* FreezeTradePlan / Guard; must not embed Execution buy path.
	// Tokens are split so this assertion block does not self-match.
	forbidden := []string{
		"RunPaperOpenBuy" + "Once(",
		"TryBegin" + "Execute(",
		"RunDailyCandidate" + "AndPlan(",
		"InitPaperOpenBuy" + "Jobs(",
	}
	for _, token := range forbidden {
		require.NotContains(t, src, token)
	}
	require.Contains(t, src, "ApproveTradePlanByID")
	require.Contains(t, src, "FreezeTradePlan")
	require.Contains(t, src, "RequireFrozenReadyTradePlan")
	require.Contains(t, src, "schema v5")
}
