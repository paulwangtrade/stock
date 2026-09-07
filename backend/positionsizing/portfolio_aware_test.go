package positionsizing_test

import (
	"os"
	"strings"
	"testing"

	"go-stock/backend/portfolio"
	"go-stock/backend/positionsizing"

	"github.com/stretchr/testify/require"
)

func sampleSnapshot(cash, equity, exposure float64) *portfolio.Snapshot {
	return &portfolio.Snapshot{
		Found:         true,
		Cash:          cash,
		AvailableCash: cash,
		TotalEquity:   equity,
		MarketValue:   exposure,
		TotalExposure: exposure,
	}
}

func TestPortfolioAwareSizer_CaseB_EqualSplitCashBound(t *testing.T) {
	// equity=1e6 cash=400k exposure=0 N=3
	// available = min(400000, 0.85*1e6 - 0) = 400000
	// per = floor(400000/3) = 133333
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(400_000, 1_000_000, 0),
		CandidateCount: 3,
	})
	require.Equal(t, positionsizing.MethodPortfolioAware, got.Method)
	require.Equal(t, float64(133_333), got.PlannedAmount)
	require.Equal(t, int64(0), got.PlannedVolume)
	require.LessOrEqual(t, got.PlannedAmount*3, 400_000.0)
	require.Greater(t, got.PlannedAmount, 0.0)
	require.NotNil(t, got.AmountReason)
	require.Equal(t, float64(133_333), got.AmountReason.FinalAmount)
}

func TestPortfolioAwareSizer_CaseC_NearFullNot100000(t *testing.T) {
	// headroom = 0.85e6 - 830000 = 20000; min(cash=400000, 20000)=20000
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(400_000, 1_000_000, 830_000),
		CandidateCount: 3,
	})
	require.Equal(t, float64(6_666), got.PlannedAmount) // floor(20000/3)
	require.NotEqual(t, float64(100_000), got.PlannedAmount)
	require.Less(t, got.PlannedAmount, float64(100_000))
}

func TestPortfolioAwareSizer_CaseC_SingleWeightCap(t *testing.T) {
	// equity=1e6 cash=1e6 exposure=0 N=1
	// budget = min(1e6, 0.85e6) = 850000; raw=850000; single_cap=200000
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(1_000_000, 1_000_000, 0),
		CandidateCount: 1,
	})
	require.Equal(t, float64(200_000), got.PlannedAmount)
	require.NotNil(t, got.AmountReason)
	require.Equal(t, float64(850_000), got.AmountReason.RawAmount)
	require.Equal(t, float64(200_000), got.AmountReason.FinalAmount)
	require.Contains(t, got.AmountReason.CappedBy, "single_weight")
}

func TestPortfolioAwareSizer_CaseD_ZeroCash(t *testing.T) {
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(0, 1_000_000, 0),
		CandidateCount: 3,
	})
	require.Equal(t, float64(0), got.PlannedAmount)
	require.NotNil(t, got.AmountReason)
	require.Equal(t, float64(0), got.AmountReason.FinalAmount)
}

func TestPortfolioAwareSizer_CaseE_FullBook(t *testing.T) {
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(400_000, 1_000_000, 900_000),
		CandidateCount: 3,
	})
	require.Equal(t, float64(0), got.PlannedAmount)
	require.NotNil(t, got.AmountReason)
	require.Contains(t, got.AmountReason.CappedBy, "gross")
}

func TestPortfolioAwareSizer_MinOrderAmountBlocksTinySlice(t *testing.T) {
	// headroom=2000 / 3 = 666 < 1000
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(400_000, 1_000_000, 848_000),
		CandidateCount: 3,
	})
	require.Equal(t, float64(0), got.PlannedAmount)
	require.Contains(t, got.AmountReason.CappedBy, "min_order")
}

func TestPortfolioAwareSizer_CaseF_AmountReason(t *testing.T) {
	got := (&positionsizing.PortfolioAwareSizer{}).Propose(positionsizing.Request{
		Portfolio:      sampleSnapshot(400_000, 1_000_000, 0),
		CandidateCount: 3,
	})
	r := got.AmountReason
	require.NotNil(t, r)
	require.Equal(t, "portfolio_aware", r.Method)
	require.Equal(t, float64(1_000_000), r.Equity)
	require.Equal(t, float64(400_000), r.Cash)
	require.Equal(t, float64(0), r.CurrentExposure)
	require.Equal(t, float64(400_000), r.Budget)
	require.Equal(t, 3, r.CandidateCount)
	require.Equal(t, float64(133_333), r.RawAmount)
	require.Equal(t, float64(133_333), r.FinalAmount)
	require.Equal(t, got.PlannedAmount, r.FinalAmount)
	require.NotEmpty(t, r.CappedBy)
}

func TestPortfolioAwareSizer_DoesNotQueryDB(t *testing.T) {
	files := []string{"portfolio_aware.go", "types.go", "fixed_amount.go"}
	for _, name := range files {
		raw, err := os.ReadFile(name)
		require.NoError(t, err, name)
		src := string(raw)
		require.NotContains(t, src, "db.Dao")
		require.NotContains(t, src, "\"go-stock/backend/db\"")
		require.NotContains(t, src, "NewService().Snapshot")
	}
}

func TestForMode_SelectsSizerWithoutDefaultSwitch(t *testing.T) {
	fixed := positionsizing.ForMode("")
	_, ok := fixed.(*positionsizing.FixedAmountSizer)
	require.True(t, ok)

	aware := positionsizing.ForMode("portfolio_aware")
	_, ok = aware.(*positionsizing.PortfolioAwareSizer)
	require.True(t, ok)

	p := positionsizing.Default().Propose(positionsizing.Request{})
	require.Equal(t, positionsizing.MethodFixedAmount, p.Method)
}

func TestPositionsizingPackage_NoPaperSimQueries(t *testing.T) {
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(e.Name())
		require.NoError(t, err)
		src := string(raw)
		require.NotContains(t, src, "\"go-stock/backend/db\"", e.Name())
		require.NotContains(t, src, "papertrading.Get", e.Name())
	}
}
