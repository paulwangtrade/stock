package controlledmonitor_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"go-stock/backend/allocationengine"
	"go-stock/backend/controlledmonitor"
	"go-stock/backend/providershadow"

	"github.com/stretchr/testify/require"
)

func TestDefaultEnabled_IsFalse(t *testing.T) {
	require.False(t, controlledmonitor.DefaultEnabled)
}

func TestObserve_Disabled_Skipped(t *testing.T) {
	rep := controlledmonitor.Observe(controlledmonitor.Input{
		Enabled:   false,
		TradeDate: "2026-08-22",
		Shadow:    &providershadow.ShadowComparisonRecord{Comparable: true},
	})
	require.True(t, rep.Skipped)
	require.Equal(t, controlledmonitor.OutcomeSkipped, rep.Outcome.Code)
	require.False(t, rep.Success)
	require.False(t, rep.AllocationDiff.Present)
	require.True(t, rep.NotATradePlan)
	require.True(t, rep.NotExecution)
	require.True(t, rep.NotProviderSwitch)
}

func TestObserve_ShadowComparable_FillsDiffs(t *testing.T) {
	legAmt := 10000.0
	portAmt := 8000.0
	delta := portAmt - legAmt
	shadow := &providershadow.ShadowComparisonRecord{
		TradeDate:  "2026-08-22",
		Comparable: true,
		ComparisonSummary: providershadow.ComparisonSummary{
			OnlyLegacyCount:    1,
			OnlyPortfolioCount: 2,
			CommonCount:        3,
			AmountDeltaCount:   1,
		},
		LegacySummary:    providershadow.EnvelopeRef{OK: true, Provider: "fixed_amount", LineCount: 4},
		PortfolioSummary: providershadow.EnvelopeRef{OK: true, Provider: "portfolio_allocation", LineCount: 5},
		ReserveSummary: &providershadow.ReserveSummary{
			LegacyReserve: 0, PortfolioReserve: 1000, Delta: 1000,
		},
		AllocationBudgetSummary: &providershadow.AllocationBudgetSummary{
			ImpliedLegacyNotional: 40000,
			CapitalVsImpliedDelta: -5000,
			Portfolio: providershadow.BudgetView{
				AvailableCapital: 35000,
				ReserveCash:      1000,
				Binding:          "gross_headroom",
			},
		},
		Report: &providershadow.ProviderComparisonReport{
			Comparable:    true,
			ChainProvider: "fixed_amount",
			Legacy:        providershadow.EnvelopeRef{OK: true, LineCount: 4},
			Portfolio:     providershadow.EnvelopeRef{OK: true, LineCount: 5},
			Amounts: []providershadow.AmountDiff{{
				Symbol:    "sz000001",
				Legacy:    providershadow.AmountView{Presence: providershadow.PresenceActual, Value: &legAmt},
				Portfolio: providershadow.AmountView{Presence: providershadow.PresenceActual, Value: &portAmt},
				Delta:     &delta,
				Kind:      providershadow.KindBothActual,
			}},
			Allocation: &providershadow.AllocationCompare{
				Comparable: true,
				Legacy: providershadow.AllocSideSummary{
					OK: true, Method: "fixed", AllocationSetCount: 4, WaitlistCount: 0,
				},
				Portfolio: providershadow.AllocSideSummary{
					OK: true, Method: "equal_weight", AllocationSetCount: 5, WaitlistCount: 1, Binding: "gross_headroom",
				},
				BudgetDiff: providershadow.BudgetDiff{CapitalVsImpliedDelta: -5000, Binding: "gross_headroom"},
			},
		},
	}

	rep := controlledmonitor.Observe(controlledmonitor.Input{
		Enabled:   true,
		AsOf:      time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC),
		TradeDate: "2026-08-22",
		Shadow:    shadow,
		Metadata: controlledmonitor.ProviderMetadata{
			ProviderMode:     "controlled",
			DecisionProvider: controlledmonitor.ProviderPortfolioAllocation,
			DecisionVersion:  "decision_provider.g2-1",
			AllocationVersion: "allocation@f1-v1-equal",
		},
		LegacyFilter:    controlledmonitor.FilterSideFromCounts(4, 1, "pass", map[string]int{"CASH": 1}),
		PortfolioFilter: controlledmonitor.FilterSideFromCounts(3, 2, "partial", map[string]int{"CASH_INSUFFICIENT": 2}),
	})

	require.False(t, rep.Skipped)
	require.True(t, rep.Success)
	require.False(t, rep.Failure)
	require.Equal(t, controlledmonitor.OutcomeOK, rep.Outcome.Code)
	require.Equal(t, controlledmonitor.ProviderPortfolioAllocation, rep.ProviderUsed)

	require.True(t, rep.SymbolCountDiff.Present)
	require.Equal(t, 4, rep.SymbolCountDiff.LegacyCount)
	require.Equal(t, 5, rep.SymbolCountDiff.PortfolioCount)
	require.Equal(t, 1, rep.SymbolCountDiff.Delta)
	require.Equal(t, 1, rep.SymbolCountDiff.OnlyLegacyCount)

	require.True(t, rep.AmountDiff.Present)
	require.Equal(t, 1, rep.AmountDiff.SampleCount)
	require.InDelta(t, -2000, rep.AmountDiff.Delta, 1e-9)

	require.True(t, rep.AllocationDiff.Present)
	require.Equal(t, 1, rep.AllocationDiff.SelectedCountDelta)
	require.Equal(t, "gross_headroom", rep.AllocationDiff.PortfolioBinding)

	require.True(t, rep.FilterRejectDiff.Present)
	require.Equal(t, -1, rep.FilterRejectDiff.AcceptedDelta)
	require.Equal(t, 1, rep.FilterRejectDiff.RejectedDelta)
	require.Equal(t, 2, rep.FilterRejectDiff.PortfolioRejectReasons["CASH_INSUFFICIENT"])

	require.True(t, rep.CashUsageDiff.Present)
	require.InDelta(t, 40000, rep.CashUsageDiff.LegacyBuyNotional, 1e-9)
	require.InDelta(t, 35000, rep.CashUsageDiff.PortfolioBuyNotional, 1e-9)
	require.InDelta(t, 1000, rep.CashUsageDiff.ReserveDelta, 1e-9)

	raw, err := json.Marshal(rep)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"provider_used"`)
	require.NotContains(t, string(raw), "CreatePlanWithItems")
}

func TestObserve_PortfolioFailed(t *testing.T) {
	rep := controlledmonitor.Observe(controlledmonitor.Input{
		Enabled: true,
		Shadow: &providershadow.ShadowComparisonRecord{
			Comparable:         false,
			IncomparableReason: providershadow.IncomparablePortfolioFailed,
			Failure:            providershadow.ShadowFailure{PortfolioFailed: true, ErrorReason: "decide_boom"},
			LegacySummary:      providershadow.EnvelopeRef{OK: true, LineCount: 2},
			PortfolioSummary:   providershadow.EnvelopeRef{OK: false},
		},
	})
	require.True(t, rep.Failure)
	require.False(t, rep.Success)
	require.Equal(t, controlledmonitor.OutcomePortfolioFail, rep.Outcome.Code)
	require.Contains(t, rep.Outcome.Detail, "decide_boom")
}

func TestObserve_AllocationAndFilterOnly(t *testing.T) {
	leg := &controlledmonitor.AllocationSide{
		Present: true, OK: true, Method: "fixed", SelectedCount: 3, SumNotional: 30000,
	}
	port := &controlledmonitor.AllocationSide{
		Present: true, OK: true, Method: "equal_weight", SelectedCount: 2,
		SumNotional: 18000, ReserveCash: 2000, AvailableCapital: 20000, Binding: "reserve",
	}
	rep := controlledmonitor.Observe(controlledmonitor.Input{
		Enabled:             true,
		LegacyAllocation:    leg,
		PortfolioAllocation: port,
		LegacyFilter:        controlledmonitor.FilterSideFromCounts(3, 0, "pass", nil),
		PortfolioFilter:     controlledmonitor.FilterSideFromCounts(2, 1, "partial", map[string]int{"NAME_LIMIT": 1}),
		Metadata: controlledmonitor.ProviderMetadata{
			DecisionProvider: controlledmonitor.ProviderFixedAmount,
			ProviderMode:     "off",
		},
	})
	require.True(t, rep.Success)
	require.Equal(t, controlledmonitor.ProviderFixedAmount, rep.ProviderUsed)
	require.Equal(t, -1, rep.AllocationDiff.SelectedCountDelta)
	require.InDelta(t, -12000, rep.AmountDiff.Delta, 1e-9)
	require.Equal(t, -1, rep.FilterRejectDiff.AcceptedDelta)
	require.True(t, rep.CashUsageDiff.Present)
	require.InDelta(t, 2000, rep.CashUsageDiff.ReserveDelta, 1e-9)
	require.Contains(t, rep.DataGaps, "shadow_missing")
}

func TestAllocationSideFromEngine(t *testing.T) {
	res := &allocationengine.AllocationResult{
		Method: "equal_weight",
		Budget: allocationengine.AllocationBudget{
			AvailableCapital: 50_000,
			ReserveCash:      5_000,
			Binding:          "cash",
		},
		Allocated: []allocationengine.NameAllocation{
			{StockCode: "a", TargetAmount: 25_000, InAllocationSet: true},
			{StockCode: "b", TargetAmount: 25_000, InAllocationSet: true},
		},
		Waitlist: []allocationengine.NameAllocation{{StockCode: "c", TargetAmount: 25_000}},
	}
	side := controlledmonitor.AllocationSideFromEngine(res)
	require.NotNil(t, side)
	require.Equal(t, 2, side.SelectedCount)
	require.Equal(t, 1, side.WaitlistCount)
	require.InDelta(t, 50_000, side.SumNotional, 1e-9)
	require.False(t, math.IsNaN(side.SumNotional))
}

func TestObserve_NoFakeZeroWhenDisabled(t *testing.T) {
	rep := controlledmonitor.Observe(controlledmonitor.Input{Enabled: controlledmonitor.DefaultEnabled})
	require.True(t, rep.Skipped)
	require.False(t, rep.SymbolCountDiff.Present)
	require.False(t, rep.AmountDiff.Present)
	require.False(t, rep.FilterRejectDiff.Present)
	require.False(t, rep.CashUsageDiff.Present)
}
