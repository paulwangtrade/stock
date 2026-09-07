package portfolio_test

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"

	"github.com/stretchr/testify/require"
)

func TestDashboard_EmptyAccount(t *testing.T) {
	setupPortfolioDB(t)
	view, err := portfolio.NewService().Dashboard(portfolio.DashboardOptions{
		TradeDate: "2026-08-17",
		AsOf:      time.Date(2026, 8, 17, 10, 0, 0, 0, time.Local),
	})
	require.NoError(t, err)
	require.False(t, view.Found)
	require.Equal(t, "2026-08-17", view.TradeDate)
	require.Equal(t, 0.0, view.Summary.Equity)
	require.Equal(t, 0, view.Summary.PositionCount)
	require.Nil(t, view.Summary.DailyPnL)
	require.Equal(t, portfolio.DailyPnLBasisUnavailable, view.Summary.DailyPnLBasis)
	require.Empty(t, view.Positions)
	require.Empty(t, view.Trades.Fills)
	require.Equal(t, portfolio.RiskLevelUnknown, view.Risk.RiskLevel)
	require.Nil(t, view.Risk.IndustryConcentration)

	var n int64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Count(&n).Error)
	require.Equal(t, int64(0), n, "empty dashboard must not create account")
}

func TestDashboard_WithPositions(t *testing.T) {
	setupPortfolioDB(t)
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 700_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600000", StockName: "浦发",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 12,
	}).Error)
	// Prior settlement snapshot for daily_pnl
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimDailyReport{
		AccountID: acc.ID, ReportDate: "2026-08-14", Cash: 700_000, Equity: 710_000, MarketValue: 10_000,
	}).Error)

	view, err := portfolio.NewService().Dashboard(portfolio.DashboardOptions{
		TradeDate: "2026-08-17",
		AsOf:      time.Date(2026, 8, 17, 15, 0, 0, 0, time.Local),
	})
	require.NoError(t, err)
	require.True(t, view.Found)
	require.InDelta(t, 700_000, view.Summary.Cash, 1e-9)
	require.InDelta(t, 12_000, view.Summary.MarketValue, 1e-9)
	require.InDelta(t, 712_000, view.Summary.Equity, 1e-9)
	require.Equal(t, 1, view.Summary.PositionCount)
	require.NotNil(t, view.Summary.DailyPnL)
	require.InDelta(t, 712_000-710_000, *view.Summary.DailyPnL, 1e-9)
	require.Equal(t, portfolio.DailyPnLBasisReportDelta, view.Summary.DailyPnLBasis)

	require.Len(t, view.Positions, 1)
	require.Equal(t, "sh600000", view.Positions[0].StockCode)
	require.Equal(t, "浦发", view.Positions[0].StockName)
	require.Equal(t, int64(1000), view.Positions[0].Quantity)
	require.InDelta(t, 10.0, view.Positions[0].AvgCost, 1e-9)
	require.InDelta(t, 12.0, view.Positions[0].MarketPrice, 1e-9)
	require.InDelta(t, 2000.0, view.Positions[0].UnrealizedPnL, 1e-9)

	require.InDelta(t, 12_000/712_000.0, view.Risk.MaxPositionRatio, 1e-9)
	require.Equal(t, portfolio.RiskLevelLow, view.Risk.RiskLevel)
	require.Nil(t, view.Risk.IndustryConcentration)
	require.Empty(t, view.Trades.Fills)

	// No writes to cash/mark
	var cash float64
	require.NoError(t, db.Dao.Model(&papertrading.PaperSimAccount{}).Where("id = ?", acc.ID).Pluck("cash", &cash).Error)
	require.InDelta(t, 700_000, cash, 1e-9)
}

func TestDashboard_WithFills(t *testing.T) {
	setupPortfolioDB(t)
	acc := &papertrading.PaperSimAccount{
		Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 900_000,
	}
	require.NoError(t, db.Dao.Create(acc).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安",
		TotalVolume: 100, AvailableVolume: 0, LockedVolume: 100, AvgCost: 10, MarkPrice: 10.5,
	}).Error)

	order := papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 9, PlanItemID: 1, TradeDate: "2026-08-17",
		StockCode: "sz000001", StockName: "平安", Side: "buy", Quantity: 100,
		OrderPrice: 10, Status: papertrading.OrderStatusFilled, FilledPrice: 10, FilledVolume: 100,
		OrderTime: time.Date(2026, 8, 17, 9, 31, 0, 0, time.Local),
	}
	require.NoError(t, db.Dao.Create(&order).Error)
	fill := papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: order.ID, PlanID: 9, PlanItemID: 1,
		StockCode: "sz000001", StockName: "平安", Side: "buy", Price: 10, Volume: 100,
		FillReason: papertrading.FillReasonMarketOpen,
		FilledAt:   time.Date(2026, 8, 17, 9, 31, 5, 0, time.Local),
	}
	require.NoError(t, db.Dao.Create(&fill).Error)
	// Other-day fill must not appear
	otherOrder := papertrading.PaperSimOrder{
		AccountID: acc.ID, PlanID: 8, PlanItemID: 1, TradeDate: "2026-08-14",
		StockCode: "sz000001", Side: "buy", Quantity: 50, Status: papertrading.OrderStatusFilled,
		FilledPrice: 9, FilledVolume: 50, OrderTime: time.Date(2026, 8, 14, 9, 31, 0, 0, time.Local),
	}
	require.NoError(t, db.Dao.Create(&otherOrder).Error)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimFill{
		AccountID: acc.ID, OrderID: otherOrder.ID, PlanID: 8, StockCode: "sz000001",
		Side: "buy", Price: 9, Volume: 50, FilledAt: time.Date(2026, 8, 14, 9, 31, 0, 0, time.Local),
	}).Error)

	view, err := portfolio.NewService().Dashboard(portfolio.DashboardOptions{TradeDate: "2026-08-17"})
	require.NoError(t, err)
	require.True(t, view.Found)
	require.Len(t, view.Trades.Fills, 1)
	require.Equal(t, "2026-08-17", view.Trades.TradeDate)
	f := view.Trades.Fills[0]
	require.Equal(t, "sz000001", f.StockCode)
	require.Equal(t, "buy", f.Side)
	require.Equal(t, int64(100), f.Volume)
	require.InDelta(t, 10.0, f.Price, 1e-9)
	require.Equal(t, uint(9), f.PlanID)
	require.Equal(t, order.ID, f.OrderID)
}

func TestProjectDashboard_RiskLevels(t *testing.T) {
	asOf := time.Date(2026, 8, 17, 12, 0, 0, 0, time.Local)
	acc := &papertrading.PaperSimAccount{ID: 1, Name: "paper_sim_default", Cash: 100}
	// Single name dominates equity → HIGH
	snap := portfolio.ProjectSnapshot(asOf, acc, []papertrading.PaperSimPosition{
		{StockCode: "sh600000", TotalVolume: 100, AvgCost: 10, MarkPrice: 10}, // MV=1000, equity=1100, w≈0.91
	})
	view := portfolio.ProjectDashboard(asOf, "2026-08-17", snap, 0, false, nil)
	require.Equal(t, portfolio.RiskLevelHigh, view.Risk.RiskLevel)
	require.Nil(t, view.Risk.IndustryConcentration)
}
