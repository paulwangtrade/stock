package strategy

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/readiness"

	"github.com/stretchr/testify/require"
)

func TestRunMorningIntentMaterialize_DraftIntentHappyPath(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-08-05", []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, RefSource: afterCloseRefSourcePrevClose, RefAsOf: "2026-08-04",
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		IntentStatus: afterCloseIntentStatusSelected, LimitPrice: 0, TargetVolume: 0,
	}})

	before := readiness.EvaluateExecutionIntentReadiness(plan, &readiness.Options{SkipQualityGate: true})
	require.False(t, before.Ready)
	require.NotEmpty(t, before.Blockers)
	beforeCount := len(before.Blockers)

	res, err := RunMorningIntentMaterialize(plan.ID, &MorningIntentMaterializeOpts{
		OpenPriceFn: func(code string) (float64, bool) {
			require.Equal(t, "sz000001", code)
			return 10.2, true
		},
		VolumeOpts: &MorningPositionMaterializeOpts{Snapshot: richSnapshot(1_000_000, 1_000_000), Limits: wideLimits()},
		EvaluateReadiness: func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
			return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: true})
		},
	})
	require.NoError(t, err)
	require.True(t, res.Success)
	require.Equal(t, plan.ID, res.PlanID)
	require.Equal(t, 1, res.MaterializedItems)
	require.True(t, res.ReadinessReady)
	require.Empty(t, res.Blockers)
	require.Less(t, len(res.Blockers), beforeCount)
	require.Equal(t, morningPricingStage, res.PricingStage)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	it := got.Items[0]
	require.Equal(t, morningIntentStatusPriced, it.IntentStatus)
	require.InDelta(t, 10.0*(1+0.03), it.LimitPrice, 1e-9)
	require.GreaterOrEqual(t, it.TargetVolume, morningLotSize)
	require.Equal(t, 10.2, it.OpenRefPrice)
}

func TestRunMorningIntentMaterialize_IdempotentRepeat(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-08-05", []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		IntentStatus: afterCloseIntentStatusSelected, LimitPrice: 0, TargetVolume: 0,
	}})

	opts := &MorningIntentMaterializeOpts{
		OpenPriceFn: func(string) (float64, bool) { return 10.2, true },
		VolumeOpts:  &MorningPositionMaterializeOpts{Snapshot: richSnapshot(1_000_000, 1_000_000), Limits: wideLimits()},
		EvaluateReadiness: func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
			return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: true})
		},
	}
	res1, err := RunMorningIntentMaterialize(plan.ID, opts)
	require.NoError(t, err)
	require.True(t, res1.Success)
	require.Equal(t, 1, res1.MaterializedItems)

	got1, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	limit1 := got1.Items[0].LimitPrice
	vol1 := got1.Items[0].TargetVolume

	// Second call: stage morning_materialized; must not overwrite valid priced/sized results.
	res2, err := RunMorningIntentMaterialize(plan.ID, opts)
	require.NoError(t, err)
	require.True(t, res2.Success)
	require.Equal(t, 1, res2.MaterializedItems)

	got2, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, limit1, got2.Items[0].LimitPrice)
	require.Equal(t, vol1, got2.Items[0].TargetVolume)
	require.Equal(t, morningPricingStage, got2.PricingStage)
}

func TestRunMorningIntentMaterialize_DenyFrozen(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-08-05", []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, IntentStatus: afterCloseIntentStatusSelected,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
	}})
	require.NoError(t, markPlanFrozenForTest(plan.ID, time.Now()))

	res, err := RunMorningIntentMaterialize(plan.ID, &MorningIntentMaterializeOpts{
		OpenPriceFn: func(string) (float64, bool) { return 10.2, true },
	})
	require.NoError(t, err)
	require.False(t, res.Success)
	require.Equal(t, "precheck", res.FailedStep)
	require.Contains(t, res.Message, "frozen")
}

func TestRunMorningIntentMaterialize_DenyNotDraft(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-08-05", []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, RefPrice: 10,
		IntentStatus: afterCloseIntentStatusSelected, MaxSlippage: &slip,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip,
	}})
	require.NoError(t, db.Dao.Model(&models.TradePlan{}).Where("id = ?", plan.ID).
		Update("status", models.TradePlanStatusReady).Error)

	res, err := RunMorningIntentMaterialize(plan.ID, &MorningIntentMaterializeOpts{
		OpenPriceFn: func(string) (float64, bool) { return 10.2, true },
	})
	require.NoError(t, err)
	require.False(t, res.Success)
	require.Equal(t, "precheck", res.FailedStep)
	require.Contains(t, res.Message, "not draft")
}

func TestRunMorningIntentMaterialize_NoFakePrice(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-08-05", []models.TradePlanItem{{
		TradeDate: "2026-08-05", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, IntentStatus: afterCloseIntentStatusSelected,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		LimitPrice: 0, TargetVolume: 0,
	}})

	res, err := RunMorningIntentMaterialize(plan.ID, &MorningIntentMaterializeOpts{
		OpenPriceFn: func(string) (float64, bool) { return 0, false },
		VolumeOpts:  &MorningPositionMaterializeOpts{Snapshot: richSnapshot(1_000_000, 1_000_000), Limits: wideLimits()},
		EvaluateReadiness: func(p *models.TradePlan) readiness.ExecutionIntentReadinessResult {
			return readiness.EvaluateExecutionIntentReadiness(p, &readiness.Options{SkipQualityGate: true})
		},
	})
	require.NoError(t, err)
	// Completed pipeline without fabricating; pending_open leaves limit unset.
	require.True(t, res.Success)
	require.Equal(t, 0, res.MaterializedItems)
	require.False(t, res.ReadinessReady)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, float64(0), got.Items[0].LimitPrice)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
	require.Equal(t, afterClosePricingStage, got.PricingStage)
}
