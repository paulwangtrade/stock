package portfolioselection

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelect_HardMaxNames(t *testing.T) {
	in := Input{
		TradeDate: "2026-08-25",
		PoolID:    1,
		Items: []PoolItem{
			{StockCode: "sz1", Rank: 1, Score: 10, Industry: "a"},
			{StockCode: "sz2", Rank: 2, Score: 9, Industry: "b"},
			{StockCode: "sz3", Rank: 3, Score: 8, Industry: "c"},
			{StockCode: "sz4", Rank: 4, Score: 7, Industry: "d"},
		},
		Policy: DefaultPolicy(),
		Constraints: Constraints{
			AvailableCash: 1_000_000,
			EquityBase:    1_000_000,
		},
		Snapshot: &Snapshot{Found: true, Cash: 1_000_000, Equity: 1_000_000},
	}
	in.Policy.MaxNames = 2
	out := Select(in)
	require.Equal(t, 2, len(out.Selected))
	require.GreaterOrEqual(t, len(out.Rejected), 2)
	require.Equal(t, "over_max_names", out.Rejected[len(out.Rejected)-1].RejectReason)
}

func TestSelect_SkipHolding(t *testing.T) {
	in := Input{
		Items: []PoolItem{
			{StockCode: "sz1", Rank: 1, Score: 10, Industry: "a"},
			{StockCode: "sz2", Rank: 2, Score: 9, Industry: "b"},
		},
		Policy: DefaultPolicy(),
		Constraints: Constraints{
			AvailableCash: 500_000,
			EquityBase:    1_000_000,
		},
		Snapshot: &Snapshot{
			Found: true, Cash: 500_000, Equity: 1_000_000,
			Positions: []Position{{StockCode: "sz1", MarketValue: 100_000, Industry: "a"}},
		},
	}
	in.Policy.HoldingMode = "skip_holding"
	in.Policy.MaxNames = 5
	out := Select(in)
	require.Equal(t, 1, len(out.Selected))
	require.Equal(t, "sz2", out.Selected[0].StockCode)
	require.Equal(t, "already_holding", out.Rejected[0].RejectReason)
}
