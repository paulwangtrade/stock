package papertrading_test

import (
	"fmt"
	"testing"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPaperObsTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:phase10f_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestBuildExecutionSummary_EmptyAndNoSlippage(t *testing.T) {
	setupPaperObsTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, db.Dao.Create(acc).Error)

	view, err := papertrading.BuildExecutionSummary("")
	require.NoError(t, err)
	require.Equal(t, 0, view.TotalOrders)
	require.Nil(t, view.AvgSlippage)
	require.Equal(t, 0.0, view.FillRate)

	require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 1, PlanItemID: 1, TradeDate: "2026-08-09", StockCode: "sz000001",
		Side: "buy", Quantity: 100, Status: papertrading.OrderStatusFilled,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 1, PlanItemID: 2, TradeDate: "2026-08-09", StockCode: "sz000002",
		Side: "buy", Quantity: 100, Status: papertrading.OrderStatusRejected,
	}).Error)
	view, err = papertrading.BuildExecutionSummary("2026-08-09")
	require.NoError(t, err)
	require.Equal(t, 2, view.TotalOrders)
	require.Equal(t, 1, view.FilledOrders)
	require.Equal(t, 1, view.FailedOrders)
	require.InDelta(t, 0.5, view.FillRate, 1e-9)
	require.Nil(t, view.AvgSlippage, "must not invent slippage")
}

func TestBuildRiskObservation_Weights(t *testing.T) {
	setupPaperObsTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 800_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 10,
	}).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发银行",
		TotalVolume: 1000, AvgCost: 10, MarkPrice: 30,
	}).Error)

	view, err := papertrading.BuildRiskObservation()
	require.NoError(t, err)
	require.Equal(t, "OK", view.Quality)
	require.NotNil(t, view.Cash)
	require.InDelta(t, 800_000, *view.Cash, 1e-6)
	require.NotNil(t, view.PositionValue)
	require.InDelta(t, 40_000, *view.PositionValue, 1e-6)
	require.NotNil(t, view.TotalAsset)
	require.InDelta(t, 840_000, *view.TotalAsset, 1e-6)
	require.NotNil(t, view.Concentration)
	require.InDelta(t, 0.75, *view.Concentration, 1e-6)
}

func TestBuildRiskObservation_DisabledUnknown(t *testing.T) {
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: false})
	t.Cleanup(papertrading.ResetConfigCache)
	view, err := papertrading.BuildRiskObservation()
	require.NoError(t, err)
	require.Equal(t, "UNKNOWN", view.Quality)
	require.Nil(t, view.TotalAsset)
}
