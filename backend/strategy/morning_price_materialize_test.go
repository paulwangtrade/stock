package strategy

import (
	"os"
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func seedDraftPlanWithIntent(t *testing.T, tradeDate string, items []models.TradePlanItem) *models.TradePlan {
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
		PricingStage:         afterClosePricingStage,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	return got
}

func TestMaterializeMorningLimitPrices_DraftOpenPriceSetsLimit(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-07-28", []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 100_000,
		RefPrice: 10.0, RefSource: afterCloseRefSourcePrevClose, RefAsOf: "2026-07-27",
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		IntentStatus: afterCloseIntentStatusSelected, LimitPrice: 0, TargetVolume: 0,
	}})

	res, err := MaterializeMorningLimitPrices(plan.ID, func(code string) (float64, bool) {
		require.Equal(t, "sz000001", code)
		return 10.2, true // gap 2% < 3%
	})
	require.NoError(t, err)
	require.False(t, res.NoOp)
	require.Equal(t, 1, res.PricedCount)
	require.Equal(t, 0, res.GapSkipCount)
	require.Equal(t, morningPricingStage, res.PricingStage)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, morningPricingStage, got.PricingStage)
	require.Len(t, got.Items, 1)
	it := got.Items[0]
	require.Equal(t, morningIntentStatusPriced, it.IntentStatus)
	require.InDelta(t, 10.0*(1+0.03), it.LimitPrice, 1e-9)
	require.Equal(t, 10.2, it.OpenRefPrice)
	require.Equal(t, morningPricedBy, it.PricedBy)
	require.NotNil(t, it.PricedAt)
	require.Equal(t, int64(0), it.TargetVolume, "13.1 must not write target_volume")
}

func TestMaterializeMorningLimitPrices_FrozenNoOp(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-07-28", []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, RefPrice: 10, IntentStatus: afterCloseIntentStatusSelected,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip, LimitPrice: 0,
	}})
	now := time.Now()
	require.NoError(t, markPlanFrozenForTest(plan.ID, now))

	before, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.True(t, before.IsFrozen())

	res, err := MaterializeMorningLimitPrices(plan.ID, func(string) (float64, bool) { return 10.5, true })
	require.NoError(t, err)
	require.True(t, res.NoOp)
	require.Equal(t, "frozen", res.NoOpReason)

	after, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, float64(0), after.Items[0].LimitPrice)
	require.Equal(t, afterCloseIntentStatusSelected, after.Items[0].IntentStatus)
	require.Equal(t, before.PricingStage, after.PricingStage)
}

func markPlanFrozenForTest(planID uint, at time.Time) error {
	return db.Dao.Model(&models.TradePlan{}).Where("id = ?", planID).Updates(map[string]any{
		"status":    models.TradePlanStatusReady,
		"freeze_at": at,
		"freeze_by": "review-test",
	}).Error
}

func TestMaterializeMorningLimitPrices_GapSkip(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-07-28", []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, RefPrice: 10.0,
		IntentStatus: afterCloseIntentStatusSelected,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		LimitPrice: 0, TargetVolume: 0,
	}})

	res, err := MaterializeMorningLimitPrices(plan.ID, func(string) (float64, bool) {
		return 10.5, true // gap 5% > 3%
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.PricedCount)
	require.Equal(t, 1, res.GapSkipCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	it := got.Items[0]
	require.Equal(t, morningIntentStatusGapSkip, it.IntentStatus)
	require.Equal(t, float64(0), it.LimitPrice)
	require.Equal(t, int64(0), it.TargetVolume)
	require.Equal(t, 10.5, it.OpenRefPrice)
	require.Equal(t, morningPricingStage, got.PricingStage)
}

func TestMaterializeMorningLimitPrices_PriceUnavailable(t *testing.T) {
	setupDraftPlanTestDB(t)
	slip := 0.03
	plan := seedDraftPlanWithIntent(t, "2026-07-28", []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, RefPrice: 10.0,
		IntentStatus: afterCloseIntentStatusSelected,
		EntryRule: afterCloseEntryRuleLimitRefPlusSlip, MaxSlippage: &slip,
		LimitPrice: 0,
	}})

	res, err := MaterializeMorningLimitPrices(plan.ID, func(string) (float64, bool) {
		return 0, false
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.PricedCount)
	require.Equal(t, 1, res.PendingOpenCount)
	require.Equal(t, afterClosePricingStage, res.PricingStage)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, afterCloseIntentStatusSelected, got.Items[0].IntentStatus)
	require.Equal(t, float64(0), got.Items[0].LimitPrice)
	require.Equal(t, afterClosePricingStage, got.PricingStage)
}

func TestMaterializeMorningLimitPrices_LegacyPlanNoOp(t *testing.T) {
	setupDraftPlanTestDB(t)
	plan := &models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: time.Now(),
		Status: models.TradePlanStatusDraft, Side: "buy", AmountPerStock: 100_000,
		EnableExecute: false, PlanVersion: 1, SourceSession: models.TradePlanSourceAfterClose,
		PricingPolicyVersion: 0, PricingStage: "",
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, LimitPrice: 0, TargetVolume: 0,
	}}))

	res, err := MaterializeMorningLimitPrices(plan.ID, func(string) (float64, bool) { return 11, true })
	require.NoError(t, err)
	require.True(t, res.NoOp)
	require.Equal(t, "legacy_no_intent", res.NoOpReason)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, float64(0), got.Items[0].LimitPrice)
	require.Equal(t, "", got.Items[0].IntentStatus)
}

func TestMaterializeMorningLimitPrices_SourceBoundary(t *testing.T) {
	raw, err := os.ReadFile("morning_price_materialize.go")
	require.NoError(t, err)
	src := string(raw)
	for _, token := range []string{
		"ApproveDraft(",
		"PromoteDraftToFrozen(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"RunMorningPlanPreparation(",
	} {
		require.NotContains(t, src, token, "must not call %s", token)
	}
	require.Contains(t, src, "LimitPrice")
	require.Contains(t, src, "IsFrozen()")
	require.Contains(t, src, "does NOT write target_volume")
}
