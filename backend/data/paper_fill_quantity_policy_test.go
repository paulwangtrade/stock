package data_test

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestFillPaperOrderQty_FlagOff_StillRequiresHundredLot(t *testing.T) {
	setupSubmitPolicyDB(t)
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sz000001", Side: "buy", Price: 10, Volume: 200, AutoFill: false,
	})
	require.NoError(t, err)
	require.Equal(t, int64(200), order.Volume)

	require.Error(t, api.FillPaperOrderQty(order.ID, 10, 101))
	require.NoError(t, api.FillPaperOrderQty(order.ID, 10, 100))
}

func TestFillPaperOrderQty_FlagOn_BoardMatrix(t *testing.T) {
	setupSubmitPolicyDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(1_000_000)
	require.NoError(t, err)

	cases := []struct {
		name     string
		code     string
		orderVol int64
		fillQty  int64
		ok       bool
	}{
		{"MAIN_100", "sz000001", 100, 100, true},
		{"MAIN_101", "sz000001", 200, 101, false},
		{"STAR_199", "sh688981", 200, 199, false},
		{"STAR_200", "sh688981", 200, 200, true},
		{"STAR_201", "sh688981", 201, 201, true},
		{"CB_9", "sh113052", 10, 9, false},
		{"CB_10", "sh113052", 10, 10, true},
		{"CB_11", "sh113052", 20, 11, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
				AccountID: acc.ID, StockCode: tc.code, Side: "buy", Price: 10, Volume: tc.orderVol, AutoFill: false,
			})
			require.NoError(t, err, "setup order must pass QuantityPolicy")
			require.Equal(t, tc.orderVol, order.Volume, "Submit must not normalize")

			ferr := api.FillPaperOrderQty(order.ID, 10, tc.fillQty)
			if tc.ok {
				require.NoError(t, ferr)
			} else {
				require.Error(t, ferr)
			}
		})
	}
}

func TestFillPaperOrderQty_FlagOn_DoesNotRewriteQty(t *testing.T) {
	setupSubmitPolicyDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sh113052", Side: "buy", Price: 10, Volume: 20, AutoFill: false,
	})
	require.NoError(t, err)

	// CB 11 is illegal — Fill must reject, not normalize to 10.
	require.Error(t, api.FillPaperOrderQty(order.ID, 10, 11))

	var refreshed data.PaperOrder
	require.NoError(t, db.Dao.First(&refreshed, order.ID).Error)
	require.Equal(t, int64(0), refreshed.FilledVol)
	require.Equal(t, int64(20), refreshed.Volume)
}
