package positionstate_test

import (
	"testing"

	"go-stock/backend/portfolio/positionstate"

	"github.com/stretchr/testify/require"
)

func TestCase1_YesterdayBuy_TodayAvailable_S2(t *testing.T) {
	view := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol:       "sz000001",
		TotalQty:     1000,
		AvailableQty: 1000,
		BuyRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-14", Quantity: 1000, Side: "BUY"},
		},
		TradeDate:   "2026-08-15",
		CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S2Available, view.State)
	require.Equal(t, int64(1000), view.AvailableQty)
	require.Equal(t, int64(0), view.LockedQty)
	require.False(t, view.IsNewPosition)
	require.True(t, view.CanSell)
	require.Greater(t, view.HoldingDays, 0)
}

func TestCase2_TodayBuy_S1NewLocked(t *testing.T) {
	view := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol:       "sz000001",
		TotalQty:     1000,
		AvailableQty: 0,
		BuyRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-15", Quantity: 1000, Side: "BUY"},
		},
		TradeDate:   "2026-08-15",
		CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S1NewLocked, view.State)
	require.True(t, view.IsNewPosition)
	require.False(t, view.CanSell)
	require.Equal(t, int64(1000), view.LockedQty)
	require.Equal(t, 0, view.HoldingDays)
}

func TestCase3_HistoryPlusTodayAdd_S3Partial(t *testing.T) {
	locked := int64(1000)
	view := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol:       "sz000001",
		TotalQty:     4000,
		AvailableQty: 3000,
		LockedQty:    &locked,
		BuyRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-01", Quantity: 3000, Side: "BUY"},
			{TradeDate: "2026-08-15", Quantity: 1000, Side: "BUY"},
		},
		TradeDate:   "2026-08-15",
		CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S3PartialLocked, view.State)
	require.False(t, view.IsNewPosition) // first buy is historical
	require.True(t, view.CanSell)
	require.Equal(t, int64(3000), view.AvailableQty)
	require.Equal(t, int64(1000), view.LockedQty)
}

func TestCase4_PartialSell_S4Reduced(t *testing.T) {
	view := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol:       "sz000001",
		TotalQty:     2000,
		AvailableQty: 2000,
		BuyRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-01", Quantity: 5000, Side: "BUY"},
		},
		SellRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-15", Quantity: 3000, Side: "SELL"},
		},
		TradeDate:   "2026-08-15",
		CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S4ReducedAvailable, view.State)
	require.False(t, view.IsNewPosition)
	require.True(t, view.CanSell)
	require.Equal(t, int64(2000), view.AvailableQty)
}

func TestBugFix_AvailableZero_NotNewWhenHoldingDaysPositive(t *testing.T) {
	// Legacy bug: available=0 => NEW. Fixed: first_buy_date != today => not new.
	view := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol:       "sz000001",
		TotalQty:     1000,
		AvailableQty: 0,
		BuyRecords: []positionstate.LotRecord{
			{TradeDate: "2026-08-10", Quantity: 1000, Side: "BUY"},
		},
		TradeDate:   "2026-08-15",
		CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S1NewLocked, view.State) // still locked pattern
	require.False(t, view.IsNewPosition)
	require.Equal(t, positionstate.RiskTagStaleLock, view.RiskTag)
	require.Greater(t, view.HoldingDays, 0)
}

func TestMorningUnlock_PreviousDayBuys(t *testing.T) {
	buys := []positionstate.LotRecord{
		{TradeDate: "2026-08-14", Quantity: 1000, Side: "BUY"},
		{TradeDate: "2026-08-15", Quantity: 500, Side: "BUY"},
	}
	require.Equal(t, int64(1000), positionstate.UnlockQtyForSymbol(buys, "2026-08-14"))
	avail, locked := positionstate.ApplyUnlockToQty(1500, 0, 1500, 1000)
	require.Equal(t, int64(1000), avail)
	require.Equal(t, int64(500), locked)
	after := positionstate.Calculate(positionstate.SnapshotInput{
		Symbol: "sz000001", TotalQty: 1500, AvailableQty: avail,
		BuyRecords: buys, TradeDate: "2026-08-15", CurrentDate: "2026-08-15",
	})
	require.Equal(t, positionstate.S3PartialLocked, after.State)
}
