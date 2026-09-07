package strategy

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestCalcMorningTargetVolume_FlagOff_MatchesLegacy(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())

	cases := []struct {
		amount, price float64
	}{
		{100_000, 10},
		{100_000, 10.3},
		{50, 10},
		{1010, 10},  // raw 101 → legacy 100
		{990, 10},   // raw 99 → legacy 0
		{1990, 10},  // would be STAR-ish raw; legacy still /100 → 1900
	}
	for _, tc := range cases {
		got := calcMorningTargetVolume("sz000001", tc.amount, tc.price)
		want := calcMorningLotVolume(tc.amount, tc.price)
		require.Equal(t, want, got, "amount=%.0f price=%.2f", tc.amount, tc.price)
	}
}

func TestCalcMorningTargetVolume_FlagOn_BoardMatrix(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	// amount = raw * price with price=1 → rawQty == amount
	const px = 1.0
	cases := []struct {
		name string
		code string
		raw  float64 // also amount at px=1
		want int64
	}{
		{"MAIN_99", "sz000001", 99, 0},
		{"MAIN_100", "sz000001", 100, 100},
		{"MAIN_101", "sz000001", 101, 100},
		{"STAR_199", "sh688981", 199, 0},
		{"STAR_200", "sh688981", 200, 200},
		{"STAR_201", "sh688981", 201, 201},
		{"ETF_99", "sh510300", 99, 0},
		{"ETF_100", "sh510300", 100, 100},
		{"CB_9", "sh113052", 9, 0},
		{"CB_10", "sh113052", 10, 10},
		{"CB_11", "sh113052", 11, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := calcMorningTargetVolume(tc.code, tc.raw*px, px)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestMaterializeMorningTargetVolumes_FlagOff_UnchangedMAIN(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	setupDraftPlanTestDB(t)

	plan := seedDraftPlanPriced(t, "2026-08-16", []models.TradePlanItem{{
		TradeDate: "2026-08-16", StockCode: "sz000001", Side: "buy",
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
	require.Equal(t, int64(9700), got.Items[0].TargetVolume) // legacy floor(/100)*100
}

func TestMaterializeMorningTargetVolumes_FlagOn_MAIN_STAR_ETF_CB(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	setupDraftPlanTestDB(t)

	// price=1, amount = desired raw shares; cash/equity wide so budget binds.
	items := []models.TradePlanItem{
		{TradeDate: "2026-08-16", StockCode: "sz000001", Side: "buy", Status: models.TradePlanItemPending,
			TargetAmount: 101, LimitPrice: 1.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced},
		{TradeDate: "2026-08-16", StockCode: "sh688981", Side: "buy", Status: models.TradePlanItemPending,
			TargetAmount: 201, LimitPrice: 1.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced},
		{TradeDate: "2026-08-16", StockCode: "sh510300", Side: "buy", Status: models.TradePlanItemPending,
			TargetAmount: 100, LimitPrice: 1.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced},
		{TradeDate: "2026-08-16", StockCode: "sh113052", Side: "buy", Status: models.TradePlanItemPending,
			TargetAmount: 11, LimitPrice: 1.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced},
	}
	plan := seedDraftPlanPriced(t, "2026-08-16", items)

	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 4, res.SizedCount)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	byCode := map[string]int64{}
	for _, it := range got.Items {
		byCode[it.StockCode] = it.TargetVolume
	}
	require.Equal(t, int64(100), byCode["sz000001"], "MAIN 101→100")
	require.Equal(t, int64(201), byCode["sh688981"], "STAR 201→201")
	require.Equal(t, int64(100), byCode["sh510300"], "ETF 100→100")
	require.Equal(t, int64(10), byCode["sh113052"], "CB 11→10")
}

func TestMaterializeMorningTargetVolumes_FlagOn_STARBelowMin_SizeSkip(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	setupDraftPlanTestDB(t)

	plan := seedDraftPlanPriced(t, "2026-08-16", []models.TradePlanItem{{
		TradeDate: "2026-08-16", StockCode: "sh688981", Side: "buy",
		Status: models.TradePlanItemPending, TargetAmount: 199,
		LimitPrice: 1.0, TargetVolume: 0, IntentStatus: morningIntentStatusPriced,
	}})
	res, err := MaterializeMorningTargetVolumes(plan.ID, &MorningPositionMaterializeOpts{
		Snapshot: richSnapshot(1_000_000, 1_000_000),
		Limits:   wideLimits(),
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.SizedCount)
	require.Equal(t, 1, res.SizeSkipCount)
	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, morningIntentStatusSizeSkip, got.Items[0].IntentStatus)
	require.Equal(t, int64(0), got.Items[0].TargetVolume)
}

func TestMorningBuyMinLot_FlagToggle(t *testing.T) {
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	tradingrule.SetEnableQuantityPolicyForTest(false)
	require.Equal(t, int64(100), morningBuyMinLot("sh688981"))

	tradingrule.SetEnableQuantityPolicyForTest(true)
	require.Equal(t, int64(200), morningBuyMinLot("sh688981"))
	require.Equal(t, int64(100), morningBuyMinLot("sz000001"))
	require.Equal(t, int64(10), morningBuyMinLot("sh113052"))
}
