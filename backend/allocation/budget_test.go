package allocation

import (
	"testing"

	"go-stock/backend/portfolio"

	"github.com/stretchr/testify/require"
)

func snap(cash, reserved, equity, exposure float64) *portfolio.Snapshot {
	return &portfolio.Snapshot{
		Found:         true,
		AccountID:     1,
		AccountName:   "paper_sim_default",
		Cash:          cash,
		ReservedCash:  reserved,
		AvailableCash: cash,
		TotalEquity:   equity,
		MarketValue:   exposure,
		TotalExposure: exposure,
	}
}

func TestBudget_CashSufficient(t *testing.T) {
	// cash 800k, exposure 200k, equity 1M, 85% cap → gross_headroom 650k → available 650k
	req := CapitalAllocationRequest{
		Snapshot: snap(800_000, 0, 1_000_000, 200_000),
		Policy:   AllocationPolicy{MaxGrossExposurePct: 0.85},
	}
	got := Budget(req)
	require.Equal(t, ReasonOK, got.Reason)
	require.InDelta(t, 650_000, got.AvailableCapital, 1e-6)
	require.InDelta(t, 800_000, got.CashAvailable, 1e-6)
	require.InDelta(t, 850_000, got.ExposureLimit, 1e-6)
	require.Equal(t, ReasonGross, got.Binding)
}

func TestBudget_CashInsufficient(t *testing.T) {
	req := CapitalAllocationRequest{
		Snapshot: snap(0, 0, 1_000_000, 0),
		Policy:   AllocationPolicy{MaxGrossExposurePct: 0.85},
	}
	got := Budget(req)
	require.Equal(t, ReasonCash, got.Reason)
	require.Equal(t, 0.0, got.AvailableCapital)
	require.InDelta(t, 0.0, got.CashAvailable, 1e-9)
	require.InDelta(t, 850_000, got.ExposureLimit, 1e-6)
}

func TestBudget_ExposureLimit(t *testing.T) {
	// cash plenty; exposure already above 85% equity
	req := CapitalAllocationRequest{
		Snapshot: snap(1_000_000, 0, 1_000_000, 900_000),
		Policy:   AllocationPolicy{MaxGrossExposurePct: 0.85},
	}
	got := Budget(req)
	require.Equal(t, ReasonGross, got.Reason)
	require.Equal(t, 0.0, got.AvailableCapital)
	require.InDelta(t, 1_000_000, got.CashAvailable, 1e-6)
	require.InDelta(t, 850_000, got.ExposureLimit, 1e-6)
	require.InDelta(t, -50_000, got.GrossHeadroom, 1e-6)
}

func TestBudget_ReservedCash(t *testing.T) {
	req := CapitalAllocationRequest{
		Snapshot:   snap(100_000, 40_000, 100_000, 0),
		Policy:     AllocationPolicy{MaxGrossExposurePct: 1.0},
		PendingBuy: 10_000,
	}
	got := Budget(req)
	require.Equal(t, ReasonOK, got.Reason)
	require.InDelta(t, 50_000, got.CashAvailable, 1e-6) // 100k-40k-10k
	require.InDelta(t, 50_000, got.AvailableCapital, 1e-6)
	require.InDelta(t, 100_000, got.ExposureLimit, 1e-6)
}

func TestBudget_DoesNotMutateInput(t *testing.T) {
	s := snap(700_000, 5_000, 1_200_000, 500_000)
	s.Positions = []portfolio.Position{{StockCode: "sh600000", Volume: 100, MarketValue: 500_000, Weight: 0.4}}
	req := CapitalAllocationRequest{
		Snapshot:   s,
		Policy:     AllocationPolicy{MaxGrossExposurePct: 0.85},
		PendingBuy: 1_000,
	}
	cash, reserved, eq, exp := s.Cash, s.ReservedCash, s.TotalEquity, s.TotalExposure
	code := s.Positions[0].StockCode
	_ = Budget(req)
	require.InDelta(t, cash, s.Cash, 1e-12)
	require.InDelta(t, reserved, s.ReservedCash, 1e-12)
	require.InDelta(t, eq, s.TotalEquity, 1e-12)
	require.InDelta(t, exp, s.TotalExposure, 1e-12)
	require.Equal(t, code, s.Positions[0].StockCode)
	require.InDelta(t, 1_000, req.PendingBuy, 1e-12)
	require.InDelta(t, 0.85, req.Policy.MaxGrossExposurePct, 1e-12)
}

func TestBudget_NoAccount(t *testing.T) {
	got := Budget(CapitalAllocationRequest{Snapshot: nil})
	require.Equal(t, ReasonNoAccount, got.Reason)
	require.Equal(t, 0.0, got.AvailableCapital)

	empty := &portfolio.Snapshot{Found: false, Cash: 999}
	got = Budget(CapitalAllocationRequest{Snapshot: empty})
	require.Equal(t, ReasonNoAccount, got.Reason)
	require.Equal(t, 0.0, got.AvailableCapital)
	require.InDelta(t, 999, empty.Cash, 1e-12) // no repair
}

func TestBudget_NegativeInputNotRepaired(t *testing.T) {
	s := snap(-100, 0, 1_000_000, 0)
	got := Budget(CapitalAllocationRequest{Snapshot: s, Policy: AllocationPolicy{MaxGrossExposurePct: 0.85}})
	require.Equal(t, ReasonNegativeInput, got.Reason)
	require.Equal(t, 0.0, got.AvailableCapital)
	require.InDelta(t, -100, s.Cash, 1e-12)
}

func TestBudget_DefaultGrossPctWhenUnset(t *testing.T) {
	got := Budget(CapitalAllocationRequest{Snapshot: snap(1_000_000, 0, 1_000_000, 0)})
	require.InDelta(t, DefaultMaxGrossExposurePct, got.PolicyGrossPct, 1e-12)
	require.InDelta(t, 850_000, got.AvailableCapital, 1e-6)
}

func TestAllocatorInterface_MatchesBudget(t *testing.T) {
	req := CapitalAllocationRequest{Snapshot: snap(100_000, 0, 100_000, 0), Policy: AllocationPolicy{MaxGrossExposurePct: 1}}
	require.Equal(t, Budget(req), NewAllocator().Budget(req))
}
