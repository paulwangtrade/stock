package readiness_test

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/qualitygate"
	"go-stock/backend/readiness"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestVolumeMeetsBuyLot_FlagOff_Legacy100(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())

	require.Equal(t, int64(100), readiness.LotSize)
	require.True(t, readiness.VolumeMeetsBuyLot("sz000001", 100))
	require.True(t, readiness.VolumeMeetsBuyLot("sz000001", 101)) // legacy: >=100 only
	require.False(t, readiness.VolumeMeetsBuyLot("sz000001", 99))
	// STAR 199 still "ok" under legacy >=100
	require.True(t, readiness.VolumeMeetsBuyLot("sh688981", 199))
	require.Equal(t, int64(100), readiness.EffectiveBuyLotThreshold("sh688981"))
	require.Equal(t, int64(100), readiness.EffectiveBuyLotThreshold("sh113052"))
}

func TestVolumeMeetsBuyLot_FlagOn_MAIN_STAR_ETF_CB(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	cases := []struct {
		name   string
		code   string
		vol    int64
		meets  bool
		minLot int64
	}{
		{"MAIN_100", "sz000001", 100, true, 100},
		{"MAIN_99", "sz000001", 99, false, 100},
		{"MAIN_101", "sz000001", 101, false, 100}, // Validate-only: not aligned
		{"STAR_199", "sh688981", 199, false, 200},
		{"STAR_200", "sh688981", 200, true, 200},
		{"STAR_201", "sh688981", 201, true, 200},
		{"ETF_100", "sh510300", 100, true, 100},
		{"ETF_99", "sh510300", 99, false, 100},
		{"CB_9", "sh113052", 9, false, 10},
		{"CB_10", "sh113052", 10, true, 10},
		{"CB_11", "sh113052", 11, false, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.meets, readiness.VolumeMeetsBuyLot(tc.code, tc.vol))
			require.Equal(t, tc.minLot, readiness.EffectiveBuyLotThreshold(tc.code))
		})
	}
}

func TestEvaluate_FlagOn_BoardMatrix(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	mk := func(code string, vol int64) *models.TradePlan {
		return &models.TradePlan{
			ID: 1, TradeDate: "2026-09-01", Status: models.TradePlanStatusDraft,
			Side: "buy", PricingPolicyVersion: 1, PricingStage: "morning_materialized",
			Items: []models.TradePlanItem{{
				StockCode: code, Side: "buy", IntentStatus: readiness.IntentPriced,
				LimitPrice: 10, TargetVolume: vol, TargetAmount: 100_000,
			}},
		}
	}
	opts := &readiness.Options{SkipQualityGate: true}

	t.Run("MAIN_100_ready", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sz000001", 100), opts)
		require.True(t, res.Ready)
		require.Equal(t, 1, res.TradeableCount)
	})
	t.Run("STAR_201_ready", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sh688981", 201), opts)
		require.True(t, res.Ready)
		require.Equal(t, 1, res.TradeableCount)
	})
	t.Run("STAR_199_blocked", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sh688981", 199), opts)
		require.False(t, res.Ready)
		require.Equal(t, 0, res.TradeableCount)
		require.True(t, hasCode(res.Blockers, readiness.CodeVolumeBelowLot))
		var lotBlock *readiness.Finding
		for i := range res.Blockers {
			if res.Blockers[i].Code == readiness.CodeVolumeBelowLot {
				lotBlock = &res.Blockers[i]
				break
			}
		}
		require.NotNil(t, lotBlock)
		require.Equal(t, readiness.LotSize, lotBlock.Evidence["lot_size"])
	})
	t.Run("ETF_100_ready", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sh510300", 100), opts)
		require.True(t, res.Ready)
	})
	t.Run("CB_10_ready", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sh113052", 10), opts)
		require.True(t, res.Ready)
	})
	t.Run("CB_11_blocked", func(t *testing.T) {
		res := readiness.EvaluateExecutionIntentReadiness(mk("sh113052", 11), opts)
		require.False(t, res.Ready)
		require.True(t, hasCode(res.Blockers, readiness.CodeVolumeBelowLot))
	})
}

func TestEvaluate_FlagOff_STAR199_StillTradeableLegacy(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	plan := &models.TradePlan{
		ID: 1, TradeDate: "2026-09-01", Status: models.TradePlanStatusDraft,
		Side: "buy", PricingPolicyVersion: 1, PricingStage: "morning_materialized",
		Items: []models.TradePlanItem{{
			StockCode: "sh688981", Side: "buy", IntentStatus: readiness.IntentPriced,
			LimitPrice: 10, TargetVolume: 199, TargetAmount: 100_000,
		}},
	}
	res := readiness.EvaluateExecutionIntentReadiness(plan, &readiness.Options{
		SkipQualityGate: true,
		MarketData:      qualitygate.MarketDataSnapshot{SkipGapEval: true},
	})
	require.True(t, res.Ready, "Flag OFF keeps >=100 legacy: %+v", res.Blockers)
	require.Equal(t, 1, res.TradeableCount)
}

func hasCode(fs []readiness.Finding, code string) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}
