package allocationengine

import "math"

// Allocate is the H.3 entry: SelectionResult + PortfolioSnapshot + Budget + ResolvedConstraints
// → AllocationResult (equal-weight v1 + single cap + waitlist keep).
// Pure function: no DB, no PlanFilter, no TradePlan persist, no Execution.
func Allocate(in EngineInput) *AllocationResult {
	budget := AllocationBudget{}
	if in.Budget != nil {
		budget = *in.Budget
	} else {
		budget = ComputeBudget(in.Snapshot, in.Resolved)
	}
	return Run(Input{
		Selection: in.Selection,
		Snapshot:  in.Snapshot,
		Budget:    budget,
		Policy:    in.Resolved,
		Options:   in.Options,
	})
}

// Run splits a concrete Budget across Selection (equal weight + single-name cap + waitlist copy).
func Run(in Input) *AllocationResult {
	method := in.Options.Method
	if method == "" || method != MethodEqualWeight {
		method = MethodEqualWeight
	}
	out := &AllocationResult{
		SchemaVersion: SchemaVersion,
		Items:         []NameAllocation{},
		Allocated:     []NameAllocation{},
		Waitlist:      []NameAllocation{},
		Budget:        in.Budget,
		Method:        method,
	}

	applyMin := true
	if in.Options.ApplyMinOrder != nil {
		applyMin = *in.Options.ApplyMinOrder
	}
	waitMode := in.Options.WaitlistAmountMode
	if waitMode == "" {
		waitMode = WaitlistCopyUniform
	}

	if !usableSnapshot(in.Snapshot) {
		out.Budget.Binding = BindingNoAccount
		for _, code := range in.Selection.Selected {
			item := zeroAlloc(code, true, ReasonNoAccount)
			out.Items = append(out.Items, item)
			out.Allocated = append(out.Allocated, item)
		}
		for _, code := range in.Selection.Waitlist {
			item := zeroAlloc(code, false, ReasonWaitlistUniform)
			out.Items = append(out.Items, item)
			out.Waitlist = append(out.Waitlist, item)
		}
		return out
	}

	n := len(in.Selection.Selected)
	if n == 0 {
		for _, code := range in.Selection.Waitlist {
			item := zeroAlloc(code, false, ReasonNoAllocationSet)
			out.Items = append(out.Items, item)
			out.Waitlist = append(out.Waitlist, item)
		}
		return out
	}

	// Equal-weight v1 over allocation set only (waitlist does not increase N).
	uniform := math.Floor(in.Budget.AvailableCapital / float64(n))
	if uniform < 0 {
		uniform = 0
	}
	reason := ReasonEqualSplit
	// Single position cap: min(uniform, floor(equity × max_single_weight)).
	if in.Policy.MaxSingleWeight > 0 && in.Snapshot.Equity > 0 {
		capAmt := math.Floor(in.Snapshot.Equity * in.Policy.MaxSingleWeight)
		if capAmt < uniform {
			uniform = capAmt
			reason = ReasonCappedSingleWeight
		}
	}
	out.UniformAmount = uniform

	for _, code := range in.Selection.Selected {
		itemReason := reason
		amt := uniform
		if applyMin && in.Policy.MinOrderAmount > 0 && amt > 0 && amt < in.Policy.MinOrderAmount {
			amt = 0
			itemReason = ReasonBelowMinOrder
		}
		item := NameAllocation{
			StockCode:        code,
			TargetAmount:     amt,
			TargetQuantity:   0,
			AllocationReason: itemReason,
			InAllocationSet:  true,
		}
		out.Items = append(out.Items, item)
		out.Allocated = append(out.Allocated, item)
	}

	waitAmt := 0.0
	if waitMode == WaitlistCopyUniform {
		waitAmt = uniform
	}
	for _, code := range in.Selection.Waitlist {
		item := NameAllocation{
			StockCode:        code,
			TargetAmount:     waitAmt,
			TargetQuantity:   0,
			AllocationReason: ReasonWaitlistUniform,
			InAllocationSet:  false,
		}
		out.Items = append(out.Items, item)
		out.Waitlist = append(out.Waitlist, item)
	}
	return out
}

// RunWithComputedBudget computes Budget then Run.
func RunWithComputedBudget(sel SelectionResult, snap *PortfolioSnapshot, resolved ResolvedConstraints, opts Options) *AllocationResult {
	return Allocate(EngineInput{
		Selection: sel,
		Snapshot:  snap,
		Budget:    nil,
		Resolved:  resolved,
		Options:   opts,
	})
}

func usableSnapshot(snap *PortfolioSnapshot) bool {
	return snap != nil && snap.Found
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
