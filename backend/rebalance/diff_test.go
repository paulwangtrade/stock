package rebalance

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/portfolio"

	"github.com/stretchr/testify/require"
)

func TestDiff_IdenticalBook(t *testing.T) {
	cur := &CurrentView{
		AsOf: time.Now(), Equity: 1_000_000, PriceBasis: PriceBasisSnapshotMark,
		Positions: []CurrentPosition{
			{Symbol: "sz000001", Volume: 1000, AvailableVolume: 1000, MarketValue: 100000, Weight: 0.10},
			{Symbol: "sz000002", Volume: 1000, AvailableVolume: 1000, MarketValue: 100000, Weight: 0.10},
		},
	}
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{Identity: true})
	v := Diff(cur, tgt, DefaultPolicy())
	require.Equal(t, 2, v.Counts.Keep)
	require.Equal(t, 0, v.Counts.Add)
	require.Equal(t, 0, v.Counts.Remove)
	for _, it := range v.Items {
		require.Equal(t, ActionKeep, it.Action)
		require.Equal(t, intentNone, it.IntentHint)
	}
	assertNoTradeAction(t, v)
}

func TestDiff_NewStockAdd(t *testing.T) {
	cur := &CurrentView{
		Equity: 1_000_000,
		Positions: []CurrentPosition{
			{Symbol: "sz000001", Volume: 1000, AvailableVolume: 1000, MarketValue: 200000, Weight: 0.20},
		},
	}
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{EnterSymbols: []string{"sz000002"}})
	v := Diff(cur, tgt, DefaultPolicy())
	require.Equal(t, 1, v.Counts.Add)
	require.GreaterOrEqual(t, v.Counts.Keep+v.Counts.Increase+v.Counts.Decrease, 1)
	var add *Item
	for i := range v.Items {
		if v.Items[i].Action == ActionAdd {
			add = &v.Items[i]
		}
	}
	require.NotNil(t, add)
	require.Equal(t, "sz000002", add.Symbol)
	require.Equal(t, ReasonNewTarget, add.Reason)
	assertNoTradeAction(t, v)
}

func TestDiff_RemoveStock(t *testing.T) {
	cur := &CurrentView{
		Equity: 1_000_000,
		Positions: []CurrentPosition{
			{Symbol: "sz000001", Volume: 1000, AvailableVolume: 1000, MarketValue: 100000, Weight: 0.10},
			{Symbol: "sz000002", Volume: 1000, AvailableVolume: 1000, MarketValue: 100000, Weight: 0.10},
		},
	}
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{DropSymbols: []string{"sz000002"}})
	v := Diff(cur, tgt, DefaultPolicy())
	require.Equal(t, 1, v.Counts.Remove)
	var rem *Item
	for i := range v.Items {
		if v.Items[i].Action == ActionRemove {
			rem = &v.Items[i]
		}
	}
	require.NotNil(t, rem)
	require.Equal(t, "sz000002", rem.Symbol)
	require.Equal(t, int64(1000), rem.ExecutableQty)
	assertNoTradeAction(t, v)
}

func TestDiff_WeightChange(t *testing.T) {
	cur := &CurrentView{
		Equity: 1_000_000,
		Positions: []CurrentPosition{
			{Symbol: "sz000001", Volume: 1000, AvailableVolume: 1000, MarketValue: 300000, Weight: 0.30},
			{Symbol: "sz000002", Volume: 1000, AvailableVolume: 1000, MarketValue: 100000, Weight: 0.10},
		},
	}
	// Equal weight under 0.85 budget → each 0.425 — large deltas → INCREASE/DECREASE
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{})
	v := Diff(cur, tgt, DiffPolicy{WeightBand: 0.02})
	require.Equal(t, 0, v.Counts.Add)
	require.Equal(t, 0, v.Counts.Remove)
	require.Greater(t, v.Counts.Increase+v.Counts.Decrease, 0)
	assertNoTradeAction(t, v)
}

func TestDiff_T1UnsellableAndLocked(t *testing.T) {
	cur := &CurrentView{
		Equity: 1_000_000,
		Positions: []CurrentPosition{
			{Symbol: "sz000001", Volume: 1000, AvailableVolume: 0, LockedVolume: 1000, MarketValue: 100000, Weight: 0.10,
				DecisionState: holdingdecision.StateHoldNormal},
		},
	}
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{
		DropSymbols:  []string{"sz000001"},
		EnterSymbols: []string{"sz000002"},
	})
	v := Diff(cur, tgt, DefaultPolicy())
	require.Equal(t, 1, v.Counts.Remove)
	require.Equal(t, 1, v.Counts.Add)
	var rem *Item
	for i := range v.Items {
		if v.Items[i].Action == ActionRemove {
			rem = &v.Items[i]
		}
	}
	require.NotNil(t, rem)
	require.Equal(t, int64(0), rem.ExecutableQty)
	require.Contains(t, rem.ConstraintFlags, ConstraintT1Locked)
	require.Contains(t, rem.ConstraintFlags, ConstraintInsufficientAvail)
	require.False(t, rem.SwitchOK)
	require.NotEmpty(t, rem.SwitchGroupID)
	require.Equal(t, ReasonHoldNormalBlocks, rem.SwitchReason)
	require.Equal(t, 1, v.BlockedSwitchCount)
	assertNoTradeAction(t, v)
}

func TestCurrentFromSnapshot_AndIdentity(t *testing.T) {
	snap := &portfolio.Snapshot{
		AsOf: time.Now(), Found: true, AccountID: 7, TotalEquity: 500000, Cash: 400000, TotalExposure: 100000,
		Positions: []portfolio.Position{
			{StockCode: "sz000001", Volume: 500, AvailableVolume: 500, MarketValue: 100000, Weight: 0.2},
		},
	}
	cur := CurrentFromSnapshot(snap)
	require.Equal(t, uint(7), cur.AccountID)
	require.Len(t, cur.Positions, 1)
	AttachDecisions(cur, map[string]string{"sz000001": holdingdecision.StateHoldWatch})
	require.Equal(t, holdingdecision.StateHoldWatch, cur.Positions[0].DecisionState)
	tgt := BuildObservationTarget(cur, ObservationTargetOptions{Identity: true})
	v := Diff(cur, tgt, DefaultPolicy())
	require.Equal(t, 1, v.Counts.Keep)
}

func assertNoTradeAction(t *testing.T, v *View) {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
	require.NotContains(t, up, "EXIT_NOW")
	require.NotContains(t, up, "FORCE_CLOSE")
	for _, it := range v.Items {
		require.Equal(t, intentNone, it.IntentHint)
		require.NotEqual(t, "BUY", it.Action)
		require.NotEqual(t, "SELL", it.Action)
	}
}
