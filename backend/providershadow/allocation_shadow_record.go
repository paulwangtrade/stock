package providershadow

import (
	"go-stock/backend/decisionprovider"
)

// Schema / version stamps for H3.1 Allocation Shadow Runtime.
const (
	SchemaVersionAllocShadow = "allocation_shadow.h3-1"
	ComparatorVersionH31     = "comparator@g5-1+h3-1"
)

// AllocationBudgetSummary is the Portfolio-side budget projection on the record.
type AllocationBudgetSummary struct {
	LegacyHasBudget    bool       `json:"legacy_has_budget"`
	Portfolio          BudgetView `json:"portfolio"`
	ImpliedLegacyNotional float64 `json:"implied_legacy_notional"`
	CapitalVsImpliedDelta float64 `json:"capital_vs_implied_delta"`
	Binding            string     `json:"binding,omitempty"`
	Note               string     `json:"note,omitempty"`
}

// ReserveSummary is Legacy(0) vs Portfolio reserve.
type ReserveSummary struct {
	LegacyReserve    float64 `json:"legacy_reserve"`
	PortfolioReserve float64 `json:"portfolio_reserve"`
	Delta            float64 `json:"delta"`
}

// RiskCutSummary captures Portfolio pre-cuts ignored by Legacy fixed_amount.
type RiskCutSummary struct {
	PortfolioBinding       string `json:"portfolio_binding"`
	SingleCapApplied       bool   `json:"single_cap_applied"`
	MinOrderZeroCount      int    `json:"min_order_zero_count"`
	GrossHeadroomBinding   bool   `json:"gross_headroom_binding"`
	LegacySizerIgnoresCaps bool   `json:"legacy_sizer_ignores_caps"`
	BlockedNewEntries      bool   `json:"blocked_new_entries_binding"`
}

// enrichAllocationShadowRecord fills H3.1 record fields from the comparison report.
// Does not touch TradePlan / write-chain.
func enrichAllocationShadowRecord(rec *ShadowComparisonRecord, report *ProviderComparisonReport, risk *RiskAdjustmentTrace) {
	if rec == nil {
		return
	}
	if risk != nil {
		rec.RiskConstraintTrace = risk
		rec.RiskAdjustment = risk
	}
	if report == nil {
		return
	}
	if report.RiskAdjustment != nil && rec.RiskConstraintTrace == nil {
		rec.RiskConstraintTrace = report.RiskAdjustment
		rec.RiskAdjustment = report.RiskAdjustment
	}
	alloc := report.Allocation
	if alloc == nil || !alloc.Comparable {
		// Still publish budget/reserve/risk_cut when AllocationCompare was built (even partial).
		if alloc != nil {
			rec.AllocationBudgetSummary = budgetSummaryFrom(alloc.BudgetDiff)
			rec.ReserveSummary = reserveSummaryFrom(alloc.ReserveDiff)
			rec.RiskCutSummary = riskCutSummaryFrom(alloc.RiskCutDiff)
		}
		return
	}
	rec.AllocationBudgetSummary = budgetSummaryFrom(alloc.BudgetDiff)
	rec.ReserveSummary = reserveSummaryFrom(alloc.ReserveDiff)
	rec.RiskCutSummary = riskCutSummaryFrom(alloc.RiskCutDiff)
}

func budgetSummaryFrom(d BudgetDiff) *AllocationBudgetSummary {
	return &AllocationBudgetSummary{
		LegacyHasBudget:       d.LegacyHasBudget,
		Portfolio:             d.Portfolio,
		ImpliedLegacyNotional: d.ImpliedLegacyNotional,
		CapitalVsImpliedDelta: d.CapitalVsImpliedDelta,
		Binding:               d.Binding,
		Note:                  d.Note,
	}
}

func reserveSummaryFrom(d ReserveDiff) *ReserveSummary {
	return &ReserveSummary{
		LegacyReserve:    d.LegacyReserve,
		PortfolioReserve: d.PortfolioReserve,
		Delta:            d.Delta,
	}
}

func riskCutSummaryFrom(d RiskCutDiff) *RiskCutSummary {
	return &RiskCutSummary{
		PortfolioBinding:       d.PortfolioBinding,
		SingleCapApplied:       d.SingleCapApplied,
		MinOrderZeroCount:      d.MinOrderZeroCount,
		GrossHeadroomBinding:   d.GrossHeadroomBinding,
		LegacySizerIgnoresCaps: d.LegacySizerIgnoresCaps,
		BlockedNewEntries:      d.BlockedNewEntries,
	}
}

// stampReportFlags ensures H3.1 observation flags on every report.
func stampReportFlags(report *ProviderComparisonReport) {
	if report == nil {
		return
	}
	report.RecordOnly = true
	report.NotATradePlan = true
	report.NotAProviderSwitch = true
	report.ChainProvider = decisionprovider.ProviderFixedAmount
}
