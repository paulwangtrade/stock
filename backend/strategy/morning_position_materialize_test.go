package strategy

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func seedDraftPlanPriced(t *testing.T, tradeDate string, items []models.TradePlanItem) *models.TradePlan {
	t.Helper()
	slip := afterCloseDefaultMaxSlippage
	plan := &models.TradePlan{
		TradeDate:            tradeDate,
		GeneratedAt:          time.Now(),
		Status:               models.TradePlanStatusDraft,
		Side:                 "buy",
		AmountPerStock:       100_000,
		EnableExecute:        false,
		PlanVersion:          1,
		SourceSession:        models.TradePlanSourceAfterClose,
		DefaultEntryRule:     afterCloseEntryRuleLimitRefPlusSlip,
		DefaultMaxSlippage:   &slip,
		PricingPolicyVersion: afterClosePricingPolicyVersion,
		PricingStage:         morningPricingStage,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func richSnapshot(cash, equity float64) *MorningAccountSnapshot {
	return &MorningAccountSnapshot{
		Cash:            cash,
		Equity:          equity,
		LongMarketValue: 0,
		NameMarketValue: map[string]float64{},
		PositionVolumes: map[string]int64{},
	}
}

func wideLimits() *MorningRiskLimits {
	return &MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.50}
}

func TestMaterializeMorningTargetVolumes_PricedSetsVolume(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})

	// Cash >> budget: must still cap by budget (anti cash/price).
	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.False(t, res.NoOp)
	require.Equal(t, 1, res.SizedCount)
	require.Equal(t, 0, res.SizeSkipCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	it := got.Items[0]
	require.Equal(t, morningIntentStatusPriced, it.IntentStatus)
	require.Equal(t, int64(10_000), it.TargetVolume) // 100000/10 lot
	require.Equal(t, 10.0, it.LimitPrice, "must not rewrite limit_price")
	require.Equal(t, float64(100_000), it.TargetAmount, "must not rewrite budget")
	require.NotEqual(t, int64(100_000), it.TargetVolume, "must not be cash/price alone")
}

func TestMaterializeMorningTargetVolumes_LotSizeRounding(t *testing.T) {
	setupDraftPlanTestDB(t)
	// 100000 / 10.3 → floor(9708.7/100)*100 = 9700
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.3, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})

	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(500_000, 500_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.SizedCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(9700), got.Items[0].TargetVolume)
	require.Equal(t, int64(0), got.Items[0].TargetVolume%100)
}

func TestMaterializeMorningTargetVolumes_InsufficientCash(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})

	// Equity high so single/gross headroom do not bind; cash is the clamp.
	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(45_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.SizedCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(4500), got.Items[0].TargetVolume)
	require.Less(t, got.Items[0].TargetVolume, int64(10_000), "cash must clamp below budget volume")

	// Too little for one lot → size_skip
	plan2 := seedDraftPlanPriced(t, "2026-07-30", []models.TradePlanItem{{
		TradeDate: "2026-07-30", StockCode: "sz000002", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})
	res2, err := MaterializeMorningTargetVolumes(plan2.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(500, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 0, res2.SizedCount)
	require.Equal(t, 1, res2.SizeSkipCount)
	got2, err := data.NewTradePlanRepo().GetByID(plan2.ID)
	require.NoError(t, err)
	require.Equal(t, morningIntentStatusSizeSkip, got2.Items[0].IntentStatus)
	require.Equal(t, int64(0), got2.Items[0].TargetVolume)
}

func TestMaterializeMorningTargetVolumes_PositionLimit(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})

	// Already hold → size_skip (position conflict / position limit)
	snap := richSnapshot(1_000_000, 1_000_000)
	snap.PositionVolumes["sz000001"] = 1000
	snap.NameMarketValue["sz000001"] = 10_000

	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: snap,
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.SizedCount)
	require.Equal(t, 1, res.SizeSkipCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, morningIntentStatusSizeSkip, got.Items[0].IntentStatus)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)

	// Single-name exposure headroom also clamps
	plan2 := seedDraftPlanPriced(t, "2026-07-30", []models.TradePlanItem{{
		TradeDate: "2026-07-30", StockCode: "sz000002", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})
	res2, err := MaterializeMorningTargetVolumes(plan2.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   &MorningRiskLimits{MaxGrossExposurePct: 0.95, MaxSingleNamePct: 0.05},
	})
	require.NoError(t, err)
	require.Equal(t, 1, res2.SizedCount)
	got2, err := data.NewTradePlanRepo().GetByID(plan2.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5000), got2.Items[0].TargetVolume)
}

func TestMaterializeMorningTargetVolumes_FrozenNoOp(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})
	require.NoError(t, markPlanFrozenForTest(plan.ID, time.Now()))

	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.True(t, res.NoOp)
	require.Equal(t, "frozen", res.NoOpReason)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
}

func TestMaterializeMorningTargetVolumes_LegacyNoOp(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := &models.TradePlan{
		TradeDate: "2026-07-29", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		EnableExecute: false, PlanVersion: 1, SourceSession: models.TradePlanSourceAfterClose,
		PricingPolicyVersion: 0, PricingStage: "",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}}))

	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.True(t, res.NoOp)
	require.Equal(t, "legacy_no_intent", res.NoOpReason)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
}

func TestMaterializeMorningTargetVolumes_RepeatedCallIdempotent(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := seedDraftPlanPriced(t, "2026-07-29", []models.TradePlanItem{{
		TradeDate: "2026-07-29", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		LimitPrice: 10.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})
	opts := &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	}
	res1, err := MaterializeMorningTargetVolumes(plan.ID, opts)
	require.NoError(t, err)
	require.Equal(t, 1, res1.SizedCount)

	// Second call with tighter cash must NOT overwrite existing volume.
	opts2 := &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(20_000, 20_000),
		Limits:   wideLimits(),
	}
	res2, err := MaterializeMorningTargetVolumes(plan.ID, opts2)
	require.NoError(t, err)
	require.Equal(t, 0, res2.SizedCount)
	require.Equal(t, 1, res2.IdempotentSkip)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(10_000), got.Items[0].TargetVolume)
}

func TestMaterializeMorningTargetVolumes_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("morning_position_materialize.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"ApproveDraft(",
		"ApproveTradePlan(",
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunPaperOpenPrepare(",
		"EvaluateDraftTradePlanRisk(",
		"PlanFilter(",
		"FilterPoolForTradePlan(",
	} {
		require.NotContains(t, src, token, "must not call %s", token)
	}
	require.Contains(t, src, "TargetVolume")
	require.Contains(t, src, "NOT cash/price alone")
}

func TestCalcMorningLotVolume(t *testing.T) {
	require.Equal(t, int64(10000), calcMorningLotVolume(100_000, 10))
	require.Equal(t, int64(9700), calcMorningLotVolume(100_000, 10.3))
	require.Equal(t, int64(0), calcMorningLotVolume(50, 10))
	require.Equal(t, int64(0), calcMorningLotVolume(100_000, 0))
}
