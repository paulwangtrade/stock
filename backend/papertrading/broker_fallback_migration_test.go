package papertrading_test

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestPaperBroker_FlagOn_NoAmountFallback_RejectsMissingVolume(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	// TargetVolume=0 but amount would have yielded lots under legacy /100.
	item := models.TradePlanItem{
		StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetVolume: 0, TargetAmount: 100_000, LimitPrice: 10,
	}
	plan := seedFrozenPlan(t, "2026-09-20", []models.TradePlanItem{item})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.RejectCount)
	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
}

func TestPaperBroker_FlagOff_AmountFallback_StillWorks(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())

	item := models.TradePlanItem{
		StockCode: "sz000001", StockName: "平安银行", Side: "buy",
		Status: models.TradePlanItemPending, TargetVolume: 0, TargetAmount: 100_000, LimitPrice: 10,
	}
	plan := seedFrozenPlan(t, "2026-09-21", []models.TradePlanItem{item})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.FilledCount)
	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, int64(10000), status.Orders[0].FilledVolume) // 100000/10/100*100
}

func TestPaperBroker_FlagOn_C1_BoardMatrix(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	cases := []struct {
		name   string
		date   string
		code   string
		nameCN string
		vol    int64
		open   float64
		filled bool
	}{
		{"MAIN_100", "2026-09-22", "sz000001", "平安银行", 100, 10, true},
		{"STAR_199", "2026-09-23", "sh688981", "中芯国际", 199, 50, false},
		{"STAR_200", "2026-09-24", "sh688981", "中芯国际", 200, 50, true},
		{"CB_10", "2026-09-25", "sh113052", "兴业转债", 10, 120, true},
		{"CB_11", "2026-09-26", "sh113052", "兴业转债", 11, 120, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := seedFrozenPlan(t, tc.date, []models.TradePlanItem{
				buyItem(tc.code, tc.nameCN, tc.vol),
			})
			broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
				Quotes: map[string]papertrading.Quote{tc.code: {Open: tc.open, LimitUp: tc.open * 1.2}},
			})
			res, err := broker.RunForPlan(plan.ID)
			require.NoError(t, err)
			status, err := papertrading.GetPlanPaperStatus(plan.ID)
			require.NoError(t, err)
			if tc.filled {
				require.Equal(t, 1, res.FilledCount)
				require.Equal(t, tc.vol, status.Orders[0].FilledVolume)
			} else {
				require.Equal(t, 1, res.RejectCount)
				require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
			}
		})
	}
}
