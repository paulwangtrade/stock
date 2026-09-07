package api

import (
	"testing"

	"go-stock/backend/tradeplanorigin"

	"github.com/stretchr/testify/require"
)

func TestWireOriginItem_OmitsMissingSentinel(t *testing.T) {
	items := wireOriginItems([]tradeplanorigin.ItemOrigin{{
		StockCode:       "sz301125",
		PlanID:          "42",
		SignalTime:      tradeplanorigin.Missing,
		SignalPrice:     tradeplanorigin.Missing,
		SignalTag:       tradeplanorigin.Missing,
		SourceReason:    tradeplanorigin.Missing,
		SelectionReason: tradeplanorigin.Missing,
		StrategyName:    tradeplanorigin.Missing,
		Score:           tradeplanorigin.Missing,
	}})
	require.Len(t, items, 1)
	item := items[0]
	require.False(t, item.Signal.Present)
	require.False(t, item.Reason.Present)
	require.False(t, item.Strategy.Present)
	require.Empty(t, item.SignalTag)
	require.Empty(t, item.SourceReason)
}

func TestWireOriginItem_PresentWhenResolved(t *testing.T) {
	items := wireOriginItems([]tradeplanorigin.ItemOrigin{{
		StockCode:    "sz301125",
		PlanID:       "42",
		SignalTag:    "突",
		StrategyName: "trend",
	}})
	require.Len(t, items, 1)
	item := items[0]
	require.True(t, item.Signal.Present)
	require.Equal(t, "突", item.SignalTag)
	require.True(t, item.Strategy.Present)
	require.Equal(t, "trend", item.StrategyName)
}
