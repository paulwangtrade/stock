package portfoliosim

import (
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/selection"
)

const defaultLegacySimAmount = 100_000.0

// Simulate runs the independent sandbox:
//
//	Selection → (PortfolioRiskSnapshot → SuggestTighten → ApplyTightenOnly) →
//	PortfolioDecisionProvider / AllocationEngine → PlanFilter (read-only)
//
// It does not create Draft, write trade_plans, call Execution, or materialize.
func Simulate(in Input) *SimulatedPortfolioDecisionResult {
	baseConstraints := in.Constraints
	used := resolveCandidates(in)

	riskSnap, riskTrace, effective := prepareRisk(in, baseConstraints)
	resolved := effective.Resolve()
	if used != nil && used.SelectionLimit > 0 && namesUnset(effective) {
		resolved.MaxNewNames = used.SelectionLimit
	}

	ranked := []selection.Candidate{}
	if used != nil {
		ranked = used.RankedCandidates
	}

	// Selection (portfolio layer) — same world as Provider.
	sel := portfoliolayer.SelectPortfolio(portfoliolayer.PortfolioSelectionInput{
		RankedCandidates: ranked,
		Snapshot:         in.Snapshot,
		Constraints:      resolved.AsPortfolioConstraints(),
	})

	// AllocationEngine (H.3) via portfoliolayer bridge.
	var alloc *portfoliolayer.AllocationResult
	if in.Budget != nil {
		alloc = portfoliolayer.AllocateViaEngineWithBudget(sel, in.Snapshot, resolved, *in.Budget)
	} else {
		alloc = portfoliolayer.AllocateViaEngine(sel, in.Snapshot, resolved)
	}

	notes := []string{}
	if in.Budget != nil && in.Snapshot != nil && in.Snapshot.Found &&
		in.Budget.AvailableCapital > in.Snapshot.Cash+1e-6 {
		notes = append(notes, "budget_inconsistent")
	}

	decisionTime := in.DecisionTime
	if decisionTime.IsZero() {
		if in.Snapshot != nil && !in.Snapshot.AsOf.IsZero() {
			decisionTime = in.Snapshot.AsOf
		} else {
			decisionTime = time.Unix(0, 0).UTC()
		}
	}

	// PortfolioDecisionProvider envelope (observation; not a TradePlan).
	port := decisionprovider.NewPortfolioDecisionProvider()
	ctx := decisionprovider.DecisionContext{
		Selection:     used,
		DecisionTime:  decisionTime,
		Snapshot:      in.Snapshot,
		Constraints:   effective,
		Budget:        in.Budget,
		UniformAmount: 0,
		Version: decisionprovider.DecisionVersion{
			Contract:   decisionprovider.ContractG21,
			Provider:   decisionprovider.ProviderPortfolioAllocation,
			Allocation: decisionprovider.AllocVersionF1Equal,
		},
	}
	env, _ := port.Decide(ctx)

	filterOut := runFilterCompat(in.Snapshot, resolved, sel, alloc, in.Filter)

	out := &SimulatedPortfolioDecisionResult{
		RecordOnly:            true,
		NotATradePlan:         true,
		ResolvedConstraints:   resolved,
		Selection:             used,
		Selected:              []portfoliolayer.PortfolioPick{},
		Waitlist:              []portfoliolayer.PortfolioPick{},
		Rejected:              []portfoliolayer.PortfolioReject{},
		ScanList:              []selection.Candidate{},
		Allocation:            alloc,
		PortfolioRiskSnapshot: riskSnap,
		RiskConstraintTrace:   riskTrace,
		DecisionEnvelope:      env,
		PotentialFilter:       filterOut,
		Notes:                 notes,
		LegacyCompare:         buildLegacyCompare(sel, in.LegacyAmountPerName),
	}
	if in.Snapshot != nil {
		out.SnapshotAsOf = in.Snapshot.AsOf
	}
	if sel != nil {
		out.Selected = sel.Selected
		out.Waitlist = sel.Waitlist
		out.Rejected = sel.Rejected
		out.ScanList = sel.ScanList()
	}
	if alloc != nil {
		out.Budget = alloc.Budget
	}

	out.Evaluation = buildMetrics(used, sel, alloc, filterOut)

	if in.AttachShadow {
		obs := portfoliolayer.Observe(portfoliolayer.DecisionInput{
			RankedCandidates: ranked,
			SelectionLimit:   selectionLimitOf(used, resolved),
			Ledger:           in.Snapshot,
			Constraints:      effective,
		}, portfoliolayer.ObserveOptions{
			Enabled:             true,
			LegacyAmountPerName: in.LegacyAmountPerName,
		})
		if obs != nil {
			out.ShadowAnnex = &ShadowAnnex{Report: obs.Report, Evaluation: obs.Evaluation}
		}
	}
	return out
}

func prepareRisk(in Input, base portfoliolayer.ConstraintSet) (
	*portfoliorisk.PortfolioRiskSnapshot,
	*RiskConstraintTrace,
	portfoliolayer.ConstraintSet,
) {
	effective := base
	trace := &RiskConstraintTrace{
		RecordOnly:        true,
		NotRiskViewWrite:  true,
		NotExecutionWrite: true,
		Notes:             []portfoliorisk.TightenNote{},
	}

	ledger := (*portfolio.Snapshot)(nil)
	if in.Snapshot != nil {
		ledger = in.Snapshot.Ledger()
	}
	consCopy := base
	riskSnap := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:    ledger,
		Risk:        in.Risk,
		Constraints: &consCopy,
	})
	if riskSnap != nil {
		trace.InputsFingerprint = riskSnap.InputsFingerprint
	}

	if in.SkipRiskTighten {
		return riskSnap, trace, effective
	}

	sug := portfoliorisk.SuggestTighten(riskSnap, base)
	trace.Notes = sug.Notes
	if trace.Notes == nil {
		trace.Notes = []portfoliorisk.TightenNote{}
	}
	trace.HasPatches = sug.HasPatches
	trace.Applied = sug.HasPatches
	effective = portfoliorisk.ApplyTightenOnly(base, sug.ConstraintSet)
	// Never widen Risk / touch Execution (ApplyTightenOnly already preserves; re-pin).
	effective.Risk = base.Risk
	effective.Execution = base.Execution
	return riskSnap, trace, effective
}

func buildLegacyCompare(sel *portfoliolayer.PortfolioSelectionResult, amount float64) *LegacyCompareSummary {
	if amount <= 0 {
		amount = defaultLegacySimAmount
	}
	n := 0
	if sel != nil {
		n = len(sel.Selected)
	}
	return &LegacyCompareSummary{
		Method:           "fixed_amount",
		AmountPerName:    amount,
		SelectedCount:    n,
		SumAllocationSet: amount * float64(n),
	}
}

func resolveCandidates(in Input) *selection.CandidateSelectionResult {
	if in.Candidates != nil {
		return in.Candidates
	}
	if len(in.RankedFallback) == 0 {
		return &selection.CandidateSelectionResult{
			RankedCandidates:   []selection.Candidate{},
			CandidateDecisions: []selection.CandidateDecision{},
		}
	}
	return selection.Select(in.RankedFallback, in.SelectionCtx)
}

func namesUnset(c portfoliolayer.ConstraintSet) bool {
	return c.User.MaxNewNames == nil && c.Strategy.MaxNewNames == nil && c.Portfolio.MaxNewNames == nil
}

func selectionLimitOf(used *selection.CandidateSelectionResult, resolved portfoliolayer.ResolvedConstraints) int {
	if used != nil && used.SelectionLimit > 0 {
		return used.SelectionLimit
	}
	return resolved.MaxNewNames
}

func buildMetrics(
	used *selection.CandidateSelectionResult,
	sel *portfoliolayer.PortfolioSelectionResult,
	alloc *portfoliolayer.AllocationResult,
	filter SimulatedFilterResult,
) SimulatedDecisionMetrics {
	m := SimulatedDecisionMetrics{QuantityAlwaysZero: true}
	if used != nil {
		m.RankedCount = len(used.RankedCandidates)
	}
	if sel != nil {
		m.SelectedCount = len(sel.Selected)
		m.WaitlistCount = len(sel.Waitlist)
		m.RejectedCount = len(sel.Rejected)
	}
	if alloc != nil {
		m.ReserveCash = alloc.Budget.ReserveCash
		m.AvailableCapital = alloc.Budget.AvailableCapital
		for _, item := range alloc.Items {
			if item.InAllocationSet {
				m.AllocationSetNotional += item.TargetAmount
			}
			if item.TargetQuantity != 0 {
				m.QuantityAlwaysZero = false
			}
		}
		if len(alloc.Items) == 0 {
			m.QuantityAlwaysZero = true
		}
	}
	m.PotentialPendingCount = len(filter.Pending)
	m.PotentialSkippedCount = len(filter.Skipped)
	return m
}
