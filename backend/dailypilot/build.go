package dailypilot

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-stock/backend/controlledmonitor"
	"go-stock/backend/portfoliohistory"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/providershadow"
)

const exportNote = "local-only Controlled Pilot daily observation; does not enable Controlled, modify Provider, write TradePlan, or auto-trade"

// BuildDailyPilotReport assembles a read-only daily pilot ticket. Default OFF unless in.Enabled.
func BuildDailyPilotReport(in DailyPilotInput) *DailyPilotReport {
	at := in.GeneratedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	tradeDate := strings.TrimSpace(in.TradeDate)

	rep := &DailyPilotReport{
		SchemaVersion:      SchemaVersion,
		TradeDate:          tradeDate,
		GeneratedAt:        at,
		ReviewID:           strings.TrimSpace(in.ReviewID),
		Enabled:            in.Enabled,
		RecordOnly:         true,
		ReadOnly:           true,
		NotAutoTune:        true,
		NotAutoExpandScope: true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotProviderSwitch:  true,
		NotABacktest:       true,
		KnownGaps:          []string{"write_chain_may_omit_tighten"},
		HumanVerdict: HumanVerdictBlock{
			Status:           verdictUnset,
			NextDateApproved: false,
		},
		ExportNote: exportNote,
	}

	if !in.Enabled {
		rep.Skipped = true
		rep.SkipReason = "enabled=false (default OFF; DailyPilotReport observation only)"
		return rep
	}

	openOK := policyPresent(in.PolicySnapshotOpen)
	closeOK := policyPresent(in.PolicySnapshotClose)
	policy := in.PolicySnapshotClose
	if !closeOK {
		policy = in.PolicySnapshotOpen
	}
	rep.Scope = projectScope(policy)

	records := filterShadowRecords(in.ShadowRecords, tradeDate)
	rep.Sources = buildSources(in, records, tradeDate)
	rep.DataGaps = buildDataGaps(in, records, tradeDate)

	daily := providershadow.BuildPortfolioShadowDailyReport(records, tradeDate)
	histDay := pickHistoryDay(in.PortfolioHistory, tradeDate)
	rep.LegacyVsPortfolio = buildLegacyVsPortfolio(daily, records, in.Validation, tradeDate, in.ControlledMonitor)
	rep.Allocation = buildAllocation(records, daily, histDay)
	rep.RiskConstraint = buildRiskConstraint(records, in.Validation, tradeDate, histDay)
	rep.Failure = buildFailure(records, in.Validation, tradeDate, in.WriteChainHints, rep.Scope, rep.LegacyVsPortfolio)
	rep.Rollback = buildRollback(tradeDate, in.PolicySnapshotOpen, in.PolicySnapshotClose, openOK, closeOK)

	return rep
}

func buildSources(in DailyPilotInput, records []providershadow.ShadowComparisonRecord, tradeDate string) SourcePresence {
	src := SourcePresence{
		ShadowPresent:     len(records) > 0,
		ShadowRecordCount: len(records),
	}
	if in.Validation != nil {
		src.ValidationPresent = true
		src.ValidationSkipped = in.Validation.Skipped || !in.Validation.Enabled
	}
	if in.ControlledMonitor != nil {
		src.MonitorPresent = true
		src.MonitorSkipped = in.ControlledMonitor.Skipped || !in.ControlledMonitor.Enabled
	}
	if in.PortfolioHistory != nil {
		src.HistoryPresent = true
		src.HistoryDayMatched = pickHistoryDay(in.PortfolioHistory, tradeDate) != nil
	}
	src.PolicyOpenPresent = policyPresent(in.PolicySnapshotOpen)
	src.PolicyClosePresent = policyPresent(in.PolicySnapshotClose)
	src.WriteChainHintsPresent = in.WriteChainHints != nil
	return src
}

func buildDataGaps(in DailyPilotInput, records []providershadow.ShadowComparisonRecord, tradeDate string) []string {
	var gaps []string
	if len(records) == 0 {
		gaps = append(gaps, "shadow_records_missing")
	}
	if in.Validation == nil {
		gaps = append(gaps, "validation_missing")
	} else if in.Validation.Skipped || !in.Validation.Enabled {
		gaps = append(gaps, "validation_skipped")
	}
	if in.ControlledMonitor == nil {
		gaps = append(gaps, "controlled_monitor_missing")
	} else if in.ControlledMonitor.Skipped || !in.ControlledMonitor.Enabled {
		gaps = append(gaps, "controlled_monitor_skipped")
	}
	if in.PortfolioHistory == nil {
		gaps = append(gaps, "portfolio_history_missing")
	} else if pickHistoryDay(in.PortfolioHistory, tradeDate) == nil {
		gaps = append(gaps, "portfolio_history_day_unmatched")
	}
	if !policyPresent(in.PolicySnapshotOpen) && !policyPresent(in.PolicySnapshotClose) {
		gaps = append(gaps, "policy_snapshot_missing")
	}
	return gaps
}

func filterShadowRecords(records []providershadow.ShadowComparisonRecord, tradeDate string) []providershadow.ShadowComparisonRecord {
	if tradeDate == "" {
		out := make([]providershadow.ShadowComparisonRecord, len(records))
		copy(out, records)
		return out
	}
	out := make([]providershadow.ShadowComparisonRecord, 0, len(records))
	for _, r := range records {
		if strings.TrimSpace(r.TradeDate) == tradeDate {
			out = append(out, r)
		}
	}
	return out
}

func buildLegacyVsPortfolio(
	daily *providershadow.PortfolioShadowDailyReport,
	records []providershadow.ShadowComparisonRecord,
	validation *portfoliovalidation.PortfolioValidationReport,
	tradeDate string,
	monitor *controlledmonitor.ControlledPilotObservationReport,
) LegacyVsPortfolioBlock {
	block := LegacyVsPortfolioBlock{
		IncomparableReasons: []ReasonCount{},
		ValidationOverlay:   ValidationOverlayBlock{},
		MonitorOverlay:      MonitorOverlayBlock{},
	}
	if daily != nil {
		block.ComparableCount = daily.ComparableCount
		block.IncomparableCount = daily.IncomparableCount
		block.NameCount = NameCountBlock{
			AvgLegacy:    daily.AvgLegacyStockCount,
			AvgPortfolio: daily.AvgPortfolioStockCount,
		}
		if daily.ComparableCount > 0 {
			block.NameCount.DeltaMean = daily.AvgPortfolioStockCount - daily.AvgLegacyStockCount
		}
		block.SymbolDiff = SymbolDiffBlock{
			OnlyLegacySample:    sampleSymbols(daily.OnlyLegacyStocks, maxSymbolSample),
			OnlyPortfolioSample: sampleSymbols(daily.OnlyPortfolioStocks, maxSymbolSample),
		}
		block.Amount = AmountBlock{SampleCount: daily.AmountDiffSampleCount}
		if daily.AmountDiffSampleCount > 0 {
			block.Amount.AvgSignedDelta = daily.AvgAmountDelta
			block.Amount.AvgAbsDelta = daily.AvgAbsAmountDelta
		}
	}
	block.IncomparableReasons = incomparableHistogram(records)
	block.SymbolDiff.OnlyLegacyCount, block.SymbolDiff.OnlyPortfolioCount, block.SymbolDiff.CommonCount = symbolSetCounts(records)
	block.Amount.AmountDeltaCount = amountDeltaCountSum(records)
	block.RoleChangeCount = roleChangeCountSum(records)
	block.ValidationOverlay = validationOverlay(validation, tradeDate)
	block.MonitorOverlay = monitorOverlay(monitor)
	return block
}

func incomparableHistogram(records []providershadow.ShadowComparisonRecord) []ReasonCount {
	counts := map[string]int{}
	for _, rec := range records {
		comparable := rec.Comparable
		reason := strings.TrimSpace(rec.IncomparableReason)
		if rec.Report != nil {
			if !rec.Report.Comparable {
				comparable = false
				if reason == "" {
					reason = strings.TrimSpace(rec.Report.IncomparableReason)
				}
			}
		}
		if comparable {
			continue
		}
		if reason == "" {
			reason = "unknown"
		}
		counts[reason]++
	}
	return topReasonCounts(counts, maxReasons)
}

func symbolSetCounts(records []providershadow.ShadowComparisonRecord) (onlyL, onlyP, common int) {
	for _, rec := range records {
		sum := rec.ComparisonSummary
		if rec.Report != nil {
			m := rec.Report.Metrics
			if sum.OnlyLegacyCount == 0 && sum.OnlyPortfolioCount == 0 && sum.CommonCount == 0 {
				sum.OnlyLegacyCount = m.OnlyLegacyCount
				sum.OnlyPortfolioCount = m.OnlyPortfolioCount
				sum.CommonCount = m.CommonCount
			}
		}
		onlyL += sum.OnlyLegacyCount
		onlyP += sum.OnlyPortfolioCount
		common += sum.CommonCount
	}
	return onlyL, onlyP, common
}

func amountDeltaCountSum(records []providershadow.ShadowComparisonRecord) int {
	n := 0
	for _, rec := range records {
		n += rec.ComparisonSummary.AmountDeltaCount
		if rec.Report != nil && rec.ComparisonSummary.AmountDeltaCount == 0 {
			n += rec.Report.Metrics.AmountDiffCount
		}
	}
	return n
}

func roleChangeCountSum(records []providershadow.ShadowComparisonRecord) int {
	n := 0
	for _, rec := range records {
		n += rec.ComparisonSummary.RoleChangeCount
		if rec.Report != nil && rec.ComparisonSummary.RoleChangeCount == 0 {
			n += rec.Report.Metrics.RoleDiffCount
		}
	}
	return n
}

func validationOverlay(validation *portfoliovalidation.PortfolioValidationReport, tradeDate string) ValidationOverlayBlock {
	if validation == nil {
		return ValidationOverlayBlock{}
	}
	if validation.Skipped || !validation.Enabled {
		return ValidationOverlayBlock{Used: true, Skipped: true}
	}
	day := pickValidationDay(validation, tradeDate)
	if day == nil {
		out := ValidationOverlayBlock{
			Used:           true,
			DayUnmatched:   true,
			NameCountDelta: int(validation.Summary.NameCountDeltaMean),
		}
		if validation.Summary.NotionalDeltaMean != 0 {
			out.NotionalDelta = validation.Summary.NotionalDeltaMean
		}
		return out
	}
	return ValidationOverlayBlock{
		Used:           true,
		NameCountDelta: day.Difference.NameCountDelta,
		NotionalDelta:  day.PortfolioDecision.BuyNotionalSum - day.LegacyDecision.BuyNotionalSum,
	}
}

func pickValidationDay(validation *portfoliovalidation.PortfolioValidationReport, tradeDate string) *portfoliovalidation.DayValidation {
	if validation == nil || len(validation.Days) == 0 {
		return nil
	}
	td := strings.TrimSpace(tradeDate)
	if td != "" {
		for i := range validation.Days {
			if strings.TrimSpace(validation.Days[i].TradeDate) == td {
				return &validation.Days[i]
			}
		}
		return nil
	}
	if len(validation.Days) == 1 {
		return &validation.Days[0]
	}
	return nil
}

func monitorOverlay(monitor *controlledmonitor.ControlledPilotObservationReport) MonitorOverlayBlock {
	if monitor == nil || monitor.Skipped || !monitor.Enabled {
		return MonitorOverlayBlock{}
	}
	return MonitorOverlayBlock{
		Present:          true,
		OutcomeCode:      monitor.Outcome.Code,
		ProviderUsed:     monitor.ProviderUsed,
		SymbolCountDelta: monitor.SymbolCountDiff.Delta,
		AcceptedDelta:    monitor.FilterRejectDiff.AcceptedDelta,
		RejectedDelta:    monitor.FilterRejectDiff.RejectedDelta,
	}
}

func buildAllocation(records []providershadow.ShadowComparisonRecord, daily *providershadow.PortfolioShadowDailyReport, day *portfoliohistory.DailyRecord) AllocationBlock {
	block := AllocationBlock{
		BindingHistogram: []ReasonCount{},
		ChangeReasons:    []ReasonCount{},
	}
	bindings := map[string]int{}
	var (
		reserveLegacy, reservePort, reserveDelta float64
		reserveN                                 int
		budgetLegacyCount                        int
		capitalDeltaSum                          float64
		budgetN                                  int
		waitLegacy, waitPort                     float64
		waitN                                    int
		minOrderZero                             int
	)
	for _, rec := range records {
		if rec.AllocationBudgetSummary != nil {
			block.RecordsWithAllocSummary++
			b := rec.AllocationBudgetSummary
			if b.LegacyHasBudget {
				budgetLegacyCount++
			}
			if b.Binding != "" {
				bindings[b.Binding]++
			}
			capitalDeltaSum += b.CapitalVsImpliedDelta
			budgetN++
		}
		if rec.ReserveSummary != nil {
			rs := rec.ReserveSummary
			reserveLegacy += rs.LegacyReserve
			reservePort += rs.PortfolioReserve
			reserveDelta += rs.Delta
			reserveN++
		}
		if rec.RiskCutSummary != nil {
			rc := rec.RiskCutSummary
			minOrderZero += rc.MinOrderZeroCount
			if rc.PortfolioBinding != "" {
				bindings[rc.PortfolioBinding]++
			}
		}
		if rec.Report != nil && rec.Report.Allocation != nil && rec.Report.Allocation.Comparable {
			block.ComparableAllocCount++
			ac := rec.Report.Allocation
			waitLegacy += float64(ac.Legacy.WaitlistCount)
			waitPort += float64(ac.Portfolio.WaitlistCount)
			waitN++
		}
	}
	block.BindingHistogram = topReasonCounts(bindings, maxReasons)
	block.Reserve = ReserveBlock{SampleCount: reserveN}
	if reserveN > 0 {
		n := float64(reserveN)
		block.Reserve.AvgLegacy = reserveLegacy / n
		block.Reserve.AvgPortfolio = reservePort / n
		block.Reserve.DeltaMean = reserveDelta / n
	}
	block.Budget = BudgetBlock{
		LegacyHasBudgetCount: budgetLegacyCount,
		SampleCount:          budgetN,
	}
	if budgetN > 0 {
		block.Budget.CapitalVsImpliedDeltaMean = capitalDeltaSum / float64(budgetN)
	}
	block.Waitlist = WaitlistBlock{SampleCount: waitN}
	if waitN > 0 {
		n := float64(waitN)
		block.Waitlist.AvgLegacyCount = waitLegacy / n
		block.Waitlist.AvgPortfolioCount = waitPort / n
	}
	block.MinOrderZeroCountSum = minOrderZero
	if daily != nil {
		reasons := make([]ReasonCount, 0, len(daily.AllocationChangeReasons))
		for _, r := range daily.AllocationChangeReasons {
			reasons = append(reasons, ReasonCount{Code: r.Reason, Count: r.Count})
		}
		block.ChangeReasons = reasons
	}
	block.HistoryShape = historyShape(day)
	return block
}

func historyShape(day *portfoliohistory.DailyRecord) HistoryShapeBlock {
	if day == nil {
		return HistoryShapeBlock{}
	}
	out := HistoryShapeBlock{Available: true}
	if day.Risk.Present && day.Risk.Found {
		out.GrossHeadroomVsCap = day.Risk.HeadroomVsCap
		out.Top1Weight = day.Risk.Top1Weight
		out.CashRatio = day.Risk.CashRatio
		out.NameCount = day.Risk.NameCount
	}
	return out
}

func buildRiskConstraint(
	records []providershadow.ShadowComparisonRecord,
	validation *portfoliovalidation.PortfolioValidationReport,
	tradeDate string,
	day *portfoliohistory.DailyRecord,
) RiskConstraintBlock {
	block := RiskConstraintBlock{
		ShadowTighten: ShadowTightenBlock{NoteCodes: []string{}},
		HistoryRisk:   ObservationRiskBlock{},
		Validation:    ValidationRiskBlock{},
	}
	noteCodes := map[string]struct{}{}
	var lastTrace *providershadow.RiskAdjustmentTrace
	for _, rec := range records {
		trace := rec.RiskConstraintTrace
		if trace == nil {
			trace = rec.RiskAdjustment
		}
		if trace == nil && rec.Report != nil {
			trace = rec.Report.RiskAdjustment
		}
		if trace == nil {
			if rec.RiskCutSummary != nil {
				rc := rec.RiskCutSummary
				if rc.SingleCapApplied {
					block.ShadowCuts.SingleCapAppliedCount++
				}
				if rc.GrossHeadroomBinding {
					block.ShadowCuts.GrossHeadroomBindingCount++
				}
				if rc.BlockedNewEntries {
					block.ShadowCuts.BlockedNewEntriesCount++
				}
				block.ShadowCuts.MinOrderZeroCountSum += rc.MinOrderZeroCount
				if rc.LegacySizerIgnoresCaps {
					block.ShadowCuts.LegacyIgnoresCaps = true
				}
			}
			continue
		}
		block.ShadowTighten.Present = true
		if trace.Applied {
			block.ShadowTighten.AppliedCount++
		}
		if trace.SuggestHasPatches {
			block.ShadowTighten.SuggestHasPatchesCount++
		}
		for _, n := range trace.Notes {
			code := strings.TrimSpace(n.Field)
			if code == "" {
				code = strings.TrimSpace(n.Reason)
			}
			if code != "" {
				noteCodes[code] = struct{}{}
			}
		}
		lastTrace = trace
		if rec.RiskCutSummary != nil {
			rc := rec.RiskCutSummary
			if rc.SingleCapApplied {
				block.ShadowCuts.SingleCapAppliedCount++
			}
			if rc.GrossHeadroomBinding {
				block.ShadowCuts.GrossHeadroomBindingCount++
			}
			if rc.BlockedNewEntries {
				block.ShadowCuts.BlockedNewEntriesCount++
			}
			block.ShadowCuts.MinOrderZeroCountSum += rc.MinOrderZeroCount
			if rc.LegacySizerIgnoresCaps {
				block.ShadowCuts.LegacyIgnoresCaps = true
			}
		}
	}
	if lastTrace != nil {
		block.ShadowTighten.EffectiveMaxNewNames = lastTrace.EffectiveMaxNewNames
		block.ShadowTighten.SkipAlreadyHolding = lastTrace.EffectiveSkipAlreadyHolding
	}
	for code := range noteCodes {
		block.ShadowTighten.NoteCodes = append(block.ShadowTighten.NoteCodes, code)
	}
	sort.Strings(block.ShadowTighten.NoteCodes)

	block.Validation = validationRisk(validation, tradeDate)
	if day != nil && day.Risk.Present {
		block.HistoryRisk.Present = true
		block.HistoryRisk.AllowSectorConstraint = day.Risk.SectorAvailable
	}
	return block
}

func validationRisk(validation *portfoliovalidation.PortfolioValidationReport, tradeDate string) ValidationRiskBlock {
	if validation == nil {
		return ValidationRiskBlock{}
	}
	out := ValidationRiskBlock{Present: true}
	if validation.Skipped || !validation.Enabled {
		out.Skipped = true
		return out
	}
	day := pickValidationDay(validation, tradeDate)
	if day != nil {
		out.TightenCountLegacy = day.Difference.RiskTightenCountLegacy
		out.TightenCountPortfolio = day.Difference.RiskTightenCountPortfolio
		reasons := make([]ReasonCount, 0, len(day.Difference.FilterRejectReasonsPortfolio))
		for code, n := range day.Difference.FilterRejectReasonsPortfolio {
			reasons = append(reasons, ReasonCount{Code: code, Count: n})
		}
		sort.Slice(reasons, func(i, j int) bool {
			if reasons[i].Count != reasons[j].Count {
				return reasons[i].Count > reasons[j].Count
			}
			return reasons[i].Code < reasons[j].Code
		})
		out.FilterRejectReasonsPortfolio = reasons
		return out
	}
	out.TightenCountPortfolio = validation.Summary.TightenCountPortfolioTotal
	reasons := make([]ReasonCount, 0, len(validation.Summary.FilterRejectReasonTotals))
	for code, n := range validation.Summary.FilterRejectReasonTotals {
		reasons = append(reasons, ReasonCount{Code: code, Count: n})
	}
	sort.Slice(reasons, func(i, j int) bool {
		if reasons[i].Count != reasons[j].Count {
			return reasons[i].Count > reasons[j].Count
		}
		return reasons[i].Code < reasons[j].Code
	})
	out.FilterRejectReasonsPortfolio = reasons
	return out
}

func buildFailure(
	records []providershadow.ShadowComparisonRecord,
	validation *portfoliovalidation.PortfolioValidationReport,
	tradeDate string,
	hints *WriteChainHints,
	scope PilotScope,
	lvp LegacyVsPortfolioBlock,
) FailureBlock {
	block := FailureBlock{
		HumanWatchCodes: []string{},
	}
	errHist := map[string]int{}
	for _, rec := range records {
		fail := rec.Failure
		if fail.PortfolioFailed || rec.FailureSummary.PortfolioFailed {
			if !rec.LegacySummary.OK {
				block.Shadow.BothFailedCount++
			} else {
				block.Shadow.PortfolioFailedCount++
			}
		}
		if !rec.Comparable {
			reason := strings.TrimSpace(rec.IncomparableReason)
			switch reason {
			case providershadow.IncomparableLegacyFailed:
				block.Shadow.LegacyFailedCount++
			case providershadow.IncomparableBothFailed:
				block.Shadow.BothFailedCount++
			case providershadow.IncomparablePortfolioFailed:
				block.Shadow.PortfolioFailedCount++
			}
		}
		code := strings.TrimSpace(fail.ErrorReason)
		if code == "" {
			code = strings.TrimSpace(rec.FailureSummary.ErrorReason)
		}
		if code != "" {
			errHist[code]++
		}
	}
	block.Shadow.ErrorReasonHistogram = topReasonCounts(errHist, maxReasons)

	if hints != nil {
		block.WriteChain.Present = true
		block.WriteChain.BlockedDraftCount = hints.BlockedDraftCount
		block.WriteChain.PortfolioDraftCount = hints.PortfolioDraftCount
		block.WriteChain.LegacyDraftCount = hints.LegacyDraftCount
		if hints.PersistAttempts > 0 {
			block.WriteChain.PersistSuccessRate = float64(hints.PersistSuccesses) / float64(hints.PersistAttempts)
			if math.IsNaN(block.WriteChain.PersistSuccessRate) {
				block.WriteChain.PersistSuccessRate = 0
			}
		}
	}

	if validation != nil && !validation.Skipped && validation.Enabled {
		td := strings.TrimSpace(tradeDate)
		for _, d := range validation.Days {
			if td != "" && strings.TrimSpace(d.TradeDate) != td {
				continue
			}
			if !d.OK {
				block.ValidationDaysNotOK++
			}
		}
	}

	codes := []string{}
	if scope.ScopeExpanded {
		codes = append(codes, "scope_expanded")
	}
	if block.Shadow.PortfolioFailedCount >= 2 {
		codes = append(codes, "shadow_fail_ge_2")
	}
	if lvp.ComparableCount > 0 && lvp.IncomparableCount > lvp.ComparableCount {
		codes = append(codes, "incomparable_majority")
	}
	if hints != nil && hints.BlockedDraftCount > 0 {
		codes = append(codes, "write_blocked")
	}
	block.HumanWatchCodes = codes
	return block
}

func pickHistoryDay(view *portfoliohistory.PortfolioHistoryView, tradeDate string) *portfoliohistory.DailyRecord {
	if view == nil || len(view.Days) == 0 {
		return nil
	}
	td := strings.TrimSpace(tradeDate)
	if td == "" {
		return &view.Days[len(view.Days)-1]
	}
	for i := range view.Days {
		if strings.TrimSpace(view.Days[i].TradeDate) == td {
			return &view.Days[i]
		}
	}
	return nil
}

// ToJSON pretty-prints the report.
func ToJSON(rep *DailyPilotReport) ([]byte, error) {
	if rep == nil {
		return nil, fmt.Errorf("nil report")
	}
	return json.MarshalIndent(rep, "", "  ")
}

// DefaultReportFilename suggests a local JSON filename.
func DefaultReportFilename(tradeDate string) string {
	td := strings.TrimSpace(tradeDate)
	if td == "" {
		td = time.Now().UTC().Format("20060102")
	}
	td = strings.NewReplacer("-", "", "/", "").Replace(td)
	return fmt.Sprintf("DAILY_PILOT_%s.json", td)
}

// WriteReportFile writes the report under dir (creates dir if needed).
func WriteReportFile(dir string, rep *DailyPilotReport) (string, error) {
	raw, err := ToJSON(rep)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := DefaultReportFilename(rep.TradeDate)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
