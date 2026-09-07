package providershadow

import (
	"strings"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfoliolayer"
)

// H3.1 Allocation axis on Provider Shadow Comparator — observation only.

// BudgetView is a persist-safe budget snapshot (Portfolio side).
type BudgetView struct {
	AvailableCash    float64 `json:"available_cash"`
	ReserveCash      float64 `json:"reserve_cash"`
	RiskBudget       float64 `json:"risk_budget"`
	AvailableCapital float64 `json:"available_capital"`
	Binding          string  `json:"binding,omitempty"`
	PolicyGrossPct   float64 `json:"policy_gross_pct,omitempty"`
}

// BudgetDiff: Legacy has no generate-time budget; Portfolio carries AllocationResult.Budget.
type BudgetDiff struct {
	LegacyHasBudget       bool       `json:"legacy_has_budget"`
	Portfolio             BudgetView `json:"portfolio"`
	ImpliedLegacyNotional float64    `json:"implied_legacy_notional"`
	CapitalVsImpliedDelta float64    `json:"capital_vs_implied_delta"`
	Binding               string     `json:"binding,omitempty"`
	Note                  string     `json:"note,omitempty"`
}

// ReserveDiff: Legacy reserve is always 0 at generate time.
type ReserveDiff struct {
	LegacyReserve    float64 `json:"legacy_reserve"`
	PortfolioReserve float64 `json:"portfolio_reserve"`
	Delta            float64 `json:"delta"`
}

// RiskCutDiff explains Portfolio pre-cuts that FixedAmount ignores.
type RiskCutDiff struct {
	PortfolioBinding       string `json:"portfolio_binding"`
	SingleCapApplied       bool   `json:"single_cap_applied"`
	MinOrderZeroCount      int    `json:"min_order_zero_count"`
	GrossHeadroomBinding   bool   `json:"gross_headroom_binding"`
	LegacySizerIgnoresCaps bool   `json:"legacy_sizer_ignores_caps"`
	BlockedNewEntries      bool   `json:"blocked_new_entries_binding"`
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

// UniformDiff compares fixed scalar vs equal-weight uniform.
type UniformDiff struct {
	LegacyScalar     float64 `json:"legacy_scalar"`
	PortfolioUniform float64 `json:"portfolio_uniform"`
	Delta            float64 `json:"delta"`
	PortfolioCapped  bool    `json:"portfolio_capped"`
}

// AllocSideSummary is one side of the allocation compare.
type AllocSideSummary struct {
	OK                  bool        `json:"ok"`
	Method              string      `json:"method"`
	AllocationSetCount  int         `json:"allocation_set_count"`
	WaitlistCount       int         `json:"waitlist_count"`
	PositiveAmountCount int         `json:"positive_amount_count"`
	SumAllocationSet    float64     `json:"sum_allocation_set"`
	SumScanList         float64     `json:"sum_scan_list"`
	UniformOrScalar     float64     `json:"uniform_or_scalar"`
	Budget              *BudgetView `json:"budget,omitempty"`
	Binding             string      `json:"binding,omitempty"`
}

// AllocationCompare is the H3.1 allocation sidecar on ProviderComparisonReport.
// No winner / auto-promote; chain stays fixed_amount.
type AllocationCompare struct {
	RecordOnly         bool                `json:"record_only"`
	NotATradePlan      bool                `json:"not_a_trade_plan"`
	NotAProviderSwitch bool                `json:"not_a_provider_switch"`
	ChainProvider      string              `json:"chain_provider"`
	Comparable         bool                `json:"comparable"`
	Legacy             AllocSideSummary    `json:"legacy"`
	Portfolio          AllocSideSummary    `json:"portfolio"`
	BudgetDiff         BudgetDiff          `json:"budget_diff"`
	AmountDiffs        []AmountDiff        `json:"amount_diffs"`
	ReserveDiff        ReserveDiff         `json:"reserve_diff"`
	RiskCutDiff        RiskCutDiff         `json:"risk_cut_diff"`
	WaitlistAmountDiff WaitlistAmountDiff  `json:"waitlist_amount_diff"`
	UniformDiff        UniformDiff         `json:"uniform_diff"`
	Notes              []string            `json:"notes,omitempty"`
}

// BuildAllocationCompare projects Legacy fixed-amount vs Portfolio AllocationResult envelopes.
// Observation only — does not mutate TradePlan or switch providers.
func BuildAllocationCompare(
	legacy, portfolio *decisionprovider.DecisionEnvelope,
	amountDiffs []AmountDiff,
) *AllocationCompare {
	out := &AllocationCompare{
		RecordOnly:         true,
		NotATradePlan:      true,
		NotAProviderSwitch: true,
		ChainProvider:      decisionprovider.ProviderFixedAmount,
		AmountDiffs:        amountDiffs,
		Notes:              []string{},
	}
	if amountDiffs == nil {
		out.AmountDiffs = []AmountDiff{}
	}

	legacyOK := legacy != nil && legacy.OK
	portOK := portfolio != nil && portfolio.OK
	out.Comparable = legacyOK && portOK
	if !out.Comparable {
		out.Notes = append(out.Notes, "incomparable_skip_full_alloc_axis")
		out.RiskCutDiff.LegacySizerIgnoresCaps = true
		return out
	}

	leg := summarizeLegacy(legacy)
	port := summarizePortfolio(portfolio)
	out.Legacy = leg
	out.Portfolio = port

	portBudget := BudgetView{}
	if port.Budget != nil {
		portBudget = *port.Budget
	}

	out.BudgetDiff = BudgetDiff{
		LegacyHasBudget:       false,
		Portfolio:             portBudget,
		ImpliedLegacyNotional: leg.SumAllocationSet,
		CapitalVsImpliedDelta: portBudget.AvailableCapital - leg.SumAllocationSet,
		Binding:               port.Binding,
		Note:                  "legacy_no_generation_budget",
	}
	out.ReserveDiff = ReserveDiff{
		LegacyReserve:    0,
		PortfolioReserve: portBudget.ReserveCash,
		Delta:            portBudget.ReserveCash,
	}
	singleCap := false
	minZeros := 0
	if portfolio != nil {
		for _, ln := range portfolio.Lines {
			if !ln.Metadata.InAllocationSet {
				continue
			}
			reason := strings.ToLower(strings.TrimSpace(ln.Metadata.AllocationReason))
			if reason == "capped_single_weight" {
				singleCap = true
			}
			if reason == "below_min_order" {
				minZeros++
			}
		}
	}
	portWaitAmt := 0.0
	if port.WaitlistCount > 0 {
		portWaitAmt = port.UniformOrScalar
	}
	out.RiskCutDiff = RiskCutDiff{
		PortfolioBinding:       port.Binding,
		SingleCapApplied:       singleCap,
		MinOrderZeroCount:      minZeros,
		GrossHeadroomBinding:   port.Binding == "gross",
		LegacySizerIgnoresCaps: true,
		BlockedNewEntries:      port.Binding == "blocked",
	}
	out.WaitlistAmountDiff = WaitlistAmountDiff{
		Mode:                    "copy_uniform",
		LegacyWaitlistAmount:    leg.UniformOrScalar,
		PortfolioWaitlistAmount: portWaitAmt,
		LegacyWaitlistCount:     leg.WaitlistCount,
		PortfolioWaitlistCount:  port.WaitlistCount,
		Note:                    "waitlist_excluded_from_sum_allocation_set",
	}
	out.UniformDiff = UniformDiff{
		LegacyScalar:     leg.UniformOrScalar,
		PortfolioUniform: port.UniformOrScalar,
		Delta:            port.UniformOrScalar - leg.UniformOrScalar,
		PortfolioCapped:  singleCap,
	}
	return out
}

func summarizeLegacy(env *decisionprovider.DecisionEnvelope) AllocSideSummary {
	out := AllocSideSummary{OK: env != nil && env.OK, Method: "fixed_amount"}
	if env == nil {
		return out
	}
	scalar := env.Metadata.UniformAmount
	selN, waitN := 0, 0
	sumSet, sumScan := 0.0, 0.0
	pos := 0
	for _, ln := range env.Lines {
		sumScan += ln.TargetAmount
		if ln.TargetAmount > 0 {
			pos++
		}
		if ln.Metadata.InAllocationSet {
			selN++
			sumSet += ln.TargetAmount
			if scalar <= 0 && ln.TargetAmount > 0 {
				scalar = ln.TargetAmount
			}
		} else {
			waitN++
		}
	}
	out.AllocationSetCount = selN
	out.WaitlistCount = waitN
	out.PositiveAmountCount = pos
	out.SumAllocationSet = sumSet
	out.SumScanList = sumScan
	out.UniformOrScalar = scalar
	return out
}

func summarizePortfolio(env *decisionprovider.DecisionEnvelope) AllocSideSummary {
	out := AllocSideSummary{OK: env != nil && env.OK, Method: "equal_weight"}
	if env == nil {
		return out
	}
	b := env.Metadata.Budget
	bv := budgetViewFrom(b)
	out.Budget = &bv
	out.Binding = b.Binding
	out.UniformOrScalar = env.Metadata.UniformAmount

	selN, waitN := 0, 0
	sumSet, sumScan := 0.0, 0.0
	pos := 0
	for _, ln := range env.Lines {
		sumScan += ln.TargetAmount
		if ln.TargetAmount > 0 {
			pos++
		}
		if ln.Metadata.InAllocationSet {
			selN++
			sumSet += ln.TargetAmount
		} else {
			waitN++
		}
	}
	out.AllocationSetCount = selN
	out.WaitlistCount = waitN
	out.PositiveAmountCount = pos
	out.SumAllocationSet = sumSet
	out.SumScanList = sumScan
	return out
}

func budgetViewFrom(b portfoliolayer.AllocationBudget) BudgetView {
	return BudgetView{
		AvailableCash:    b.AvailableCash,
		ReserveCash:      b.ReserveCash,
		RiskBudget:       b.RiskBudget,
		AvailableCapital: b.AvailableCapital,
		Binding:          b.Binding,
		PolicyGrossPct:   b.PolicyGrossPct,
	}
}
