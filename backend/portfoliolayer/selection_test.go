package portfoliolayer

import (
	"testing"

	"go-stock/backend/selection"

	"github.com/stretchr/testify/require"
)

func TestSelectPortfolio_NameLimitKeepsWaitlistOnScanList(t *testing.T) {
	t.Parallel()
	snap := FromLedger(testLedger())
	got := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000002", "sz000003", "sz000004", "sz000005", "sz000006"),
		Snapshot:         snap,
		Constraints:      PortfolioConstraints{MaxNewNames: 3},
	})
	require.Len(t, got.Selected, 3)
	require.Len(t, got.Waitlist, 2)
	require.Empty(t, got.Rejected)
	require.Equal(t, ReasonInAllocationSet, got.Selected[0].Reason)
	require.Equal(t, ReasonNameLimit, got.Waitlist[0].Reason)
	require.False(t, got.Waitlist[0].InAllocationSet)
	scan := got.ScanList()
	require.Len(t, scan, 5)
	require.Equal(t, "sz000002", scan[0].StockCode)
	require.Equal(t, "sz000006", scan[4].StockCode)
	assertNoRiskCodes(t, got)
}

func TestSelectPortfolio_SkipAlreadyHoldingRejectsHold(t *testing.T) {
	t.Parallel()
	snap := FromLedger(testLedger())
	got := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000001", "sz000002"),
		Snapshot:         snap,
		Constraints:      PortfolioConstraints{MaxNewNames: 5, SkipAlreadyHolding: true},
	})
	require.Len(t, got.Rejected, 1)
	require.Equal(t, ReasonAlreadyHolding, got.Rejected[0].Reason)
	require.Equal(t, "sz000001", got.Rejected[0].Candidate.StockCode)
	require.Len(t, got.Selected, 1)
	require.Equal(t, "sz000002", got.Selected[0].Candidate.StockCode)
	for _, c := range got.ScanList() {
		require.NotEqual(t, "sz000001", c.StockCode)
	}
	assertNoRiskCodes(t, got)
}

func TestSelectPortfolio_DefaultDoesNotSkipHoldings(t *testing.T) {
	t.Parallel()
	snap := FromLedger(testLedger())
	got := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000001", "sz000002"),
		Snapshot:         snap,
		Constraints:      PortfolioConstraints{MaxNewNames: 5},
	})
	require.Empty(t, got.Rejected)
	require.Equal(t, "sz000001", got.Selected[0].Candidate.StockCode)
}

func TestSelectPortfolio_NoSnapshotDoesNotFillAllocationSet(t *testing.T) {
	t.Parallel()
	got := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: ranked("sz000002", "sz000003"),
		Snapshot:         FromLedger(nil),
		Constraints:      PortfolioConstraints{MaxNewNames: 5},
	})
	require.Empty(t, got.Selected)
	require.Len(t, got.Waitlist, 2)
	require.Equal(t, ReasonNoSnapshot, got.Waitlist[0].Reason)
	require.Contains(t, got.Reasons, ReasonNoSnapshot)
	assertNoRiskCodes(t, got)
}

func TestSelectPortfolio_SectorNameCap(t *testing.T) {
	t.Parallel()
	cands := []selection.Candidate{
		{StockCode: "sz000010", Industry: "bank", Rank: 1},
		{StockCode: "sz000011", Industry: "bank", Rank: 2},
		{StockCode: "sz000012", Industry: "tech", Rank: 3},
	}
	got := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: cands,
		Snapshot:         FromLedger(testLedger()),
		Constraints:      PortfolioConstraints{MaxNewNames: 5, MaxNamesPerSector: 1},
	})
	require.Len(t, got.Selected, 2)
	require.Equal(t, "sz000010", got.Selected[0].Candidate.StockCode)
	require.Equal(t, "sz000012", got.Selected[1].Candidate.StockCode)
	require.Equal(t, ReasonSectorLimit, got.Waitlist[0].Reason)
	require.Equal(t, "sz000011", got.Waitlist[0].Candidate.StockCode)
}

func assertNoRiskCodes(t *testing.T, got *PortfolioSelectionResult) {
	t.Helper()
	check := func(reason string) {
		require.False(t, usesForbiddenRiskCode(reason), "portfolio layer emitted risk code %s", reason)
	}
	for _, p := range got.Selected {
		check(p.Reason)
	}
	for _, p := range got.Waitlist {
		check(p.Reason)
	}
	for _, p := range got.Rejected {
		check(p.Reason)
	}
	for _, r := range got.Reasons {
		check(r)
	}
}
