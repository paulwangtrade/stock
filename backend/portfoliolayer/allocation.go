package portfoliolayer

import (
	"math"
	"strings"

	"go-stock/backend/allocation"
)

const (
	MethodEqualWeight = "equal_weight"

	AllocReasonEqualSplit         = "equal_split"
	AllocReasonCappedSingleWeight = "capped_single_weight"
	AllocReasonBelowMinOrder      = "below_min_order"
	AllocReasonWaitlistUniform    = "waitlist_uniform"
	AllocReasonNoAllocationSet    = "no_allocation_set"
	AllocReasonNoAccount          = "no_account"
)

// AllocationBudget is the F.1 round buy budget (not per-name sizes).
type AllocationBudget struct {
	AvailableCash    float64
	ReserveCash      float64
	RiskBudget       float64
	AvailableCapital float64
	Binding          string
	PolicyGrossPct   float64
}

// NameAllocation is one name envelope. v1 target_quantity is always 0.
type NameAllocation struct {
	StockCode        string
	TargetAmount     float64
	TargetQuantity   int64
	AllocationReason string
	InAllocationSet  bool
}

// AllocationResult is the F.1 "how much money" output. It does not pick names or lots.
type AllocationResult struct {
	Items         []NameAllocation
	Budget        AllocationBudget
	Method        string
	UniformAmount float64
}

// ComputeBudget maps allocation.Budget onto the F.1 view. Does not mutate Snapshot.
func ComputeBudget(snap *PortfolioSnapshot, resolved ResolvedConstraints) AllocationBudget {
	req := allocation.CapitalAllocationRequest{
		Policy: allocation.AllocationPolicy{
			MaxGrossExposurePct: resolved.MaxGrossExposurePct,
			MaxSingleNamePct:    resolved.MaxSingleWeight,
			MaxNames:            resolved.MaxNewNames,
		},
	}
	if snap != nil {
		ledger := *snap.Ledger()
		if resolved.ReserveCashRatio > 0 {
			want := ledger.Cash * resolved.ReserveCashRatio
			if want > ledger.ReservedCash {
				ledger.ReservedCash = want
			}
		}
		req.Snapshot = &ledger
	}

	raw := allocation.Budget(req)
	reserve := 0.0
	if req.Snapshot != nil {
		reserve = req.Snapshot.ReservedCash
	}
	return AllocationBudget{
		AvailableCash:    raw.CashAvailable,
		ReserveCash:      reserve,
		RiskBudget:       raw.GrossHeadroom,
		AvailableCapital: raw.AvailableCapital,
		Binding:          normalizeBinding(raw.Binding),
		PolicyGrossPct:   raw.PolicyGrossPct,
	}
}

// Allocate splits ComputeBudget(snapshot) equally across the allocation set.
func Allocate(sel *PortfolioSelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints) *AllocationResult {
	return AllocateWithBudget(sel, snap, resolved, ComputeBudget(snap, resolved))
}

// AllocateWithBudget splits an injected (or precomputed) budget. Does not call ComputeBudget.
// v1: waitlist copies uniform_amount; target_quantity is always 0.
func AllocateWithBudget(sel *PortfolioSelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints, budget AllocationBudget) *AllocationResult {
	out := &AllocationResult{
		Items:  []NameAllocation{},
		Budget: budget,
		Method: MethodEqualWeight,
	}
	if !isUsableSnapshot(snap) {
		out.Budget.Binding = "no_account"
		if sel != nil {
			for _, p := range sel.Selected {
				out.Items = append(out.Items, zeroAlloc(p.Candidate.StockCode, true, AllocReasonNoAccount))
			}
			for _, p := range sel.Waitlist {
				out.Items = append(out.Items, zeroAlloc(p.Candidate.StockCode, false, AllocReasonWaitlistUniform))
			}
		}
		return out
	}

	n := 0
	if sel != nil {
		n = len(sel.Selected)
	}
	if n == 0 {
		if sel != nil {
			for _, p := range sel.Waitlist {
				out.Items = append(out.Items, zeroAlloc(p.Candidate.StockCode, false, AllocReasonNoAllocationSet))
			}
		}
		return out
	}

	uniform := math.Floor(budget.AvailableCapital / float64(n))
	if uniform < 0 {
		uniform = 0
	}
	reason := AllocReasonEqualSplit
	if resolved.MaxSingleWeight > 0 && snap.Equity > 0 {
		capAmt := math.Floor(snap.Equity * resolved.MaxSingleWeight)
		if capAmt < uniform {
			uniform = capAmt
			reason = AllocReasonCappedSingleWeight
		}
	}
	out.UniformAmount = uniform

	for _, p := range sel.Selected {
		itemReason := reason
		amt := uniform
		if resolved.MinOrderAmount > 0 && amt > 0 && amt < resolved.MinOrderAmount {
			amt = 0
			itemReason = AllocReasonBelowMinOrder
		}
		out.Items = append(out.Items, NameAllocation{
			StockCode:        p.Candidate.StockCode,
			TargetAmount:     amt,
			TargetQuantity:   0,
			AllocationReason: itemReason,
			InAllocationSet:  true,
		})
	}
	for _, p := range sel.Waitlist {
		out.Items = append(out.Items, NameAllocation{
			StockCode:        p.Candidate.StockCode,
			TargetAmount:     uniform,
			TargetQuantity:   0,
			AllocationReason: AllocReasonWaitlistUniform,
			InAllocationSet:  false,
		})
	}
	return out
}

func zeroAlloc(code string, inSet bool, reason string) NameAllocation {
	return NameAllocation{
		StockCode:        code,
		TargetAmount:     0,
		TargetQuantity:   0,
		AllocationReason: reason,
		InAllocationSet:  inSet,
	}
}

func normalizeBinding(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case allocation.ReasonCash:
		return "cash"
	case allocation.ReasonGross:
		return "gross"
	case allocation.ReasonNoAccount:
		return "no_account"
	case allocation.ReasonBlocked:
		return "blocked"
	case allocation.ReasonNegativeInput:
		return "negative_input"
	case allocation.ReasonOK:
		return "ok"
	default:
		return strings.ToLower(raw)
	}
}
