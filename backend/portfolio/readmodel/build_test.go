package readmodel_test

import (
	"testing"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/portfolio/readmodel"

	"github.com/stretchr/testify/require"
)

func TestBuild_EmptyWhenSnapshotMissing(t *testing.T) {
	v := readmodel.Build(readmodel.Options{AsOf: time.Date(2026, 8, 19, 10, 0, 0, 0, time.Local)})
	require.False(t, v.Found)
	require.Empty(t, v.Positions)
	require.Equal(t, 0.0, v.Equity)
}

func TestBuild_AccountingIgnoresDisplayQuotes(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true, Cash: 700_000, AsOf: time.Now(),
		Positions: []portfolio.Position{{
			StockCode: "sh600000", StockName: "浦发",
			Volume: 1000, AvailableVolume: 400, LockedVolume: 600,
			AvgCost: 10, MarkPrice: 12,
			MarketValue: 12_000, UnrealizedPnL: 2000,
		}},
	}
	quotes := map[string]marketdata.Quote{
		"sh600000": {Code: "sh600000", Price: 20, PreClose: 19},
	}
	v := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, Quotes: quotes, TradeDate: "2026-08-19",
	})
	require.True(t, v.Found)
	require.InDelta(t, 700_000, v.Cash, 1e-9)
	require.InDelta(t, 12_000, v.MarketValue, 1e-9)
	require.InDelta(t, 712_000, v.Equity, 1e-9)
	require.Len(t, v.Positions, 1)
	row := v.Positions[0]
	require.Equal(t, int64(1000), row.TotalQty)
	require.Equal(t, int64(400), row.AvailableQty)
	require.Equal(t, int64(600), row.LockedQty)
	require.InDelta(t, 10.0, row.AvgCost, 1e-9)
	require.InDelta(t, 12.0, row.MarkPrice, 1e-9)
	require.InDelta(t, 2000.0, row.PnL, 1e-9)
	require.NotNil(t, row.PnLPercent)
	require.InDelta(t, 0.2, *row.PnLPercent, 1e-9)
	require.Equal(t, positionstate.S3PartialLocked, row.PositionState.State)
	require.NotNil(t, row.DisplayPrice)
	require.InDelta(t, 20.0, *row.DisplayPrice, 1e-9)
	require.Equal(t, readmodel.QuoteSourceLive, row.DisplayQuoteSource)
	require.NotNil(t, row.DisplayPnL)
	require.InDelta(t, 10_000.0, *row.DisplayPnL, 1e-9)
	require.NotNil(t, row.QuotePreClose)
	require.InDelta(t, 19.0, *row.QuotePreClose, 1e-9)
	require.NotNil(t, row.TodayPnL)
	require.InDelta(t, 1_000.0, *row.TodayPnL, 1e-9) // (20-19)*1000
	require.NotEqual(t, row.PnL, *row.TodayPnL)
	require.NotNil(t, row.QuotePrice)
	require.InDelta(t, 20.0, *row.QuotePrice, 1e-9)
	require.Equal(t, readmodel.PriceFreshnessUnknown, row.PriceFreshness) // no FetchedAt
}

func TestBuild_QuoteTruth_FreshnessAndMarkUnchanged(t *testing.T) {
	asOf := time.Date(2026, 9, 7, 15, 5, 0, 0, time.Local)
	snap := &portfolio.Snapshot{
		Found: true, Cash: 100_000, AsOf: asOf,
		Positions: []portfolio.Position{{
			StockCode: "sh600000", Volume: 1000, AvailableVolume: 1000,
			AvgCost: 10, MarkPrice: 12,
		}},
	}
	fetched := asOf.Add(-2 * time.Minute)
	quotes := map[string]marketdata.Quote{
		"sh600000": {Code: "sh600000", Price: 14.25, PreClose: 14.0, FetchedAt: fetched},
	}
	v := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, Quotes: quotes, TradeDate: "2026-09-07", AsOf: asOf,
	})
	row := v.Positions[0]
	require.InDelta(t, 12.0, row.MarkPrice, 1e-9)
	require.InDelta(t, 12_000, v.MarketValue, 1e-9)
	require.InDelta(t, 112_000, v.Equity, 1e-9)
	require.NotNil(t, row.QuotePrice)
	require.InDelta(t, 14.25, *row.QuotePrice, 1e-9)
	require.NotNil(t, row.QuoteTimestamp)
	require.True(t, row.QuoteTimestamp.Equal(fetched))
	require.Equal(t, readmodel.PriceFreshnessFresh, row.PriceFreshness)
	require.NotNil(t, row.TodayPnL)
	require.InDelta(t, 250.0, *row.TodayPnL, 1e-9) // (14.25-14)*1000

	// Quote refresh updates display only — mark/equity unchanged.
	quotes2 := map[string]marketdata.Quote{
		"sh600000": {Code: "sh600000", Price: 15.0, PreClose: 14.0, FetchedAt: asOf},
	}
	v2 := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, Quotes: quotes2, TradeDate: "2026-09-07", AsOf: asOf,
	})
	row2 := v2.Positions[0]
	require.InDelta(t, 12.0, row2.MarkPrice, 1e-9)
	require.InDelta(t, v.Equity, v2.Equity, 1e-9)
	require.NotNil(t, row2.QuotePrice)
	require.InDelta(t, 15.0, *row2.QuotePrice, 1e-9)
	require.Equal(t, readmodel.PriceFreshnessFresh, row2.PriceFreshness)
	require.NotNil(t, row2.TodayPnL)
	require.InDelta(t, 1_000.0, *row2.TodayPnL, 1e-9)
}

func TestBuild_QuoteTruth_StaleAndPersistedUnknown(t *testing.T) {
	asOf := time.Date(2026, 9, 7, 15, 5, 0, 0, time.Local)
	snap := &portfolio.Snapshot{
		Found: true, Cash: 50_000, AsOf: asOf,
		Positions: []portfolio.Position{{
			StockCode: "sz000001", Volume: 100, AvailableVolume: 100,
			AvgCost: 8, MarkPrice: 9,
		}},
	}
	staleAt := asOf.Add(-10 * time.Minute)
	vLive := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, TradeDate: "2026-09-07", AsOf: asOf,
		Quotes: map[string]marketdata.Quote{
			"sz000001": {Code: "sz000001", Price: 9.5, PreClose: 9.2, FetchedAt: staleAt},
		},
	})
	require.Equal(t, readmodel.PriceFreshnessStale, vLive.Positions[0].PriceFreshness)
	require.InDelta(t, 9.0, vLive.Positions[0].MarkPrice, 1e-9)

	vNoQuote := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, TradeDate: "2026-09-07", AsOf: asOf,
	})
	row := vNoQuote.Positions[0]
	require.Nil(t, row.QuotePrice)
	require.Nil(t, row.QuoteTimestamp)
	require.Equal(t, readmodel.PriceFreshnessUnknown, row.PriceFreshness)
	require.Nil(t, row.TodayPnL)
	require.InDelta(t, 9.0, row.MarkPrice, 1e-9)
	require.InDelta(t, row.MarkPrice, *row.DisplayPrice, 1e-9) // overlay falls back for display_price only
}

func TestBuild_TodayPnL_NullWhenPreCloseMissing(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true, Cash: 100_000, AsOf: time.Now(),
		Positions: []portfolio.Position{{
			StockCode: "sz000001", StockName: "平安",
			Volume: 500, AvailableVolume: 500, LockedVolume: 0,
			AvgCost: 10, MarkPrice: 11,
		}},
	}
	quotes := map[string]marketdata.Quote{
		"sz000001": {Code: "sz000001", Price: 12}, // PreClose absent
	}
	v := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: true, Quotes: quotes, TradeDate: "2026-08-19",
	})
	require.Len(t, v.Positions, 1)
	row := v.Positions[0]
	require.NotNil(t, row.DisplayPrice)
	require.Nil(t, row.QuotePreClose)
	require.Nil(t, row.TodayPnL)
}

func TestBuild_TodayPnL_NullWhenNoDisplayOverlay(t *testing.T) {
	snap := &portfolio.Snapshot{
		Found: true, Cash: 100_000, AsOf: time.Now(),
		Positions: []portfolio.Position{{
			StockCode: "sz000001", Volume: 500, AvailableVolume: 500,
			AvgCost: 10, MarkPrice: 11,
		}},
	}
	v := readmodel.Build(readmodel.Options{
		Snapshot: snap, IncludeDisplay: false, TradeDate: "2026-08-19",
	})
	require.Len(t, v.Positions, 1)
	require.Nil(t, v.Positions[0].TodayPnL)
	require.Nil(t, v.Positions[0].QuotePreClose)
}
