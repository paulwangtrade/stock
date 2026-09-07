package controlledmonitor

import (
	"math"
	"strings"
	"time"

	"go-stock/backend/providershadow"
)

// Input injects read-only pilot observation facts. Any field may be nil/empty.
type Input struct {
	// Enabled must be true to build a filled report. Default false → skipped.
	Enabled bool

	AsOf      time.Time
	TradeDate string

	Shadow   *providershadow.ShadowComparisonRecord
	Metadata ProviderMetadata

	LegacyAllocation    *AllocationSide
	PortfolioAllocation *AllocationSide

	LegacyFilter    *FilterSide
	PortfolioFilter *FilterSide

	Notes []string
}

// Observe builds ControlledPilotObservationReport. Pure function; no I/O, no writes.
func Observe(in Input) *ControlledPilotObservationReport {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	rep := &ControlledPilotObservationReport{
		SchemaVersion:      SchemaVersion,
		AsOf:               asOf,
		TradeDate:          strings.TrimSpace(in.TradeDate),
		Enabled:            in.Enabled,
		RecordOnly:         true,
		ReadOnly:           true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotProviderSwitch:  true,
		NotAutoExpandScope: true,
		DataGaps:           []string{},
		Notes: append([]string{
			"controlled pilot observation · read-only",
			"does not write TradePlan / call Execution / switch Provider",
			"default disabled; Enabled=true required to fill diffs",
		}, in.Notes...),
		DataSourceNote: "controlledmonitor.Observe · injected Shadow + metadata + allocation + filter; not write-chain",
	}

	if !in.Enabled {
		rep.Skipped = true
		rep.SkipReason = "enabled=false (default OFF; Controlled Pilot Observation Monitor)"
		rep.Outcome = Outcome{Code: OutcomeSkipped, Detail: rep.SkipReason}
		return rep
	}

	rep.Sources = SourceFlags{
		ShadowPresent:          in.Shadow != nil,
		MetadataPresent:        metadataPresent(in.Metadata),
		LegacyAllocPresent:     in.LegacyAllocation != nil && in.LegacyAllocation.Present,
		PortfolioAllocPresent:  in.PortfolioAllocation != nil && in.PortfolioAllocation.Present,
		LegacyFilterPresent:    in.LegacyFilter != nil && in.LegacyFilter.Present,
		PortfolioFilterPresent: in.PortfolioFilter != nil && in.PortfolioFilter.Present,
	}
	if in.Shadow == nil {
		rep.DataGaps = append(rep.DataGaps, "shadow_missing")
	}
	if !rep.Sources.MetadataPresent {
		rep.DataGaps = append(rep.DataGaps, "provider_metadata_missing")
	}
	if !rep.Sources.LegacyAllocPresent && !rep.Sources.PortfolioAllocPresent && in.Shadow == nil {
		rep.DataGaps = append(rep.DataGaps, "allocation_missing")
	}
	if !rep.Sources.LegacyFilterPresent && !rep.Sources.PortfolioFilterPresent {
		rep.DataGaps = append(rep.DataGaps, "filter_missing")
	}

	rep.ProviderMetadata = in.Metadata
	rep.ProviderUsed = resolveProviderUsed(in)

	if in.TradeDate == "" && in.Shadow != nil {
		rep.TradeDate = strings.TrimSpace(in.Shadow.TradeDate)
	}

	fillFromShadow(rep, in.Shadow)
	fillFromAllocation(rep, in.LegacyAllocation, in.PortfolioAllocation)
	fillFromFilter(rep, in.LegacyFilter, in.PortfolioFilter)
	fillCashUsage(rep, in)

	rep.Outcome = deriveOutcome(in)
	rep.Success = rep.Outcome.Success
	rep.Failure = rep.Outcome.Failure
	return rep
}

func metadataPresent(m ProviderMetadata) bool {
	return strings.TrimSpace(m.ProviderMode) != "" ||
		strings.TrimSpace(m.DecisionProvider) != "" ||
		strings.TrimSpace(m.DecisionVersion) != "" ||
		strings.TrimSpace(m.AllocationVersion) != ""
}

func resolveProviderUsed(in Input) string {
	if dp := strings.TrimSpace(in.Metadata.DecisionProvider); dp != "" {
		return dp
	}
	if in.Shadow != nil && in.Shadow.Report != nil {
		if p := strings.TrimSpace(in.Shadow.Report.Portfolio.Provider); p != "" && in.Shadow.Comparable {
			// Shadow always observes both; write-chain identity is metadata.
			_ = p
		}
		if chain := strings.TrimSpace(in.Shadow.Report.ChainProvider); chain != "" {
			return chain
		}
	}
	if in.PortfolioAllocation != nil && in.PortfolioAllocation.Present && in.PortfolioAllocation.OK {
		if in.Metadata.ProviderMode == "controlled" {
			return ProviderPortfolioAllocation
		}
	}
	if in.LegacyAllocation != nil && in.LegacyAllocation.Present {
		return ProviderFixedAmount
	}
	return ""
}

func deriveOutcome(in Input) Outcome {
	if in.Shadow != nil {
		fail := in.Shadow.Failure
		if !fail.PortfolioFailed && in.Shadow.FailureSummary.PortfolioFailed {
			fail = in.Shadow.FailureSummary
		}
		if fail.PortfolioFailed {
			legOK := in.Shadow.LegacySummary.OK
			if in.Shadow.Report != nil {
				legOK = in.Shadow.Report.Legacy.OK
			}
			if !legOK {
				return Outcome{Failure: true, Code: OutcomeBothFail, Detail: fail.ErrorReason}
			}
			return Outcome{Failure: true, Code: OutcomePortfolioFail, Detail: fail.ErrorReason}
		}
		if !in.Shadow.Comparable {
			reason := strings.TrimSpace(in.Shadow.IncomparableReason)
			if reason == "" && in.Shadow.Report != nil {
				reason = in.Shadow.Report.IncomparableReason
			}
			switch reason {
			case providershadow.IncomparableLegacyFailed:
				return Outcome{Failure: true, Code: OutcomeLegacyFail, Detail: reason}
			case providershadow.IncomparableBothFailed:
				return Outcome{Failure: true, Code: OutcomeBothFail, Detail: reason}
			case providershadow.IncomparablePortfolioFailed:
				return Outcome{Failure: true, Code: OutcomePortfolioFail, Detail: reason}
			default:
				if reason == "" {
					reason = "incomparable"
				}
				return Outcome{Failure: true, Code: OutcomeIncomparable, Detail: reason}
			}
		}
		portOK := in.Shadow.PortfolioSummary.OK
		legOK := in.Shadow.LegacySummary.OK
		if in.Shadow.Report != nil {
			portOK = in.Shadow.Report.Portfolio.OK
			legOK = in.Shadow.Report.Legacy.OK
		}
		if portOK && legOK {
			return Outcome{Success: true, Code: OutcomeOK}
		}
		if !portOK && !legOK {
			return Outcome{Failure: true, Code: OutcomeBothFail}
		}
		if !portOK {
			return Outcome{Failure: true, Code: OutcomePortfolioFail}
		}
		return Outcome{Failure: true, Code: OutcomeLegacyFail}
	}

	// No shadow: infer from injected sides.
	port := in.PortfolioAllocation
	leg := in.LegacyAllocation
	if port != nil && port.Present && !port.OK {
		if leg != nil && leg.Present && !leg.OK {
			return Outcome{Failure: true, Code: OutcomeBothFail, Detail: port.ErrorCode}
		}
		return Outcome{Failure: true, Code: OutcomePortfolioFail, Detail: port.ErrorCode}
	}
	if leg != nil && leg.Present && !leg.OK {
		return Outcome{Failure: true, Code: OutcomeLegacyFail, Detail: leg.ErrorCode}
	}
	if (port != nil && port.Present && port.OK) || (leg != nil && leg.Present && leg.OK) {
		return Outcome{Success: true, Code: OutcomeOK}
	}
	return Outcome{Code: OutcomePartialInputs, Detail: "insufficient allocation/shadow inputs"}
}

func fillFromShadow(rep *ControlledPilotObservationReport, shadow *providershadow.ShadowComparisonRecord) {
	if shadow == nil {
		return
	}
	sum := shadow.ComparisonSummary
	if shadow.Report != nil {
		m := shadow.Report.Metrics
		if sum.OnlyLegacyCount == 0 && sum.OnlyPortfolioCount == 0 && sum.CommonCount == 0 {
			sum.OnlyLegacyCount = m.OnlyLegacyCount
			sum.OnlyPortfolioCount = m.OnlyPortfolioCount
			sum.CommonCount = m.CommonCount
			sum.AmountDeltaCount = m.AmountDiffCount
			sum.RoleChangeCount = m.RoleDiffCount
		}
	}

	legN := shadow.LegacySummary.LineCount
	portN := shadow.PortfolioSummary.LineCount
	if shadow.Report != nil {
		if legN == 0 {
			legN = shadow.Report.Legacy.LineCount
		}
		if portN == 0 {
			portN = shadow.Report.Portfolio.LineCount
		}
	}

	rep.SymbolCountDiff = SymbolCountDiff{
		Present:            true,
		LegacyCount:        legN,
		PortfolioCount:     portN,
		Delta:              portN - legN,
		OnlyLegacyCount:    sum.OnlyLegacyCount,
		OnlyPortfolioCount: sum.OnlyPortfolioCount,
		CommonCount:        sum.CommonCount,
	}

	amount := AmountDiffBlock{Present: true, AmountDeltaCount: sum.AmountDeltaCount}
	if shadow.Report != nil {
		var deltaSum float64
		var absSum float64
		var n int
		var legSum, portSum float64
		for _, a := range shadow.Report.Amounts {
			if a.Kind != providershadow.KindBothActual || a.Delta == nil {
				continue
			}
			d := *a.Delta
			if math.IsNaN(d) || math.IsInf(d, 0) {
				continue
			}
			deltaSum += d
			absSum += math.Abs(d)
			n++
			if a.Legacy.Value != nil {
				legSum += *a.Legacy.Value
			}
			if a.Portfolio.Value != nil {
				portSum += *a.Portfolio.Value
			}
		}
		amount.SampleCount = n
		if n > 0 {
			amount.LegacySum = legSum
			amount.PortfolioSum = portSum
			amount.Delta = deltaSum
			amount.AbsDelta = absSum
		}
	}
	if !amountPresent(amount) && shadow.AllocationBudgetSummary != nil {
		b := shadow.AllocationBudgetSummary
		amount.Present = true
		amount.LegacySum = b.ImpliedLegacyNotional
		amount.PortfolioSum = b.Portfolio.AvailableCapital
		amount.Delta = b.CapitalVsImpliedDelta
		amount.AbsDelta = math.Abs(b.CapitalVsImpliedDelta)
	}
	rep.AmountDiff = amount

	alloc := AllocationDiff{Present: true}
	alloc.LegacySelectedCount = legN
	alloc.PortfolioSelectedCount = portN
	alloc.SelectedCountDelta = portN - legN
	if shadow.Report != nil && shadow.Report.Allocation != nil {
		ac := shadow.Report.Allocation
		alloc.LegacySelectedCount = ac.Legacy.AllocationSetCount
		alloc.PortfolioSelectedCount = ac.Portfolio.AllocationSetCount
		alloc.SelectedCountDelta = alloc.PortfolioSelectedCount - alloc.LegacySelectedCount
		alloc.LegacyWaitlistCount = ac.Legacy.WaitlistCount
		alloc.PortfolioWaitlistCount = ac.Portfolio.WaitlistCount
		alloc.LegacyMethod = ac.Legacy.Method
		alloc.PortfolioMethod = ac.Portfolio.Method
		alloc.PortfolioBinding = ac.BudgetDiff.Binding
		if ac.BudgetDiff.Binding == "" {
			alloc.PortfolioBinding = ac.Portfolio.Binding
		}
		alloc.CapitalDelta = ac.BudgetDiff.CapitalVsImpliedDelta
	}
	if shadow.RiskCutSummary != nil {
		alloc.PortfolioBinding = firstNonEmpty(alloc.PortfolioBinding, shadow.RiskCutSummary.PortfolioBinding)
	}
	rep.AllocationDiff = alloc

	if shadow.ReserveSummary != nil || shadow.AllocationBudgetSummary != nil {
		cash := CashUsageDiff{Present: true}
		if shadow.ReserveSummary != nil {
			cash.LegacyReserve = shadow.ReserveSummary.LegacyReserve
			cash.PortfolioReserve = shadow.ReserveSummary.PortfolioReserve
			cash.ReserveDelta = shadow.ReserveSummary.Delta
		}
		if shadow.AllocationBudgetSummary != nil {
			b := shadow.AllocationBudgetSummary
			cash.LegacyBuyNotional = b.ImpliedLegacyNotional
			cash.PortfolioBuyNotional = b.Portfolio.AvailableCapital
			cash.BuyNotionalDelta = b.CapitalVsImpliedDelta
		} else if amount.Present && amount.SampleCount > 0 {
			cash.LegacyBuyNotional = amount.LegacySum
			cash.PortfolioBuyNotional = amount.PortfolioSum
			cash.BuyNotionalDelta = amount.Delta
		}
		rep.CashUsageDiff = cash
	}
}

func amountPresent(a AmountDiffBlock) bool {
	return a.Present && (a.SampleCount > 0 || a.AmountDeltaCount > 0 || a.LegacySum != 0 || a.PortfolioSum != 0 || a.Delta != 0)
}

func fillFromAllocation(rep *ControlledPilotObservationReport, legacy, portfolio *AllocationSide) {
	if legacy == nil && portfolio == nil {
		return
	}
	legOK := legacy != nil && legacy.Present
	portOK := portfolio != nil && portfolio.Present
	if !legOK && !portOK {
		return
	}
	// Explicit sides override / fill gaps when Shadow did not set Present.
	if !rep.AllocationDiff.Present {
		rep.AllocationDiff.Present = true
	}
	d := rep.AllocationDiff
	if legOK {
		d.LegacySelectedCount = legacy.SelectedCount
		d.LegacyWaitlistCount = legacy.WaitlistCount
		d.LegacyMethod = legacy.Method
		d.LegacyBinding = legacy.Binding
	}
	if portOK {
		d.PortfolioSelectedCount = portfolio.SelectedCount
		d.PortfolioWaitlistCount = portfolio.WaitlistCount
		d.PortfolioMethod = portfolio.Method
		d.PortfolioBinding = portfolio.Binding
	}
	if legOK && portOK {
		d.SelectedCountDelta = portfolio.SelectedCount - legacy.SelectedCount
		d.CapitalDelta = portfolio.AvailableCapital - legacy.AvailableCapital
	}
	rep.AllocationDiff = d

	if !rep.SymbolCountDiff.Present && (legOK || portOK) {
		sc := SymbolCountDiff{Present: true}
		if legOK {
			sc.LegacyCount = legacy.SelectedCount
		}
		if portOK {
			sc.PortfolioCount = portfolio.SelectedCount
		}
		sc.Delta = sc.PortfolioCount - sc.LegacyCount
		rep.SymbolCountDiff = sc
	}

	if !rep.AmountDiff.Present && (legOK || portOK) {
		am := AmountDiffBlock{Present: true}
		if legOK {
			am.LegacySum = legacy.SumNotional
		}
		if portOK {
			am.PortfolioSum = portfolio.SumNotional
		}
		am.Delta = am.PortfolioSum - am.LegacySum
		am.AbsDelta = math.Abs(am.Delta)
		rep.AmountDiff = am
	}
}

func fillFromFilter(rep *ControlledPilotObservationReport, legacy, portfolio *FilterSide) {
	if legacy == nil && portfolio == nil {
		return
	}
	legOK := legacy != nil && legacy.Present
	portOK := portfolio != nil && portfolio.Present
	if !legOK && !portOK {
		return
	}
	d := FilterRejectDiff{Present: true}
	if legOK {
		d.LegacyAccepted = legacy.AcceptedCount
		d.LegacyRejected = legacy.RejectedCount
		d.LegacyRiskStatus = legacy.RiskStatus
	}
	if portOK {
		d.PortfolioAccepted = portfolio.AcceptedCount
		d.PortfolioRejected = portfolio.RejectedCount
		d.PortfolioRiskStatus = portfolio.RiskStatus
		if len(portfolio.RejectReasons) > 0 {
			d.PortfolioRejectReasons = copyReasonMap(portfolio.RejectReasons)
		}
	}
	d.AcceptedDelta = d.PortfolioAccepted - d.LegacyAccepted
	d.RejectedDelta = d.PortfolioRejected - d.LegacyRejected
	rep.FilterRejectDiff = d
}

func fillCashUsage(rep *ControlledPilotObservationReport, in Input) {
	if rep.CashUsageDiff.Present {
		return
	}
	leg := in.LegacyAllocation
	port := in.PortfolioAllocation
	legOK := leg != nil && leg.Present
	portOK := port != nil && port.Present
	if !legOK && !portOK {
		return
	}
	c := CashUsageDiff{Present: true}
	if legOK {
		c.LegacyBuyNotional = leg.SumNotional
		c.LegacyReserve = leg.ReserveCash
	}
	if portOK {
		c.PortfolioBuyNotional = port.SumNotional
		if port.SumNotional == 0 && port.AvailableCapital > 0 {
			c.PortfolioBuyNotional = port.AvailableCapital
		}
		c.PortfolioReserve = port.ReserveCash
	}
	c.BuyNotionalDelta = c.PortfolioBuyNotional - c.LegacyBuyNotional
	c.ReserveDelta = c.PortfolioReserve - c.LegacyReserve
	rep.CashUsageDiff = c
}

func copyReasonMap(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
