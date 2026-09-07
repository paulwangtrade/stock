package portfoliolayer

import (
	"math"

	"go-stock/backend/allocationengine"
)

// H3.1 Allocation Shadow types — record-only; no write-chain mutation.

// BudgetDiff compares generate-time budgets. Legacy has none.
type BudgetDiff struct {
	LegacyHasBudget       bool             `json:"legacy_has_budget"`
	Portfolio             AllocationBudget `json:"portfolio"`
	ImpliedLegacyNotional float64          `json:"implied_legacy_notional"`
	CapitalVsImpliedDelta float64          `json:"capital_vs_implied_delta"`
	Binding               string           `json:"binding,omitempty"`
	Note                  string           `json:"note,omitempty"`
}

// UniformDiff compares Legacy fixed scalar vs Portfolio equal-weight uniform.
type UniformDiff struct {
	LegacyScalar     float64 `json:"legacy_scalar"`
	PortfolioUniform float64 `json:"portfolio_uniform"`
	Delta            float64 `json:"delta"`
	PortfolioCapped  bool    `json:"portfolio_capped"`
}

// ReserveDiff compares generate-time cash reserve (Legacy always 0).
type ReserveDiff struct {
	LegacyReserve    float64 `json:"legacy_reserve"`
	PortfolioReserve float64 `json:"portfolio_reserve"`
	Delta            float64 `json:"delta"`
}

// RiskCutDiff records Portfolio pre-cuts that Legacy sizer ignores.
type RiskCutDiff struct {
	PortfolioBinding         string `json:"portfolio_binding"`
	SingleCapApplied         bool   `json:"single_cap_applied"`
	MinOrderZeroCount        int    `json:"min_order_zero_count"`
	GrossHeadroomBinding     bool   `json:"gross_headroom_binding"`
	LegacySizerIgnoresCaps   bool   `json:"legacy_sizer_ignores_caps"`
	BlockedNewEntriesBinding bool   `json:"blocked_new_entries_binding"`
}

// WaitlistAmountDiff compares waitlist envelope amounts (not occupancy).
type WaitlistAmountDiff struct {
	Mode                    string  `json:"mode"`
	LegacyWaitlistAmount    float64 `json:"legacy_waitlist_amount"`
	PortfolioWaitlistAmount float64 `json:"portfolio_waitlist_amount"`
	LegacyWaitlistCount     int     `json:"legacy_waitlist_count"`
	PortfolioWaitlistCount  int     `json:"portfolio_waitlist_count"`
	Note                    string  `json:"note,omitempty"`
}

// AllocSideSummary is one side of the H3.1 allocation compare.
type AllocSideSummary struct {
	OK                  bool              `json:"ok"`
	Method              string            `json:"method"`
	AllocationSetCount  int               `json:"allocation_set_count"`
	WaitlistCount       int               `json:"waitlist_count"`
	PositiveAmountCount int               `json:"positive_amount_count"`
	SumAllocationSet    float64           `json:"sum_allocation_set"`
	SumScanList         float64           `json:"sum_scan_list"`
	UniformOrScalar     float64           `json:"uniform_or_scalar"`
	Budget              *AllocationBudget `json:"budget,omitempty"`
	Binding             string            `json:"binding,omitempty"`
}

// AllocationShadowMetrics are observational counts (not a score).
type AllocationShadowMetrics struct {
	AllocationSetCountDelta int     `json:"allocation_set_count_delta"`
	SumAllocationSetDelta   float64 `json:"sum_allocation_set_delta"`
	UniformDelta            float64 `json:"uniform_delta"`
	ReserveDelta            float64 `json:"reserve_delta"`
	SingleCapHit            bool    `json:"single_cap_hit"`
	MinOrderZeroCount       int     `json:"min_order_zero_count"`
}

// AllocationShadowReport is the H3.1 allocation-axis sidecar on Portfolio Shadow.
// Chain stays fixed_amount; this report never promotes Portfolio.
type AllocationShadowReport struct {
	RecordOnly         bool                    `json:"record_only"`
	NotATradePlan      bool                    `json:"not_a_trade_plan"`
	NotAProviderSwitch bool                    `json:"not_a_provider_switch"`
	ChainProvider      string                  `json:"chain_provider"`
	LegacyAmountSource string                  `json:"legacy_amount_source"`
	Comparable         bool                    `json:"comparable"`
	Legacy             AllocSideSummary        `json:"legacy"`
	Portfolio          AllocSideSummary        `json:"portfolio"`
	BudgetDiff         BudgetDiff              `json:"budget_diff"`
	UniformDiff        UniformDiff             `json:"uniform_diff"`
	ReserveDiff        ReserveDiff             `json:"reserve_diff"`
	RiskCutDiff        RiskCutDiff             `json:"risk_cut_diff"`
	WaitlistAmountDiff WaitlistAmountDiff      `json:"waitlist_amount_diff"`
	Metrics            AllocationShadowMetrics `json:"metrics"`
	Notes              []string                `json:"notes,omitempty"`
}

// BuildAllocationShadow compares Legacy fixed-amount vs Portfolio allocation engine output.
// Pure observation; does not mutate write chain or call PlanFilter / Execution.
func BuildAllocationShadow(report *PortfolioShadowReport) *AllocationShadowReport {
	if report == nil {
		return nil
	}
	out := &AllocationShadowReport{
		RecordOnly:         true,
		NotATradePlan:      true,
		NotAProviderSwitch: true,
		ChainProvider:      "fixed_amount",
		LegacyAmountSource: "constant",
		Comparable:         true,
		Notes:              []string{},
	}

	legacyAmt := report.Legacy.AmountPerName
	if legacyAmt <= 0 {
		legacyAmt = LegacyFixedAmountPerName
	}
	legacyBasketN := len(report.Legacy.Basket)
	legacyWaitN := len(report.Legacy.Waitlist)
	implied := legacyAmt * float64(legacyBasketN)

	legacySumScan := legacyAmt * float64(legacyBasketN+legacyWaitN)
	legacyPositive := 0
	if legacyAmt > 0 {
		legacyPositive = legacyBasketN + legacyWaitN
	}
	out.Legacy = AllocSideSummary{
		OK:                  true,
		Method:              "fixed_amount",
		AllocationSetCount:  legacyBasketN,
		WaitlistCount:       legacyWaitN,
		PositiveAmountCount: legacyPositive,
		SumAllocationSet:    implied,
		SumScanList:         legacySumScan,
		UniformOrScalar:     legacyAmt,
	}

	portUniform := 0.0
	portBinding := ""
	portBudget := AllocationBudget{}
	portHasBudget := false
	selN, waitN := 0, 0
	sumSet, sumScan := 0.0, 0.0
	posCount := 0
	minOrderZeros := 0
	singleCapped := false
	portWaitAmt := 0.0
	method := allocationengine.MethodEqualWeight

	if report.Allocation != nil {
		portHasBudget = true
		portBudget = report.Allocation.Budget
		portBinding = report.Allocation.Budget.Binding
		portUniform = report.Allocation.UniformAmount
		method = report.Allocation.Method
		if method == "" {
			method = allocationengine.MethodEqualWeight
		}
		for _, item := range report.Allocation.Items {
			sumScan += item.TargetAmount
			if item.TargetAmount > 0 {
				posCount++
			}
			if item.InAllocationSet {
				selN++
				sumSet += item.TargetAmount
				if item.AllocationReason == AllocReasonBelowMinOrder || item.AllocationReason == allocationengine.ReasonBelowMinOrder {
					minOrderZeros++
				}
				if item.AllocationReason == AllocReasonCappedSingleWeight || item.AllocationReason == allocationengine.ReasonCappedSingleWeight {
					singleCapped = true
				}
			} else {
				waitN++
				portWaitAmt = item.TargetAmount
			}
		}
		if report.Selection != nil {
			// Prefer selection counts when allocation items omitted waitlist zeros oddly.
			if selN == 0 && len(report.Selection.Selected) > 0 {
				selN = len(report.Selection.Selected)
			}
			if waitN == 0 && len(report.Selection.Waitlist) > 0 && len(report.Allocation.Items) == 0 {
				waitN = len(report.Selection.Waitlist)
			}
		}
	} else {
		out.Comparable = false
		out.Notes = append(out.Notes, "portfolio_allocation_missing")
	}

	var budgetPtr *AllocationBudget
	if portHasBudget {
		b := portBudget
		budgetPtr = &b
	}
	out.Portfolio = AllocSideSummary{
		OK:                  report.Allocation != nil,
		Method:              method,
		AllocationSetCount:  selN,
		WaitlistCount:       waitN,
		PositiveAmountCount: posCount,
		SumAllocationSet:    sumSet,
		SumScanList:         sumScan,
		UniformOrScalar:     portUniform,
		Budget:              budgetPtr,
		Binding:             portBinding,
	}

	out.BudgetDiff = BudgetDiff{
		LegacyHasBudget:       false,
		Portfolio:             portBudget,
		ImpliedLegacyNotional: implied,
		CapitalVsImpliedDelta: portBudget.AvailableCapital - implied,
		Binding:               portBinding,
		Note:                  "legacy_no_generation_budget",
	}
	out.UniformDiff = UniformDiff{
		LegacyScalar:     legacyAmt,
		PortfolioUniform: portUniform,
		Delta:            portUniform - legacyAmt,
		PortfolioCapped:  singleCapped,
	}
	out.ReserveDiff = ReserveDiff{
		LegacyReserve:    0,
		PortfolioReserve: portBudget.ReserveCash,
		Delta:            portBudget.ReserveCash - 0,
	}
	out.RiskCutDiff = RiskCutDiff{
		PortfolioBinding:         portBinding,
		SingleCapApplied:         singleCapped,
		MinOrderZeroCount:        minOrderZeros,
		GrossHeadroomBinding:     portBinding == allocationengine.BindingGross || portBinding == "gross",
		LegacySizerIgnoresCaps:   true,
		BlockedNewEntriesBinding: portBinding == allocationengine.BindingBlocked || portBinding == "blocked",
	}
	out.WaitlistAmountDiff = WaitlistAmountDiff{
		Mode:                    allocationengine.WaitlistCopyUniform,
		LegacyWaitlistAmount:    legacyAmt,
		PortfolioWaitlistAmount: portWaitAmt,
		LegacyWaitlistCount:     legacyWaitN,
		PortfolioWaitlistCount:  waitN,
		Note:                    "waitlist_excluded_from_sum_allocation_set",
	}
	if math.IsNaN(out.BudgetDiff.CapitalVsImpliedDelta) {
		out.BudgetDiff.CapitalVsImpliedDelta = 0
	}

	out.Metrics = AllocationShadowMetrics{
		AllocationSetCountDelta: selN - legacyBasketN,
		SumAllocationSetDelta:   sumSet - implied,
		UniformDelta:            portUniform - legacyAmt,
		ReserveDelta:            portBudget.ReserveCash,
		SingleCapHit:            singleCapped,
		MinOrderZeroCount:       minOrderZeros,
	}
	return out
}
