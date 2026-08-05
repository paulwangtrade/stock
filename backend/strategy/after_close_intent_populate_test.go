package strategy

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/marketdata/anchor"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

type stubAnchorProvider struct {
	fn func(anchor.Context) (anchor.Result, bool)
}

func (s stubAnchorProvider) Resolve(ctx anchor.Context) (anchor.Result, bool) {
	return s.fn(ctx)
}

func withAnchorProvider(t *testing.T, p anchor.Provider) {
	t.Helper()
	orig := afterCloseAnchorProvider
	afterCloseAnchorProvider = p
	t.Cleanup(func() { afterCloseAnchorProvider = orig })
}

func TestBuildDraftTradePlanFromCandidatePool_PopulatesAfterCloseIntent(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-07-28", "sz000001", "sh600519")

	withAnchorProvider(t, stubAnchorProvider{fn: func(ctx anchor.Context) (anchor.Result, bool) {
		switch ctx.StockCode {
		case "sz000001":
			return anchor.Result{RefPrice: 10.5, RefSource: afterCloseRefSourceStrategySnap, RefAsOf: "2026-07-21"}, true
		case "sh600519":
			return anchor.Result{RefPrice: 1800, RefSource: afterCloseRefSourceStrategySnap, RefAsOf: "2026-07-21"}, true
		default:
			return anchor.Result{}, false
		}
	}})

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.NotNil(t, plan)

	require.Equal(t, afterClosePricingPolicyVersion, plan.PricingPolicyVersion)
	require.Equal(t, afterClosePricingStage, plan.PricingStage)
	require.Equal(t, afterCloseEntryRuleLimitRefPlusSlip, plan.DefaultEntryRule)
	require.NotNil(t, plan.DefaultMaxSlippage)
	require.InDelta(t, afterCloseDefaultMaxSlippage, *plan.DefaultMaxSlippage, 1e-9)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, afterClosePricingStage, got.PricingStage)
	require.Equal(t, len(plan.Items), len(got.Items))

	for _, it := range got.Items {
		require.Equal(t, float64(0), it.LimitPrice, "limit_price must stay 0")
		require.Equal(t, int64(0), it.TargetVolume, "target_volume must stay 0")
		require.Equal(t, afterCloseIntentStatusSelected, it.IntentStatus)
		require.Equal(t, afterCloseEntryRuleLimitRefPlusSlip, it.EntryRule)
		require.NotNil(t, it.MaxSlippage)
		require.InDelta(t, afterCloseDefaultMaxSlippage, *it.MaxSlippage, 1e-9)
		require.Greater(t, it.RefPrice, 0.0)
		require.NotEmpty(t, it.RefSource)
		require.NotEmpty(t, it.RefAsOf)
		require.Empty(t, it.PricedBy)
		require.Nil(t, it.PricedAt)
	}
}

func TestBuildDraftTradePlanFromCandidatePool_LimitAndVolumeStayZero(t *testing.T) {
	setupDraftPlanTestDB(t)
	pool := seedReadyPool(t, "2026-07-28", "sz000001")

	withAnchorProvider(t, stubAnchorProvider{fn: func(anchor.Context) (anchor.Result, bool) {
		return anchor.Result{RefPrice: 12.3, RefSource: afterCloseRefSourcePrevClose, RefAsOf: "2026-07-27"}, true
	}})

	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	for _, it := range plan.Items {
		require.Equal(t, float64(0), it.LimitPrice)
		require.Equal(t, int64(0), it.TargetVolume)
	}
}

func TestCreatePlanWithItems_LegacyPlanUnaffectedByIntentPopulate(t *testing.T) {
	setupDraftPlanTestDB(t)

	// Legacy path: direct CreatePlanWithItems without AfterClose Intent populate.
	plan := &models.TradePlan{
		TradeDate:      "2026-07-28",
		GeneratedAt:    time.Now(),
		Status:         models.TradePlanStatusDraft,
		Side:           "buy",
		AmountPerStock: 100_000,
		EnableExecute:  false,
		PlanVersion:    1,
		SourceSession:  models.TradePlanSourceAfterClose,
	}
	items := []models.TradePlanItem{{
		TradeDate: "2026-07-28", StockCode: "sz000001", StockName: "平安银行",
		Side: "buy", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending,
		LimitPrice: 0, TargetVolume: 0,
	}}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, items))

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 0, got.PricingPolicyVersion)
	require.Equal(t, "", got.PricingStage)
	require.Equal(t, "", got.DefaultEntryRule)
	require.Nil(t, got.DefaultMaxSlippage)
	require.Len(t, got.Items, 1)
	require.Equal(t, "", got.Items[0].IntentStatus)
	require.Equal(t, float64(0), got.Items[0].RefPrice)
	require.Equal(t, float64(0), got.Items[0].LimitPrice)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
}

func TestPopulateAfterCloseExecutionIntent_DoesNotChangeItemCount(t *testing.T) {
	plan := &models.TradePlan{TradeDate: "2026-07-28"}
	items := []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending, LimitPrice: 99, TargetVolume: 100},
		{StockCode: "sh600000", Side: "buy", Status: models.TradePlanItemSkipped, LimitPrice: 88, TargetVolume: 200},
	}
	withAnchorProvider(t, stubAnchorProvider{fn: func(ctx anchor.Context) (anchor.Result, bool) {
		if ctx.StockCode == "sz000001" {
			return anchor.Result{RefPrice: 10, RefSource: afterCloseRefSourcePrevClose, RefAsOf: "2026-07-27"}, true
		}
		return anchor.Result{}, false
	}})

	populateAfterCloseExecutionIntent(plan, items, nil)
	require.Len(t, items, 2)
	require.Equal(t, float64(0), items[0].LimitPrice)
	require.Equal(t, int64(0), items[0].TargetVolume)
	require.Equal(t, afterCloseIntentStatusSelected, items[0].IntentStatus)
	require.Equal(t, "", items[1].IntentStatus)
	require.Equal(t, float64(0), items[1].LimitPrice)
	require.Equal(t, int64(0), items[1].TargetVolume)
}

func TestPopulate_DefaultProviderFollowedParity(t *testing.T) {
	setupDraftPlanTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&data.FollowedStock{}))
	require.NoError(t, db.Dao.Create(&data.FollowedStock{
		StockCode:   "sz000001",
		Name:        "平安银行",
		FollowPrice: 10.5,
		Price:       11.0,
		Time:        time.Date(2026, 7, 21, 15, 0, 0, 0, time.Local),
	}).Error)

	// Ensure production default (Followed) is active.
	withAnchorProvider(t, anchor.DefaultProvider())

	plan := &models.TradePlan{TradeDate: "2026-07-28"}
	items := []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending},
		{StockCode: "sh999999", Side: "buy", Status: models.TradePlanItemPending},
	}
	pool := &models.CandidatePool{Source: models.CandidatePoolSourceStrategyRun}
	populateAfterCloseExecutionIntent(plan, items, pool)

	require.Equal(t, afterCloseIntentStatusSelected, items[0].IntentStatus)
	require.InDelta(t, 10.5, items[0].RefPrice, 1e-9)
	require.Equal(t, afterCloseRefSourceStrategySnap, items[0].RefSource)
	require.Equal(t, "2026-07-21", items[0].RefAsOf)
	require.Equal(t, afterClosePricingStage, plan.PricingStage)

	require.Equal(t, "", items[1].IntentStatus)
	require.Equal(t, float64(0), items[1].RefPrice)
}

func TestPopulate_ProviderSoftFail(t *testing.T) {
	withAnchorProvider(t, stubAnchorProvider{fn: func(anchor.Context) (anchor.Result, bool) {
		return anchor.Result{}, false
	}})
	plan := &models.TradePlan{TradeDate: "2026-07-28"}
	items := []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending},
	}
	populateAfterCloseExecutionIntent(plan, items, nil)
	require.Equal(t, "", items[0].IntentStatus)
	require.Equal(t, float64(0), items[0].RefPrice)
	require.Equal(t, afterClosePricingStage, plan.PricingStage)
}
