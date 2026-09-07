package papertrading_test

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/papertrading"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestPaperBroker_QuantityPolicy_FlagOff_IgnoresIllegalSTAR(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)
	require.False(t, tradingrule.EnableQuantityPolicy())

	// Flag off: legacy still trusts TargetVolume=199 on STAR (no Policy reject).
	plan := seedFrozenPlan(t, "2026-08-16", []models.TradePlanItem{
		buyItem("sh688981", "中芯国际", 199),
	})
	broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
		Quotes: map[string]papertrading.Quote{"sh688981": {Open: 50.0, LimitUp: 60.0}},
	})
	res, err := broker.RunForPlan(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, res.FilledCount)
	require.Equal(t, 0, res.RejectCount)

	status, err := papertrading.GetPlanPaperStatus(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.OrderStatusFilled, status.Orders[0].Status)
	require.Equal(t, int64(199), status.Orders[0].Quantity)
}

func TestPaperBroker_QuantityPolicy_FlagOn_STAR(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	cases := []struct {
		name   string
		vol    int64
		filled bool
		qty    int64
	}{
		{"reject_199", 199, false, 0},
		{"accept_200", 200, true, 200},
		{"accept_201", 201, true, 201},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			date := "2026-08-16"
			code := "sh688981"
			plan := seedFrozenPlan(t, date, []models.TradePlanItem{
				buyItem(code, "中芯国际", tc.vol),
			})
			// Unique trade date per case to avoid account cash coupling surprises across subtests.
			_ = i
			broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
				Quotes: map[string]papertrading.Quote{code: {Open: 50.0, LimitUp: 60.0}},
			})
			res, err := broker.RunForPlan(plan.ID)
			require.NoError(t, err)
			status, err := papertrading.GetPlanPaperStatus(plan.ID)
			require.NoError(t, err)
			if tc.filled {
				require.Equal(t, 1, res.FilledCount)
				require.Equal(t, papertrading.OrderStatusFilled, status.Orders[0].Status)
				require.Equal(t, tc.qty, status.Orders[0].Quantity)
				require.Equal(t, tc.qty, status.Orders[0].FilledVolume)
			} else {
				require.Equal(t, 1, res.RejectCount)
				require.Equal(t, papertrading.OrderStatusRejected, status.Orders[0].Status)
				require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
			}
		})
	}
}

func TestPaperBroker_QuantityPolicy_FlagOn_CB(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	t.Run("reject_9", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-17", []models.TradePlanItem{
			buyItem("sh113052", "兴业转债", 9),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sh113052": {Open: 120.0, LimitUp: 140.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.RejectCount)
		status, err := papertrading.GetPlanPaperStatus(plan.ID)
		require.NoError(t, err)
		require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
	})

	t.Run("accept_10", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-18", []models.TradePlanItem{
			buyItem("sh113052", "兴业转债", 10),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sh113052": {Open: 120.0, LimitUp: 140.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.FilledCount)
		status, err := papertrading.GetPlanPaperStatus(plan.ID)
		require.NoError(t, err)
		require.Equal(t, int64(10), status.Orders[0].FilledVolume)
	})

	t.Run("reject_11_no_normalize", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-19", []models.TradePlanItem{
			buyItem("sh113052", "兴业转债", 11),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sh113052": {Open: 120.0, LimitUp: 140.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.RejectCount)
		status, err := papertrading.GetPlanPaperStatus(plan.ID)
		require.NoError(t, err)
		require.Equal(t, papertrading.OrderStatusRejected, status.Orders[0].Status)
		require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
	})
}

func TestPaperBroker_QuantityPolicy_FlagOn_MAIN_ETF(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	t.Run("MAIN_100_accept", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-20", []models.TradePlanItem{
			buyItem("sz000001", "平安银行", 100),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.FilledCount)
	})

	t.Run("MAIN_99_reject", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-21", []models.TradePlanItem{
			buyItem("sz000001", "平安银行", 99),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.RejectCount)
	})

	t.Run("MAIN_101_reject_no_normalize", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-22", []models.TradePlanItem{
			buyItem("sz000001", "平安银行", 101),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sz000001": {Open: 10.0, LimitUp: 11.0}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.RejectCount)
		status, err := papertrading.GetPlanPaperStatus(plan.ID)
		require.NoError(t, err)
		require.Equal(t, papertrading.RejectInvalidQuantity, status.Orders[0].RejectReason)
	})

	t.Run("ETF_100_accept", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-23", []models.TradePlanItem{
			buyItem("sh510300", "沪深300ETF", 100),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.0, LimitUp: 4.5}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.FilledCount)
	})

	t.Run("ETF_99_reject", func(t *testing.T) {
		plan := seedFrozenPlan(t, "2026-08-24", []models.TradePlanItem{
			buyItem("sh510300", "沪深300ETF", 99),
		})
		broker := papertrading.NewPaperBroker(papertrading.StaticPriceProvider{
			Quotes: map[string]papertrading.Quote{"sh510300": {Open: 4.0, LimitUp: 4.5}},
		})
		res, err := broker.RunForPlan(plan.ID)
		require.NoError(t, err)
		require.Equal(t, 1, res.RejectCount)
	})
}
