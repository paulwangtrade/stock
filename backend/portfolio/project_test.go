package portfolio

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestProjectSnapshot_NilAccount(t *testing.T) {
	out := ProjectSnapshot(time.Time{}, nil, nil)
	require.False(t, out.Found)
	require.Equal(t, 0.0, out.Cash)
	require.Equal(t, 0.0, out.TotalEquity)
	require.Equal(t, 0, out.PositionCount)
}

func TestProjectSnapshot_CashOnly(t *testing.T) {
	asOf := time.Date(2026, 8, 11, 15, 0, 0, 0, time.Local)
	acc := &papertrading.PaperSimAccount{ID: 7, Name: "paper_sim_default", Cash: 800_000, Equity: 800_000}
	out := ProjectSnapshot(asOf, acc, nil)
	require.True(t, out.Found)
	require.Equal(t, uint(7), out.AccountID)
	require.InDelta(t, 800_000, out.Cash, 1e-9)
	require.InDelta(t, 800_000, out.AvailableCash, 1e-9)
	require.Equal(t, 0.0, out.ReservedCash)
	require.Equal(t, 0.0, out.MarketValue)
	require.InDelta(t, 800_000, out.TotalEquity, 1e-9)
	require.Equal(t, 0, out.PositionCount)
	require.Equal(t, 0.0, out.TotalExposure)
}

func TestProjectSnapshot_TwoNamesWeightsAndExposure(t *testing.T) {
	asOf := time.Date(2026, 8, 11, 15, 0, 0, 0, time.Local)
	acc := &papertrading.PaperSimAccount{ID: 1, Name: "paper_sim_default", Cash: 800_000}
	positions := []papertrading.PaperSimPosition{
		{StockCode: "sh600000", StockName: "浦发", TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10, MarkPrice: 12},
		{StockCode: "sz000001", StockName: "平安", TotalVolume: 2000, AvailableVolume: 1500, LockedVolume: 500, AvgCost: 20, MarkPrice: 18},
		{StockCode: "sz000002", TotalVolume: 0, MarkPrice: 99}, // skipped
	}
	out := ProjectSnapshot(asOf, acc, positions)
	require.Equal(t, 2, out.PositionCount)
	require.InDelta(t, 12.0*1000+18.0*2000, out.MarketValue, 1e-9)
	require.Equal(t, out.MarketValue, out.TotalExposure)
	require.InDelta(t, 800_000+out.MarketValue, out.TotalEquity, 1e-9)
	require.Len(t, out.Positions, 2)
	require.InDelta(t, (12.0*1000)/out.TotalEquity, out.Positions[0].Weight, 1e-9)
	require.InDelta(t, (18.0*2000)/out.TotalEquity, out.Positions[1].Weight, 1e-9)
	require.InDelta(t, (12.0-10.0)*1000, out.Positions[0].UnrealizedPnL, 1e-9)
}

func TestProjectSnapshot_UsesPersistedMarkNotAccountMarketValue(t *testing.T) {
	acc := &papertrading.PaperSimAccount{ID: 1, Name: "paper_sim_default", Cash: 100, MarketValue: 999999, Equity: 999999}
	positions := []papertrading.PaperSimPosition{
		{StockCode: "sh600000", TotalVolume: 100, MarkPrice: 10, AvgCost: 8},
	}
	out := ProjectSnapshot(time.Now(), acc, positions)
	require.InDelta(t, 1000.0, out.MarketValue, 1e-9)
	require.InDelta(t, 1100.0, out.TotalEquity, 1e-9)
	require.NotEqual(t, acc.Equity, out.TotalEquity)
}
