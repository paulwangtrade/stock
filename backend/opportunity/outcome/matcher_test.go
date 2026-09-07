package outcome

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestMatchFIFOLegs_MultiBuyPartialSell(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)

	fills := []papertrading.PaperSimFill{
		{ID: 1, Side: "buy", Volume: 300, FilledAt: t0},
		{ID: 2, Side: "buy", Volume: 200, FilledAt: t1},
		{ID: 3, Side: "sell", Volume: 400, FilledAt: t2},
	}
	legs := matchFIFOLegs(fills)
	require.Len(t, legs, 3)
	require.Equal(t, OutcomeStatusClosed, legs[0].Status)
	require.Equal(t, int64(300), legs[0].Qty)
	require.Equal(t, uint(1), legs[0].BuyFill.ID)
	require.Equal(t, OutcomeStatusClosed, legs[1].Status)
	require.Equal(t, int64(100), legs[1].Qty)
	require.Equal(t, uint(2), legs[1].BuyFill.ID)
	require.Equal(t, OutcomeStatusOpen, legs[2].Status)
	require.Equal(t, int64(100), legs[2].Qty)
}
