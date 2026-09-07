package decisionprovider

import (
	"strings"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/selection"
)

// PortfolioDecisionProvider runs SelectPortfolio + Allocate into a G.2 envelope.
// It does not run plan-state risk filtering, persist TradePlan, or replace the OFF Legacy path.
type PortfolioDecisionProvider struct{}

// NewPortfolioDecisionProvider returns the un-wired Portfolio provider (G.11).
func NewPortfolioDecisionProvider() *PortfolioDecisionProvider {
	return &PortfolioDecisionProvider{}
}

func (p *PortfolioDecisionProvider) Name() string { return ProviderPortfolioAllocation }

func (p *PortfolioDecisionProvider) Decide(ctx DecisionContext) (*DecisionEnvelope, error) {
	if ctx.Version.Provider != "" && ctx.Version.Provider != p.Name() {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "version.provider mismatch")
	}
	if ctx.Selection == nil {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "selection is required")
	}
	if ctx.DecisionTime.IsZero() {
		return failEnvelope(ctx, p.Name(), ErrCodeContextInvalid, "decision_time is required")
	}

	ranked := ctx.Selection.RankedCandidates
	if ranked == nil {
		ranked = []selection.Candidate{}
	}

	resolved := ctx.Constraints.Resolve()
	if ctx.Selection.SelectionLimit > 0 && constraintMaxNamesUnset(ctx.Constraints) {
		resolved.MaxNewNames = ctx.Selection.SelectionLimit
	}

	sel := portfoliolayer.SelectPortfolio(portfoliolayer.PortfolioSelectionInput{
		RankedCandidates: ranked,
		Snapshot:         ctx.Snapshot,
		Constraints:      resolved.AsPortfolioConstraints(),
	})

	// H.3: Portfolio provider sizes via AllocationEngine (not Legacy fixed-amount path).
	var alloc *portfoliolayer.AllocationResult
	if ctx.Budget != nil {
		alloc = portfoliolayer.AllocateViaEngineWithBudget(sel, ctx.Snapshot, resolved, *ctx.Budget)
	} else {
		alloc = portfoliolayer.AllocateViaEngine(sel, ctx.Snapshot, resolved)
	}

	if err := validateAllocation(sel, alloc); err != nil {
		env := emptyEnvelope(ctx, p.Name())
		env.Error = err
		return env, err
	}

	lines, rejected, ferr := envelopeFromPortfolio(sel, alloc)
	if ferr != nil {
		env := emptyEnvelope(ctx, p.Name())
		env.Error = ferr
		return env, ferr
	}

	if allocationSetAllZero(sel, lines) {
		return failEnvelope(ctx, p.Name(), ErrCodeZeroAmount, "allocation set has no positive target_amount")
	}

	ver := ctx.Version
	if ver.Provider == "" {
		ver.Provider = p.Name()
	}
	if ver.Contract == "" {
		ver.Contract = ContractG21
	}
	if ver.Allocation == "" {
		ver.Allocation = AllocVersionF1Equal
	}

	notes := []string{}
	if ctx.Budget != nil && ctx.Snapshot != nil && ctx.Snapshot.Found &&
		ctx.Budget.AvailableCapital > ctx.Snapshot.Cash+1e-6 {
		notes = append(notes, "budget_inconsistent")
	}

	found := ctx.Snapshot != nil && ctx.Snapshot.Found
	uniform := 0.0
	budget := portfoliolayer.AllocationBudget{}
	if alloc != nil {
		uniform = alloc.UniformAmount
		budget = alloc.Budget
	}

	return &DecisionEnvelope{
		OK:             true,
		Provider:       p.Name(),
		DecisionTime:   ctx.DecisionTime,
		Version:        ver,
		SelectionLimit: ctx.Selection.SelectionLimit,
		Lines:          lines,
		Rejected:       rejected,
		Metadata: EnvelopeMeta{
			UniformAmount: uniform,
			SnapshotFound: found,
			Budget:        budget,
			Notes:         notes,
		},
	}, nil
}

func constraintMaxNamesUnset(c portfoliolayer.ConstraintSet) bool {
	return c.User.MaxNewNames == nil && c.Strategy.MaxNewNames == nil && c.Portfolio.MaxNewNames == nil
}

func validateAllocation(sel *portfoliolayer.PortfolioSelectionResult, alloc *portfoliolayer.AllocationResult) *DecisionError {
	if alloc == nil {
		return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "allocation result is nil"}
	}
	if alloc.Method != "" && alloc.Method != portfoliolayer.MethodEqualWeight {
		return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "allocation method is not equal_weight"}
	}
	scan := []selection.Candidate{}
	if sel != nil {
		scan = sel.ScanList()
	}
	if len(alloc.Items) != len(scan) {
		return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "allocation items do not match scan_list"}
	}
	seen := map[string]struct{}{}
	for i, item := range alloc.Items {
		if item.TargetQuantity != 0 {
			return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "target_quantity must be 0"}
		}
		if item.TargetAmount < 0 || !isFinite(item.TargetAmount) {
			return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "illegal target_amount"}
		}
		code := normSymbol(item.StockCode)
		if code == "" {
			return &DecisionError{Code: ErrCodeMissingSymbol, Message: "allocation item missing symbol"}
		}
		if _, dup := seen[code]; dup {
			return &DecisionError{Code: ErrCodeInvalidAllocation, Message: "duplicate symbol"}
		}
		seen[code] = struct{}{}
		scanCode := ""
		if i < len(scan) {
			scanCode = normSymbol(scan[i].StockCode)
		}
		if scanCode != code {
			return &DecisionError{Code: ErrCodeMissingSymbol, Message: "allocation symbol not on scan_list", Symbol: item.StockCode}
		}
	}
	return nil
}

func envelopeFromPortfolio(sel *portfoliolayer.PortfolioSelectionResult, alloc *portfoliolayer.AllocationResult) ([]DecisionLine, []RejectedLine, *DecisionError) {
	lines := []DecisionLine{}
	rejected := []RejectedLine{}
	if sel == nil {
		return lines, rejected, nil
	}

	pickBy := map[string]portfoliolayer.PortfolioPick{}
	for _, p := range sel.Selected {
		pickBy[normSymbol(p.Candidate.StockCode)] = p
	}
	for _, p := range sel.Waitlist {
		pickBy[normSymbol(p.Candidate.StockCode)] = p
	}

	scan := sel.ScanList()
	for i, c := range scan {
		sym := normSymbol(c.StockCode)
		if sym == "" {
			return nil, nil, &DecisionError{Code: ErrCodeMissingSymbol, Message: "empty symbol on scan_list"}
		}
		item := alloc.Items[i]
		pick := pickBy[sym]
		reason := strings.TrimSpace(pick.Reason)
		if reason == "" {
			reason = strings.TrimSpace(item.AllocationReason)
		}
		lines = append(lines, DecisionLine{
			Symbol:       sym,
			TargetAmount: item.TargetAmount,
			Reason:       reason,
			Metadata: LineMeta{
				Rank:             c.Rank,
				Score:            c.Score,
				StockName:        strings.TrimSpace(c.StockName),
				Industry:         strings.TrimSpace(c.Industry),
				InAllocationSet:  item.InAllocationSet,
				AllocationReason: item.AllocationReason,
				SourceProvider:   ProviderPortfolioAllocation,
				CandidateReason:  strings.TrimSpace(c.Reason),
			},
		})
	}

	rejectSet := map[string]struct{}{}
	for _, r := range sel.Rejected {
		sym := normSymbol(r.Candidate.StockCode)
		if sym == "" {
			return nil, nil, &DecisionError{Code: ErrCodeMissingSymbol, Message: "empty symbol in rejected"}
		}
		if _, ok := rejectSet[sym]; ok {
			continue
		}
		rejectSet[sym] = struct{}{}
		rejected = append(rejected, RejectedLine{Symbol: sym, Reason: r.Reason, Rank: r.Rank})
	}
	for _, line := range lines {
		if _, hit := rejectSet[line.Symbol]; hit {
			return nil, nil, &DecisionError{Code: ErrCodeInvalidAllocation, Message: "rejected symbol in lines", Symbol: line.Symbol}
		}
	}

	selectedSet := map[string]struct{}{}
	for _, p := range sel.Selected {
		selectedSet[normSymbol(p.Candidate.StockCode)] = struct{}{}
	}
	inLines := map[string]bool{}
	for _, line := range lines {
		inLines[line.Symbol] = line.Metadata.InAllocationSet
	}
	for code := range selectedSet {
		if !inLines[code] {
			return nil, nil, &DecisionError{Code: ErrCodeInvalidAllocation, Message: "selected name missing from lines", Symbol: code}
		}
	}
	return lines, rejected, nil
}

func allocationSetAllZero(sel *portfoliolayer.PortfolioSelectionResult, lines []DecisionLine) bool {
	if sel == nil || len(sel.Selected) == 0 {
		return false
	}
	for _, line := range lines {
		if line.Metadata.InAllocationSet && line.TargetAmount > 0 && isFinite(line.TargetAmount) {
			return false
		}
	}
	return true
}

func normSymbol(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
