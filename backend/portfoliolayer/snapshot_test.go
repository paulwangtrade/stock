package portfoliolayer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromLedger_MapsF1Fields(t *testing.T) {
	t.Parallel()
	view := FromLedger(testLedger())
	require.True(t, view.Found)
	require.Equal(t, uint(1), view.AccountID)
	require.InDelta(t, 1_000_000.0, view.Equity, 1e-6)
	require.InDelta(t, 800_000.0, view.Cash, 1e-6)
	require.InDelta(t, 200_000.0, view.Exposure, 1e-6)
	require.Len(t, view.Positions, 1)
	require.Contains(t, view.HoldingCodes(), "sz000001")
	require.Equal(t, testLedger().TotalEquity, view.Ledger().TotalEquity)
}

func TestFromLedger_NilIsNotEmptyBook(t *testing.T) {
	t.Parallel()
	view := FromLedger(nil)
	require.False(t, view.Found)
	require.Empty(t, view.HoldingCodes())
}
