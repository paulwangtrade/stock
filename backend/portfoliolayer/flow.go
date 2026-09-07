package portfoliolayer

import (
	"go-stock/backend/allocationengine"
	"go-stock/backend/selection"
)

// DecisionInput is the F.3 decision-core input. Snapshot is injected; this layer does not load accounts.
type DecisionInput struct {
	RankedCandidates []selection.Candidate
	SelectionLimit   int
	Ledger           *PortfolioSnapshot
	Constraints      ConstraintSet
}

// DecisionObservation is an in-memory decision-core result. It is not a TradePlan.
type DecisionObservation struct {
	Snapshot    *PortfolioSnapshot
	Constraints ResolvedConstraints
	Selection   *PortfolioSelectionResult
	Budget      AllocationBudget
	Allocation  *AllocationResult
	ScanList    []selection.Candidate
}

// DecisionFlow is the F.3 decision-core contract (Select → Allocate).
// Implementations must not persist TradePlan, call PlanFilter, Freeze, or Execution.
type DecisionFlow interface {
	Run(DecisionInput) (*DecisionObservation, error)
}

type skeletonFlow struct{}

// NewSkeletonFlow returns the in-process pure-function flow used by shadow Observe.
func NewSkeletonFlow() DecisionFlow {
	return skeletonFlow{}
}

func (skeletonFlow) Run(in DecisionInput) (*DecisionObservation, error) {
	resolved := in.Constraints.Resolve()
	if in.SelectionLimit > 0 && (in.Constraints.User.MaxNewNames == nil &&
		in.Constraints.Strategy.MaxNewNames == nil &&
		in.Constraints.Portfolio.MaxNewNames == nil) {
		resolved.MaxNewNames = in.SelectionLimit
	}
	sel := SelectPortfolio(PortfolioSelectionInput{
		RankedCandidates: in.RankedCandidates,
		Snapshot:         in.Ledger,
		Constraints:      resolved.AsPortfolioConstraints(),
	})
	// H.3 / H3.1: Portfolio shadow path sizes via allocationengine (not FixedAmountSizer).
	alloc := allocateViaEngine(sel, in.Ledger, resolved)
	return &DecisionObservation{
		Snapshot:    in.Ledger,
		Constraints: resolved,
		Selection:   sel,
		Budget:      alloc.Budget,
		Allocation:  alloc,
		ScanList:    sel.ScanList(),
	}, nil
}

func allocateViaEngine(sel *PortfolioSelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints) *AllocationResult {
	engineSel := allocationengine.Selection{}
	if sel != nil {
		for _, p := range sel.Selected {
			engineSel.Selected = append(engineSel.Selected, p.Candidate.StockCode)
		}
		for _, p := range sel.Waitlist {
			engineSel.Waitlist = append(engineSel.Waitlist, p.Candidate.StockCode)
		}
	}
	eng := allocationengine.RunWithComputedBudget(engineSel, toEngineSnapshot(snap), toEnginePolicy(resolved), allocationengine.Options{})
	return fromEngineResult(eng)
}

// AllocateViaEngine sizes names through allocationengine (H.3).
// Exported for DecisionProvider / Shadow Runtime Portfolio paths.
func AllocateViaEngine(sel *PortfolioSelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints) *AllocationResult {
	return allocateViaEngine(sel, snap, resolved)
}

// AllocateViaEngineWithBudget injects a precomputed budget into the H.3 engine.
func AllocateViaEngineWithBudget(sel *PortfolioSelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints, budget AllocationBudget) *AllocationResult {
	engineSel := allocationengine.Selection{}
	if sel != nil {
		for _, p := range sel.Selected {
			engineSel.Selected = append(engineSel.Selected, p.Candidate.StockCode)
		}
		for _, p := range sel.Waitlist {
			engineSel.Waitlist = append(engineSel.Waitlist, p.Candidate.StockCode)
		}
	}
	engBudget := allocationengine.Budget{
		AvailableCash:    budget.AvailableCash,
		ReserveCash:      budget.ReserveCash,
		RiskBudget:       budget.RiskBudget,
		AvailableCapital: budget.AvailableCapital,
		Binding:          budget.Binding,
		PolicyGrossPct:   budget.PolicyGrossPct,
	}
	eng := allocationengine.Allocate(allocationengine.EngineInput{
		Selection: engineSel,
		Snapshot:  toEngineSnapshot(snap),
		Budget:    &engBudget,
		Resolved:  toEnginePolicy(resolved),
		Options:   allocationengine.Options{},
	})
	return fromEngineResult(eng)
}

func toEngineSnapshot(snap *PortfolioSnapshot) *allocationengine.Snapshot {
	if snap == nil {
		return &allocationengine.Snapshot{Found: false}
	}
	return &allocationengine.Snapshot{
		Found:        snap.Found,
		Equity:       snap.Equity,
		Cash:         snap.Cash,
		ReservedCash: snap.ReservedCash,
		Exposure:     snap.Exposure,
	}
}

func toEnginePolicy(resolved ResolvedConstraints) allocationengine.Policy {
	return allocationengine.Policy{
		MaxGrossExposurePct: resolved.MaxGrossExposurePct,
		MaxSingleWeight:     resolved.MaxSingleWeight,
		ReserveCashRatio:    resolved.ReserveCashRatio,
		MinOrderAmount:      resolved.MinOrderAmount,
		BlockNewEntries:     resolved.RiskBlockNewEntries,
	}
}

func fromEngineResult(eng *allocationengine.Result) *AllocationResult {
	if eng == nil {
		return &AllocationResult{Items: []NameAllocation{}, Method: MethodEqualWeight}
	}
	out := &AllocationResult{
		Items:         make([]NameAllocation, 0, len(eng.Items)),
		Method:        eng.Method,
		UniformAmount: eng.UniformAmount,
		Budget: AllocationBudget{
			AvailableCash:    eng.Budget.AvailableCash,
			ReserveCash:      eng.Budget.ReserveCash,
			RiskBudget:       eng.Budget.RiskBudget,
			AvailableCapital: eng.Budget.AvailableCapital,
			Binding:          eng.Budget.Binding,
			PolicyGrossPct:   eng.Budget.PolicyGrossPct,
		},
	}
	for _, it := range eng.Items {
		out.Items = append(out.Items, NameAllocation{
			StockCode:        it.StockCode,
			TargetAmount:     it.TargetAmount,
			TargetQuantity:   it.TargetQuantity,
			AllocationReason: mapEngineReason(it.AllocationReason),
			InAllocationSet:  it.InAllocationSet,
		})
	}
	return out
}

func mapEngineReason(r string) string {
	switch r {
	case allocationengine.ReasonEqualSplit:
		return AllocReasonEqualSplit
	case allocationengine.ReasonCappedSingleWeight:
		return AllocReasonCappedSingleWeight
	case allocationengine.ReasonBelowMinOrder:
		return AllocReasonBelowMinOrder
	case allocationengine.ReasonWaitlistUniform:
		return AllocReasonWaitlistUniform
	case allocationengine.ReasonNoAllocationSet:
		return AllocReasonNoAllocationSet
	case allocationengine.ReasonNoAccount:
		return AllocReasonNoAccount
	default:
		return r
	}
}
