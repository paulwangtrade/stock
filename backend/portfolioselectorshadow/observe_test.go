package portfolioselectorshadow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sampleInput() Input {
	return Input{
		TradeDate:      "2026-08-25",
		PoolID:         7,
		LegacyAmount:   100_000,
		LegacyMaxNames: 5,
		Trigger:        "diagnostic",
		PoolItems: []PoolItemView{
			{StockCode: "sz000001", StockName: "A", Industry: "bank", Rank: 1, Score: 90},
			{StockCode: "sz000002", StockName: "B", Industry: "bank", Rank: 2, Score: 80},
			{StockCode: "sz000003", StockName: "C", Industry: "tech", Rank: 3, Score: 70},
			{StockCode: "sz000004", StockName: "D", Industry: "tech", Rank: 4, Score: 60},
			{StockCode: "sz000005", StockName: "E", Industry: "energy", Rank: 5, Score: 50},
			{StockCode: "sz000006", StockName: "F", Industry: "energy", Rank: 6, Score: 40},
		},
		Snapshot: &SnapshotView{
			Found: true, Cash: 1_000_000, Equity: 2_000_000, MarketValue: 1_000_000,
			Positions: []PositionView{
				{StockCode: "sh600000", MarketValue: 200_000, Industry: "bank"},
			},
		},
	}
}

func TestObserve_DisabledSkips(t *testing.T) {
	rt := NewRuntime(Config{Enabled: false})
	rep := rt.Observe(sampleInput())
	require.True(t, rep.Skipped)
	require.Equal(t, SkipReasonDisabled, rep.SkipReason)
	require.False(t, rep.Enabled)
	require.Equal(t, 0, rep.LegacySelectedCount)
	require.Equal(t, 0, rep.PortfolioSelectedCount)
}

func TestObserve_EnabledProducesDiffs(t *testing.T) {
	rt := NewRuntime(Config{Enabled: true})
	rep := rt.Observe(sampleInput())
	require.False(t, rep.Skipped)
	require.True(t, rep.Enabled)
	require.True(t, rep.Comparable)
	require.Equal(t, SchemaVersion, rep.SchemaVersion)

	require.Equal(t, 5, rep.LegacySelectedCount)
	require.Greater(t, rep.PortfolioSelectedCount, 0)
	require.LessOrEqual(t, rep.PortfolioSelectedCount, 5)

	require.Equal(t, "fixed_amount_uniform", rep.AllocationDiff.LegacyMethod)
	require.NotEmpty(t, rep.AllocationDiff.PerName)
	require.NotNil(t, rep.SectorConcentrationDiff.BaselineSectorWeights)
	require.NotNil(t, rep.RejectedReasonSummary.PortfolioRejectHistogram)
	require.Contains(t, rep.RejectedReasonSummary.ConstructionRejectNote, "construction_reject")
	require.NotNil(t, rep.DataGaps)
}

func TestSafeObserve_RecoversPanic(t *testing.T) {
	// SafeObserve uses Observe which recovers; calling with nil runtime still works.
	rep := SafeObserve(nil, Input{})
	require.NotNil(t, rep)
	require.True(t, rep.Skipped || !rep.Enabled)
}

func TestObserve_SectorDiffUsesSnapshot(t *testing.T) {
	rt := NewRuntime(Config{Enabled: true})
	rep := rt.Observe(sampleInput())
	require.False(t, rep.SectorConcentrationDiff.Degraded)
	require.Equal(t, 2_000_000.0, rep.SectorConcentrationDiff.EquityBase)
	require.Contains(t, rep.SectorConcentrationDiff.BaselineSectorWeights, "bank")
}
