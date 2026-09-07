package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestExecutionReadService_ListOrders_PrefersSim(t *testing.T) {
	setupPaperObsTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1e6})
	t.Cleanup(papertrading.ResetConfigCache)

	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 9, PlanItemID: 1, TradeDate: "2026-08-09",
		StockCode: "sz000001", Side: "buy", Quantity: 100, Status: papertrading.OrderStatusFilled,
	}).Error)

	svc := papertrading.DefaultExecutionReadService()
	orders, src, err := svc.ListOrders(papertrading.ExecutionOrderFilter{TradeDate: "2026-08-09", PlanID: 9})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionReadSourcePaperSim, src)
	require.Len(t, orders, 1)
	require.Equal(t, "filled", orders[0].Status)
}

func TestExecutionReadService_ListOrders_LegacyFallback(t *testing.T) {
	setupPaperObsTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&data.PaperOrder{}))
	require.NoError(t, db.Dao.Create(&data.PaperOrder{
		StockCode: "sz000001", Side: "buy", Status: data.PaperOrderStatusFilled,
		Price: 10, Volume: 100, FilledPrice: 10, FilledVol: 100,
		CreatedAt: time.Date(2026, 8, 9, 10, 0, 0, 0, time.Local),
		UpdatedAt: time.Date(2026, 8, 9, 10, 0, 0, 0, time.Local),
	}).Error)

	svc := &papertrading.SimExecutionReadService{}
	orders, src, err := svc.ListOrders(papertrading.ExecutionOrderFilter{TradeDate: "2026-08-09"})
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionReadSourceLegacyFallback, src)
	require.Len(t, orders, 1)
}

func TestExecutionReadService_PlanSummary_UsesSimNotLegacyOnly(t *testing.T) {
	setupPaperObsTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1e6})
	t.Cleanup(papertrading.ResetConfigCache)

	plan := &models.TradePlan{TradeDate: "2026-08-09", Status: models.TradePlanStatusExecuting}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := models.TradePlanItem{
		PlanID: plan.ID, StockCode: "sz000001", Side: "buy",
		Status: models.TradePlanItemPending, OrderID: 0, // no legacy order id
	}
	require.NoError(t, db.Dao.Create(&item).Error)

	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: "2026-08-09",
		StockCode: "sz000001", Side: "buy", Quantity: 100, Status: papertrading.OrderStatusFilled,
		FilledVolume: 100, FilledPrice: 10,
	}).Error)

	view, err := papertrading.DefaultExecutionReadService().BuildPlanExecutionSummary(plan.ID)
	require.NoError(t, err)
	require.Equal(t, papertrading.ExecutionReadSourcePaperSim, view.DataSource)
	require.Equal(t, 1, view.OrderFilledCount)
	require.Equal(t, 1, view.EffectiveFilled)
	require.Equal(t, 0, view.StillPending)

	// Wire: data.GetPlanExecutionSummary must see sim fills (LEG-1).
	sum, err := data.NewTradePlanRepo().GetPlanExecutionSummary(plan.ID)
	require.NoError(t, err)
	require.Equal(t, 1, sum.EffectiveFilled)
	require.Equal(t, 1, sum.OrderFilledCount)
}

func TestBuildExecutionSummary_ViaReadService(t *testing.T) {
	setupPaperObsTestDB(t)
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1e6})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1e6, Cash: 1e6}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 1, PlanItemID: 1, TradeDate: "2026-08-09",
		StockCode: "sz000001", Side: "buy", Quantity: 100, Status: papertrading.OrderStatusFilled,
	}).Error)

	view, err := papertrading.BuildExecutionSummary("2026-08-09")
	require.NoError(t, err)
	require.Equal(t, 1, view.TotalOrders)
	require.Contains(t, view.DataSourceNote, papertrading.ExecutionReadSourcePaperSim)
}
