package tradingrule_test

import (
	"testing"

	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestBuyFillDeltas(t *testing.T) {
	locked, avail := tradingrule.BuyFillDeltas(100, tradingrule.SellableT0)
	require.Equal(t, int64(0), locked)
	require.Equal(t, int64(100), avail)

	locked, avail = tradingrule.BuyFillDeltas(100, tradingrule.SellableT1)
	require.Equal(t, int64(100), locked)
	require.Equal(t, int64(0), avail)

	locked, avail = tradingrule.BuyFillDeltas(100, "")
	require.Equal(t, int64(100), locked)
	require.Equal(t, int64(0), avail)
}
