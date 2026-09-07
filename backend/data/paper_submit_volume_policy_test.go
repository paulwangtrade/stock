package data_test

import (
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/tradingrule"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupSubmitPolicyDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=10000"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, data.MigratePaperTrading(testDB))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestSubmitPaperOrder_FlagOff_MAIN101_StillRounds(t *testing.T) {
	setupSubmitPolicyDB(t)
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(100_000)
	require.NoError(t, err)
	order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
		AccountID: acc.ID, StockCode: "sz000001", Side: "buy", Price: 10, Volume: 101, AutoFill: false,
	})
	require.NoError(t, err)
	require.Equal(t, int64(100), order.Volume)
}

func TestSubmitPaperOrder_FlagOn_BoardMatrix(t *testing.T) {
	setupSubmitPolicyDB(t)
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	api := data.NewPaperTradingApi()
	acc, err := api.ResetAccount(1_000_000)
	require.NoError(t, err)

	cases := []struct {
		name string
		code string
		vol  int64
		ok   bool
	}{
		{"MAIN_100", "sz000001", 100, true},
		{"MAIN_101", "sz000001", 101, false},
		{"STAR_199", "sh688981", 199, false},
		{"STAR_200", "sh688981", 200, true},
		{"STAR_201", "sh688981", 201, true},
		{"CB_9", "sh113052", 9, false},
		{"CB_10", "sh113052", 10, true},
		{"CB_11", "sh113052", 11, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			order, err := api.SubmitPaperOrder(data.PaperSubmitOrderReq{
				AccountID: acc.ID, StockCode: tc.code, Side: "buy", Price: 10, Volume: tc.vol, AutoFill: false,
			})
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, tc.vol, order.Volume, "must not normalize")
			} else {
				require.Error(t, err)
				require.Nil(t, order)
			}
		})
	}
}
