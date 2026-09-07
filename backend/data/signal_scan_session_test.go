package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEffectiveSignalTradeDate_TradingDay(t *testing.T) {
	// Friday 2026-09-04 (weekday) Shanghai
	fri := time.Date(2026, 9, 4, 15, 30, 0, 0, shanghaiLoc)
	require.Equal(t, "2026-09-04", EffectiveSignalTradeDate("close", fri))
	require.False(t, SignalTradeDateAdjusted("close", fri))
}

func TestEffectiveSignalTradeDate_WeekendUsesPrevTradingDay(t *testing.T) {
	sat := time.Date(2026, 9, 5, 15, 36, 0, 0, shanghaiLoc)
	require.Equal(t, "2026-09-04", EffectiveSignalTradeDate("close", sat))
	require.True(t, SignalTradeDateAdjusted("close", sat))

	sun := time.Date(2026, 9, 6, 10, 0, 0, 0, shanghaiLoc)
	require.Equal(t, "2026-09-04", EffectiveSignalTradeDate("close", sun))
	require.True(t, SignalTradeDateAdjusted("close", sun))
}

func TestEffectiveSignalTradeDate_MondayTradingDay(t *testing.T) {
	mon := time.Date(2026, 9, 7, 9, 0, 0, 0, shanghaiLoc)
	require.Equal(t, "2026-09-07", EffectiveSignalTradeDate("midday", mon))
	require.False(t, SignalTradeDateAdjusted("midday", mon))
}
