package data

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func todayIsWeekday() bool {
	return IsWeekdayLocal(time.Now())
}

func requireDailyBlockReason(t *testing.T, st DailyTradingStatus, expected string) {
	t.Helper()
	if !todayIsWeekday() {
		require.Equal(t, "non_weekday", st.BlockReason)
		require.Contains(t, st.BlockReasons, "non_weekday")
		return
	}
	require.Equal(t, expected, st.BlockReason)
}

func requireDailyNoBlockReason(t *testing.T, st DailyTradingStatus) {
	t.Helper()
	if !todayIsWeekday() {
		require.Equal(t, "non_weekday", st.BlockReason)
		return
	}
	require.Empty(t, st.BlockReason)
	require.Empty(t, st.BlockReasons)
}

func requireDailyBlockReasonsContain(t *testing.T, st DailyTradingStatus, code string) {
	t.Helper()
	require.Contains(t, st.BlockReasons, code)
}

func requireDailyMessageContains(t *testing.T, st DailyTradingStatus, fragment string) {
	t.Helper()
	if !todayIsWeekday() {
		require.Contains(t, st.Message, "non_weekday")
		return
	}
	require.Contains(t, st.Message, fragment)
}
