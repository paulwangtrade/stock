package strategy

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/models"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestObserveQuantityPolicyShadow_MatchAndDiffMatrix(t *testing.T) {
	require.False(t, tradingrule.EnableQuantityPolicy(), "production Flag must stay off for shadow")

	// price=1 → amount == raw shares for easy fixtures
	const px = 1.0
	cases := []struct {
		name       string
		code       string
		amount     float64
		old        int64
		policy     int64
		diff       int64
		reason     string
		match      string
		abnormal   bool // policy < old with old>0
	}{
		// MAIN: legacy /100 == policy for common sizes
		{"MAIN_100_match", "sz000001", 100, 100, 100, 0, tradingrule.BuyQtyReasonOK, shadowMatch, false},
		{"MAIN_101_match", "sz000001", 101, 100, 100, 0, tradingrule.BuyQtyReasonAdjusted, shadowMatch, false},
		{"MAIN_99_match0", "sz000001", 99, 0, 0, 0, tradingrule.BuyQtyReasonRejectBelowMin, shadowMatch, false},

		// STAR: legacy floors to 100-lots; policy min=200 step=1
		{"STAR_200_match", "sh688981", 200, 200, 200, 0, tradingrule.BuyQtyReasonOK, shadowMatch, false},
		{"STAR_201_increase", "sh688981", 201, 200, 201, 1, tradingrule.BuyQtyReasonOK, shadowDiffIncrease, false},
		{"STAR_199_decrease", "sh688981", 199, 100, 0, -100, tradingrule.BuyQtyReasonRejectBelowMin, shadowDiffDecrease, true},

		// ETF: same 100-lot as MAIN
		{"ETF_100_match", "sh510300", 100, 100, 100, 0, tradingrule.BuyQtyReasonOK, shadowMatch, false},
		{"ETF_99_match0", "sh510300", 99, 0, 0, 0, tradingrule.BuyQtyReasonRejectBelowMin, shadowMatch, false},

		// CB: legacy /100 often 0 while policy uses 10-lot
		{"CB_10_increase", "sh113052", 10, 0, 10, 10, tradingrule.BuyQtyReasonOK, shadowDiffIncrease, false},
		{"CB_11_increase", "sh113052", 11, 0, 10, 10, tradingrule.BuyQtyReasonAdjusted, shadowDiffIncrease, false},
		{"CB_9_match0", "sh113052", 9, 0, 0, 0, tradingrule.BuyQtyReasonRejectBelowMin, shadowMatch, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obs := ObserveQuantityPolicyShadow(tc.code, tc.amount*px, px)
			require.Equal(t, tc.code, obs.Instrument)
			require.Equal(t, tc.old, obs.OldQuantity, "old")
			require.Equal(t, tc.policy, obs.PolicyQuantity, "policy")
			require.Equal(t, tc.diff, obs.QuantityDiff, "diff")
			require.Equal(t, tc.reason, obs.Reason)
			require.Equal(t, tc.match, obs.MatchStatus)
			require.Equal(t, tc.abnormal, obs.IsAbnormalQuantityDecrease())
		})
	}
}

func TestMaterialize_ShadowDoesNotChangeTargetVolume_FlagOff(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())

	setupDraftPlanTestDB(t)
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
	require.Len(t, res.QuantityShadows, 4)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	byCode := map[string]models.TradePlanItem{}
	for _, it := range got.Items {
		byCode[it.StockCode] = it
	}

	// target_volume must follow legacy /100 only (Flag off).
	require.Equal(t, int64(100), byCode["sz000001"].TargetVolume)  // floor(101/100)*100
	require.Equal(t, int64(200), byCode["sh688981"].TargetVolume)  // floor(201/100)*100 — NOT policy 201
	require.Equal(t, int64(100), byCode["sh510300"].TargetVolume)
	require.Equal(t, int64(0), byCode["sh113052"].TargetVolume) // floor(11/100)*100=0 → size_skip

	shadowBy := map[string]QuantityShadowObservation{}
	for _, s := range res.QuantityShadows {
		shadowBy[s.Instrument] = s
	}
	require.Equal(t, int64(0), shadowBy["sz000001"].QuantityDiff)
	require.Equal(t, int64(1), shadowBy["sh688981"].QuantityDiff)   // policy 201 vs old 200
	require.Equal(t, int64(0), shadowBy["sh510300"].QuantityDiff)
	require.Equal(t, int64(10), shadowBy["sh113052"].QuantityDiff) // policy 10 vs old 0
	require.Equal(t, shadowDiffIncrease, shadowBy["sh688981"].MatchStatus)
	require.Equal(t, shadowDiffIncrease, shadowBy["sh113052"].MatchStatus)
}

func TestObserveQuantityPolicyShadow_DoesNotEnableFlag(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	_ = ObserveQuantityPolicyShadow("sz000001", 10_000, 10)
	require.False(t, tradingrule.EnableQuantityPolicy())
}
