package papertrading_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go-stock/backend/papertrading"
)

func tBarsNearCostPullbackBounce() []papertrading.HoldingTSignalBar {
	// Local high 10.20, drift down near cost 10.00, then bounce.
	prices := []struct{ h, l, c float64 }{
		{10.10, 10.00, 10.05},
		{10.15, 10.05, 10.12},
		{10.20, 10.10, 10.18},
		{10.18, 10.08, 10.10},
		{10.12, 10.02, 10.04},
		{10.08, 9.98, 10.00},
		{10.05, 9.97, 9.99},
		{10.06, 9.98, 10.03}, // bounce, near cost
	}
	bars := make([]papertrading.HoldingTSignalBar, len(prices))
	for i, p := range prices {
		bars[i] = papertrading.HoldingTSignalBar{
			Time:  fmt.Sprintf("10:%02d", i*5),
			High:  p.h,
			Low:   p.l,
			Close: p.c,
		}
	}
	return bars
}

func tBarsRiseAwayFade() []papertrading.HoldingTSignalBar {
	// Rise from ~10 to ~10.28 then fade last bar.
	prices := []struct{ h, l, c float64 }{
		{10.05, 9.95, 10.00},
		{10.10, 10.00, 10.08},
		{10.15, 10.05, 10.12},
		{10.20, 10.10, 10.18},
		{10.25, 10.15, 10.22},
		{10.28, 10.18, 10.26},
		{10.30, 10.20, 10.28},
		{10.29, 10.18, 10.20}, // fade
	}
	bars := make([]papertrading.HoldingTSignalBar, len(prices))
	for i, p := range prices {
		bars[i] = papertrading.HoldingTSignalBar{
			Time:  fmt.Sprintf("11:%02d", i*5),
			High:  p.h,
			Low:   p.l,
			Close: p.c,
		}
	}
	return bars
}

func TestBuildHoldingTSignal_NormalBuyWatch(t *testing.T) {
	asOf := time.Date(2026, 9, 8, 10, 35, 0, 0, time.Local)
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:        "sh600000",
		PositionID:       "42",
		HasPosition:      true,
		CanSell:          true,
		AvailableQty:     1000,
		CostPrice:        10.0,
		Freshness:        papertrading.EvalFreshnessFresh,
		SuitabilityLevel: papertrading.TSuitabilitySuitable,
		HealthGrade:      papertrading.HealthGradeB,
		Bars:             tBarsNearCostPullbackBounce(),
		AsOf:             asOf,
	})
	require.Empty(t, res.Notes)
	var buy *papertrading.HoldingTSignal
	for i := range res.Signals {
		if res.Signals[i].SignalType == papertrading.TSignalBuyWatch {
			buy = &res.Signals[i]
		}
	}
	require.NotNil(t, buy, "expected T_BUY_WATCH, got %+v", res.Signals)
	require.Equal(t, "sh600000", buy.StockCode)
	require.Equal(t, "42", buy.PositionID)
	require.InDelta(t, 10.03, buy.SignalPrice, 1e-9)
	require.Contains(t, buy.Reasons, papertrading.TSignalReasonNearCost)
	require.Greater(t, buy.Confidence, 0.0)
	require.LessOrEqual(t, buy.Confidence, 1.0)
}

func TestBuildHoldingTSignal_NormalSellWatch(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:        "sz000001",
		HasPosition:      true,
		CanSell:          true,
		AvailableQty:     500,
		CostPrice:        10.0,
		Freshness:        papertrading.EvalFreshnessFresh,
		SuitabilityLevel: papertrading.TSuitabilitySuitable,
		HealthGrade:      papertrading.HealthGradeA,
		Bars:             tBarsRiseAwayFade(),
		AsOf:             time.Now(),
	})
	var sell *papertrading.HoldingTSignal
	for i := range res.Signals {
		if res.Signals[i].SignalType == papertrading.TSignalSellWatch {
			sell = &res.Signals[i]
		}
	}
	require.NotNil(t, sell, "expected T_SELL_WATCH, got %+v notes=%v", res.Signals, res.Notes)
	require.Contains(t, sell.Reasons, papertrading.TSignalReasonAwayCost)
	require.Contains(t, sell.Reasons, papertrading.TSignalReasonMomentumFade)
}

func TestBuildHoldingTSignal_CannotSell_NoBuy(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:    "sh600000",
		HasPosition:  true,
		CanSell:      false,
		AvailableQty: 0,
		CostPrice:    10.0,
		Freshness:    papertrading.EvalFreshnessFresh,
		Bars:         tBarsNearCostPullbackBounce(),
	})
	require.Contains(t, res.Notes, papertrading.TSignalReasonCannotSell)
	for _, s := range res.Signals {
		require.NotEqual(t, papertrading.TSignalBuyWatch, s.SignalType)
	}
}

func TestBuildHoldingTSignal_PriceStale(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:    "sh600000",
		HasPosition:  true,
		CanSell:      true,
		AvailableQty: 100,
		CostPrice:    10.0,
		Freshness:    papertrading.EvalFreshnessStale,
		Bars:         tBarsNearCostPullbackBounce(),
	})
	require.Empty(t, res.Signals)
	require.Contains(t, res.Notes, papertrading.TSignalReasonPriceStale)
}

func TestBuildHoldingTSignal_No5mBars(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:    "sh600000",
		HasPosition:  true,
		CanSell:      true,
		AvailableQty: 100,
		CostPrice:    10.0,
		Freshness:    papertrading.EvalFreshnessFresh,
		Bars:         []papertrading.HoldingTSignalBar{{Close: 10, High: 10.1, Low: 9.9}},
	})
	require.Empty(t, res.Signals)
	require.Contains(t, res.Notes, papertrading.TSignalReasonNoBars)
}

func TestBuildHoldingTSignal_EmptyPosition(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{
		StockCode:   "sh600000",
		HasPosition: false,
		CanSell:     false,
		CostPrice:   10.0,
		Freshness:   papertrading.EvalFreshnessFresh,
		Bars:        tBarsNearCostPullbackBounce(),
	})
	require.Empty(t, res.Signals)
	require.Contains(t, res.Notes, papertrading.TSignalReasonNoPosition)
}

func TestBuildHoldingTSignal_NilSafeZero(t *testing.T) {
	res := papertrading.BuildHoldingTSignal(papertrading.HoldingTSignalInput{})
	require.Empty(t, res.Signals)
	require.Contains(t, res.Notes, papertrading.TSignalReasonNoPosition)
}
