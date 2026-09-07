package portfoliolayer

import (
	"math"
	"strings"

	"go-stock/backend/selection"
)

const (
	// LegacyFixedAmountPerName mirrors tradingconfig.DefaultFixedAmount.
	// Shadow copies the constant only — it does not call FixedAmountSizer.
	LegacyFixedAmountPerName = 100_000

	LegacyRoleBasket   = "basket"
	LegacyRoleWaitlist = "waitlist"
	LegacyRoleAbsent   = "absent"

	PortfolioRoleSelected = "selected"
	PortfolioRoleWaitlist = "waitlist"
	PortfolioRoleRejected = "rejected"
	PortfolioRoleAbsent   = "absent"
)

// LegacyCandidateSet is the production-shaped view: ranked order, first N in-basket,
// same scalar amount on every name (Filter scan), no skip-held / sector / equal-weight.
type LegacyCandidateSet struct {
	Ranked         []selection.Candidate `json:"ranked"`
	Basket         []selection.Candidate `json:"basket"`
	Waitlist       []selection.Candidate `json:"waitlist"`
	SelectionLimit int                   `json:"selection_limit"`
	AmountPerName  float64               `json:"amount_per_name"`
}

// CandidateDiff is one name whose legacy role and portfolio role disagree.
type CandidateDiff struct {
	StockCode     string `json:"stock_code"`
	Rank          int    `json:"rank"`
	LegacyRole    string `json:"legacy_role"`
	PortfolioRole string `json:"portfolio_role"`
	Reason        string `json:"reason"`
}

// AllocationDiff is one name whose legacy scalar amount and shadow envelope disagree.
type AllocationDiff struct {
	StockCode    string  `json:"stock_code"`
	LegacyAmount float64 `json:"legacy_amount"`
	ShadowAmount float64 `json:"shadow_amount"`
	Delta        float64 `json:"delta"`
	Reason       string  `json:"reason"`
}

// ConstraintHit is a constraint that actually fired in resolve, selection, or allocation.
type ConstraintHit struct {
	Field    string `json:"field"`
	Consumer string `json:"consumer"`
	Reason   string `json:"reason"`
	Source   string `json:"source,omitempty"`
	Count    int    `json:"count"`
}

// ShadowDecisionMetrics is a compact numeric summary of one Observe run.
type ShadowDecisionMetrics struct {
	RankedCount            int     `json:"ranked_count"`
	LegacyBasketCount      int     `json:"legacy_basket_count"`
	LegacyWaitlistCount    int     `json:"legacy_waitlist_count"`
	PortfolioSelectedCount int     `json:"portfolio_selected_count"`
	PortfolioWaitlistCount int     `json:"portfolio_waitlist_count"`
	PortfolioRejectedCount int     `json:"portfolio_rejected_count"`
	CandidateDiffCount     int     `json:"candidate_diff_count"`
	AllocationDiffCount    int     `json:"allocation_diff_count"`
	ConstraintHitCount     int     `json:"constraint_hit_count"`
	LegacyAmountPerName    float64 `json:"legacy_amount_per_name"`
	ShadowUniformAmount    float64 `json:"shadow_uniform_amount"`
	AvailableCapital       float64 `json:"available_capital"`
	QuantityAlwaysZero     bool    `json:"quantity_always_zero"`
}

// PortfolioShadowReport is the F.5 observation artifact. Not a TradePlan, not Filter input.
type PortfolioShadowReport struct {
	Status           string                    `json:"status"`
	Legacy           LegacyCandidateSet        `json:"legacy"`
	Selection        *PortfolioSelectionResult `json:"selection"`
	Allocation       *AllocationResult         `json:"allocation"`
	AllocationShadow *AllocationShadowReport   `json:"allocation_shadow,omitempty"`
	CandidateDiffs   []CandidateDiff           `json:"candidate_diffs"`
	AllocationDiffs  []AllocationDiff          `json:"allocation_diffs"`
	ConstraintTrace  []ConstraintTrace         `json:"constraint_trace"`
	ConstraintHits   []ConstraintHit           `json:"constraint_hits"`
	Metrics          ShadowDecisionMetrics     `json:"metrics"`
}

func legacyAmount(opts ObserveOptions) float64 {
	if opts.LegacyAmountPerName > 0 {
		return opts.LegacyAmountPerName
	}
	return LegacyFixedAmountPerName
}

func legacyLimit(in DecisionInput) int {
	if in.SelectionLimit > 0 {
		return in.SelectionLimit
	}
	return DefaultMaxNewNames
}

func buildLegacySet(in DecisionInput, amount float64) LegacyCandidateSet {
	limit := legacyLimit(in)
	ranked := in.RankedCandidates
	if ranked == nil {
		ranked = []selection.Candidate{}
	}
	out := LegacyCandidateSet{
		Ranked:         ranked,
		Basket:         []selection.Candidate{},
		Waitlist:       []selection.Candidate{},
		SelectionLimit: limit,
		AmountPerName:  amount,
	}
	for i, c := range ranked {
		if i < limit {
			out.Basket = append(out.Basket, c)
			continue
		}
		out.Waitlist = append(out.Waitlist, c)
	}
	return out
}

// BuildShadowReport compares the legacy (ranked + scalar) shape to Portfolio Selection / Allocation.
// Pure memory: no PlanFilter, TradePlan, Sizer, Execution, or materialization.
func BuildShadowReport(in DecisionInput, obs *DecisionObservation, opts ObserveOptions) *PortfolioShadowReport {
	amount := legacyAmount(opts)
	legacy := buildLegacySet(in, amount)
	report := &PortfolioShadowReport{
		Status:          ShadowObserved,
		Legacy:          legacy,
		CandidateDiffs:  []CandidateDiff{},
		AllocationDiffs: []AllocationDiff{},
		ConstraintTrace: []ConstraintTrace{},
		ConstraintHits:  []ConstraintHit{},
	}
	if obs != nil {
		report.Selection = obs.Selection
		report.Allocation = obs.Allocation
		report.ConstraintTrace = append([]ConstraintTrace{}, obs.Constraints.Trace...)
	}

	legacyRole := map[string]string{}
	rankBy := map[string]int{}
	for i, c := range legacy.Ranked {
		k := normCode(c.StockCode)
		if i < legacy.SelectionLimit {
			legacyRole[k] = LegacyRoleBasket
		} else {
			legacyRole[k] = LegacyRoleWaitlist
		}
		rankBy[k] = c.Rank
	}

	portRole := map[string]string{}
	portReason := map[string]string{}
	if obs != nil && obs.Selection != nil {
		for _, p := range obs.Selection.Selected {
			k := normCode(p.Candidate.StockCode)
			portRole[k] = PortfolioRoleSelected
			portReason[k] = p.Reason
			if _, ok := rankBy[k]; !ok {
				rankBy[k] = p.Rank
			}
		}
		for _, p := range obs.Selection.Waitlist {
			k := normCode(p.Candidate.StockCode)
			portRole[k] = PortfolioRoleWaitlist
			portReason[k] = p.Reason
			if _, ok := rankBy[k]; !ok {
				rankBy[k] = p.Rank
			}
		}
		for _, p := range obs.Selection.Rejected {
			k := normCode(p.Candidate.StockCode)
			portRole[k] = PortfolioRoleRejected
			portReason[k] = p.Reason
			if _, ok := rankBy[k]; !ok {
				rankBy[k] = p.Rank
			}
		}
	}

	seen := map[string]struct{}{}
	addCand := func(code string) {
		k := normCode(code)
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		lr := legacyRole[k]
		if lr == "" {
			lr = LegacyRoleAbsent
		}
		pr := portRole[k]
		if pr == "" {
			pr = PortfolioRoleAbsent
		}
		if sameCandidateRole(lr, pr) {
			return
		}
		reason := portReason[k]
		if reason == "" {
			reason = "role_mismatch"
		}
		report.CandidateDiffs = append(report.CandidateDiffs, CandidateDiff{
			StockCode:     code,
			Rank:          rankBy[k],
			LegacyRole:    lr,
			PortfolioRole: pr,
			Reason:        reason,
		})
	}
	for _, c := range legacy.Ranked {
		addCand(c.StockCode)
	}
	if obs != nil && obs.Selection != nil {
		for _, p := range obs.Selection.Selected {
			addCand(p.Candidate.StockCode)
		}
		for _, p := range obs.Selection.Waitlist {
			addCand(p.Candidate.StockCode)
		}
		for _, p := range obs.Selection.Rejected {
			addCand(p.Candidate.StockCode)
		}
	}

	allocBy := map[string]NameAllocation{}
	qtyZero := true
	if obs != nil && obs.Allocation != nil {
		for _, item := range obs.Allocation.Items {
			allocBy[normCode(item.StockCode)] = item
			if item.TargetQuantity != 0 {
				qtyZero = false
			}
		}
		if len(obs.Allocation.Items) == 0 {
			qtyZero = true
		}
	}

	addAlloc := func(code string) {
		k := normCode(code)
		if k == "" {
			return
		}
		item, hasItem := allocBy[k]
		_, inLegacy := legacyRole[k]
		legacyAmt := 0.0
		if inLegacy {
			legacyAmt = amount
		}
		shadowAmt := 0.0
		reason := "fixed_vs_equal_weight"
		if hasItem {
			shadowAmt = item.TargetAmount
			switch item.AllocationReason {
			case AllocReasonBelowMinOrder, AllocReasonNoAccount, AllocReasonCappedSingleWeight, AllocReasonNoAllocationSet:
				reason = item.AllocationReason
			}
		} else if inLegacy {
			reason = "missing_shadow_envelope"
		} else {
			return
		}
		if math.Abs(legacyAmt-shadowAmt) < 1e-6 {
			return
		}
		report.AllocationDiffs = append(report.AllocationDiffs, AllocationDiff{
			StockCode:    code,
			LegacyAmount: legacyAmt,
			ShadowAmount: shadowAmt,
			Delta:        shadowAmt - legacyAmt,
			Reason:       reason,
		})
	}
	seenAlloc := map[string]struct{}{}
	for _, c := range legacy.Ranked {
		k := normCode(c.StockCode)
		if _, ok := seenAlloc[k]; ok {
			continue
		}
		seenAlloc[k] = struct{}{}
		addAlloc(c.StockCode)
	}
	if obs != nil && obs.Allocation != nil {
		for _, item := range obs.Allocation.Items {
			k := normCode(item.StockCode)
			if _, ok := seenAlloc[k]; ok {
				continue
			}
			seenAlloc[k] = struct{}{}
			addAlloc(item.StockCode)
		}
	}

	report.ConstraintHits = buildConstraintHits(obs, report.CandidateDiffs)
	report.Metrics = ShadowDecisionMetrics{
		RankedCount:         len(legacy.Ranked),
		LegacyBasketCount:   len(legacy.Basket),
		LegacyWaitlistCount: len(legacy.Waitlist),
		CandidateDiffCount:  len(report.CandidateDiffs),
		AllocationDiffCount: len(report.AllocationDiffs),
		ConstraintHitCount:  len(report.ConstraintHits),
		LegacyAmountPerName: amount,
		QuantityAlwaysZero:  qtyZero,
	}
	if obs != nil && obs.Selection != nil {
		report.Metrics.PortfolioSelectedCount = len(obs.Selection.Selected)
		report.Metrics.PortfolioWaitlistCount = len(obs.Selection.Waitlist)
		report.Metrics.PortfolioRejectedCount = len(obs.Selection.Rejected)
	}
	if obs != nil && obs.Allocation != nil {
		report.Metrics.ShadowUniformAmount = obs.Allocation.UniformAmount
		report.Metrics.AvailableCapital = obs.Allocation.Budget.AvailableCapital
	}
	report.AllocationShadow = BuildAllocationShadow(report)
	return report
}

func buildConstraintHits(obs *DecisionObservation, diffs []CandidateDiff) []ConstraintHit {
	counts := map[string]*ConstraintHit{}
	bump := func(field, consumer, reason, source string) {
		key := consumer + "|" + reason + "|" + field
		if h, ok := counts[key]; ok {
			h.Count++
			return
		}
		counts[key] = &ConstraintHit{Field: field, Consumer: consumer, Reason: reason, Source: source, Count: 1}
	}
	if obs != nil {
		for _, tr := range obs.Constraints.Trace {
			if tr.Note == "tightest" || tr.Note == "tightened_within_ceiling" || tr.Note == "ceiling" ||
				tr.Note == "stricter_true" || strings.HasPrefix(tr.Note, "forced_") {
				bump(tr.Field, ConsumerPortfolioSelection, tr.Note, tr.Source)
			}
		}
		if obs.Selection != nil {
			for _, p := range obs.Selection.Waitlist {
				switch p.Reason {
				case ReasonNameLimit, ReasonSectorLimit, ReasonNoSnapshot:
					bump(p.Reason, ConsumerPortfolioSelection, p.Reason, "")
				}
			}
			for _, p := range obs.Selection.Rejected {
				if p.Reason == ReasonAlreadyHolding {
					bump(p.Reason, ConsumerPortfolioSelection, p.Reason, "")
				}
			}
		}
		if obs.Allocation != nil {
			for _, item := range obs.Allocation.Items {
				switch item.AllocationReason {
				case AllocReasonBelowMinOrder, AllocReasonCappedSingleWeight, AllocReasonNoAccount:
					bump(item.AllocationReason, ConsumerAllocation, item.AllocationReason, "")
				}
			}
		}
	}
	for _, d := range diffs {
		if d.Reason == ReasonAlreadyHolding || d.Reason == ReasonNameLimit || d.Reason == ReasonSectorLimit {
			bump(d.Reason, ConsumerPortfolioSelection, "candidate_diff", "")
		}
	}
	out := make([]ConstraintHit, 0, len(counts))
	for _, h := range counts {
		out = append(out, *h)
	}
	return out
}

func normCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func sameCandidateRole(legacyRole, portfolioRole string) bool {
	if legacyRole == portfolioRole {
		return true
	}
	if legacyRole == LegacyRoleBasket && portfolioRole == PortfolioRoleSelected {
		return true
	}
	if legacyRole == LegacyRoleWaitlist && portfolioRole == PortfolioRoleWaitlist {
		return true
	}
	return false
}
