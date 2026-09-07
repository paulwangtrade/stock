package opportunity_test

import (
	"testing"

	"go-stock/backend/opportunity"

	"github.com/stretchr/testify/require"
)

func TestBuildOpportunityID_Stable(t *testing.T) {
	batch := "snap:42"
	a := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-08-18", "强")
	b := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-08-18", "强")
	require.Equal(t, a, b)
	require.True(t, len(a) > 4)
	require.Equal(t, "opp_", a[:4])

	c := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-08-19", "强")
	require.NotEqual(t, a, c)
}

func TestSecucodeToStockCode(t *testing.T) {
	require.Equal(t, "sh600363", opportunity.SecucodeToStockCode("600363.SH"))
	require.Equal(t, "sz000001", opportunity.SecucodeToStockCode("000001.SZ"))
}

func TestBatchKeyFromSnapshot(t *testing.T) {
	require.Equal(t, "snap:99", opportunity.BatchKeyFromSnapshot(99, "", "", ""))
	require.Equal(t, "2026-08-18|close|default", opportunity.BatchKeyFromSnapshot(0, "2026-08-18", "close", "default"))
}
