package portfoliovalidation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/decisionshadowv2"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliosim"
)

// Validate runs the Historical Decision Validation Framework.
// Default Options.Enabled=false → skipped report (read-only / OFF).
func Validate(in Input) *PortfolioValidationReport {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	rep := &PortfolioValidationReport{
		SchemaVersion:      SchemaVersion,
		AsOf:               asOf,
		Enabled:            in.Options.Enabled,
		RecordOnly:         true,
		NotABacktest:       true,
		NotPnL:             true,
		NotSharpe:          true,
		NotAutoTune:        true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotProductionWrite: true,
		ReadOnly:           true,
		Days:               []DayValidation{},
		Summary: AggregateSummary{
			FilterRejectReasonTotals: map[string]int{},
		},
		Notes: []string{
			"historical decision validation · not a backtest",
			"no pnl / sharpe / fill simulation / auto-tune",
			"composed from portfoliosim (+ optional decisionshadowv2)",
		},
	}

	if !in.Options.Enabled {
		rep.Skipped = true
		rep.SkipReason = "enabled=false (default OFF; read-only framework)"
		return rep
	}

	nameDeltaSum := 0.0
	notionalDeltaSum := 0.0
	okN := 0

	for _, day := range in.Days {
		dv := validateDay(day, in.Options)
		rep.Days = append(rep.Days, dv)
		if !dv.OK {
			continue
		}
		okN++
		nameDeltaSum += float64(dv.Difference.NameCountDelta)
		notionalDeltaSum += dv.PortfolioDecision.BuyNotionalSum - dv.LegacyDecision.BuyNotionalSum
		rep.Summary.TightenCountPortfolioTotal += dv.Difference.RiskTightenCountPortfolio
		for code, n := range dv.Difference.FilterRejectReasonsPortfolio {
			rep.Summary.FilterRejectReasonTotals[code] += n
		}
		if dv.Difference.SectorAvailable {
			rep.Summary.SectorComparableDays++
		}
	}

	rep.Summary.DayCount = len(in.Days)
	rep.Summary.OKCount = okN
	if okN > 0 {
		rep.Summary.NameCountDeltaMean = nameDeltaSum / float64(okN)
		rep.Summary.NotionalDeltaMean = notionalDeltaSum / float64(okN)
	}
	return rep
}

// Run is Validate with explicit enabled override (tests / offline runners).
func Run(days []DayCase, enabled bool) *PortfolioValidationReport {
	return Validate(Input{
		Options: Options{Enabled: enabled, AttachDecisionShadowV2: false},
		Days:    days,
	})
}

func validateDay(day DayCase, opt Options) DayValidation {
	out := DayValidation{
		CaseID:    strings.TrimSpace(day.CaseID),
		TradeDate: strings.TrimSpace(day.TradeDate),
	}
	if out.CaseID == "" {
		out.CaseID = out.TradeDate
	}
	if day.Candidates == nil || len(day.Candidates.RankedCandidates) == 0 {
		out.Error = "candidates_required"
		return out
	}

	legacyAmt := day.LegacyAmountPerName
	if legacyAmt <= 0 {
		legacyAmt = DefaultLegacyAmount
	}

	filter := portfoliosim.FilterCompatRequest{SkipCompatibilityCheck: opt.SkipFilterCompat}

	legacyRes := portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:            day.Snapshot,
		Candidates:          day.Candidates,
		Constraints:         day.Constraints,
		Budget:              legacyBasketBudget(day, legacyAmt),
		Filter:              filter,
		DecisionTime:        day.DecisionTime,
		SkipRiskTighten:     true,
		LegacyAmountPerName: legacyAmt,
	})
	portRes := portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:     day.Snapshot,
		Candidates:   day.Candidates,
		Constraints:  day.Constraints,
		Budget:       day.Budget,
		Filter:       filter,
		DecisionTime: day.DecisionTime,
		SkipRiskTighten: false,
	})
	if legacyRes == nil || portRes == nil {
		out.Error = "simulate_nil"
		return out
	}

	industry := mergeIndustry(day)
	legacySide := sideFromSim("fixed_amount", legacyRes, day.Snapshot, industry, legacyAmt, true)
	portSide := sideFromSim("portfolio_allocation", portRes, day.Snapshot, industry, 0, false)

	if opt.AttachDecisionShadowV2 {
		v2 := decisionshadowv2.Run(decisionshadowv2.Input{
			Enabled:                 true,
			AsOf:                    day.DecisionTime,
			TradeDate:               day.TradeDate,
			CandidatePool:           day.Candidates.RankedCandidates,
			Snapshot:                day.Snapshot,
			IndustryBySymbol:        industry,
			Objective:               day.Objective,
			RiskCeilingTemplate:     ceilingOr(day),
			SelectionLimitLegacy:    selectionLimit(day),
			SelectionLimitPortfolio: selectionLimit(day),
			LegacyAmountPerName:     legacyAmt,
			Options: decisionshadowv2.Options{
				Phase:              "shadow_only",
				ProjectPostBuyBook: true,
			},
		}, true)
		if v2 != nil && !v2.Skipped {
			out.DecisionShadowV2Attached = true
			out.DecisionShadowV2FP = v2.InputsFingerprint
			// Prefer V2 sector/cash when available (same-day annex).
			enrichFromV2(&legacySide, &portSide, v2)
		}
	}

	out.LegacyDecision = legacySide
	out.PortfolioDecision = portSide
	out.Difference = diffSides(legacySide, portSide)
	out.OK = true
	return out
}

func ceilingOr(day DayCase) portfoliolayer.ConstraintSet {
	if day.Ceiling.Risk.Enabled || day.Ceiling.Risk.MaxGrossExposurePct != nil || day.Ceiling.Risk.MaxSingleNamePct != nil {
		return day.Ceiling
	}
	return day.Constraints
}

func selectionLimit(day DayCase) int {
	if day.Candidates != nil && day.Candidates.SelectionLimit > 0 {
		return day.Candidates.SelectionLimit
	}
	return 5
}

func legacyBasketBudget(day DayCase, amt float64) *portfoliolayer.AllocationBudget {
	n := selectionLimit(day)
	if day.Candidates != nil {
		ranked := len(day.Candidates.RankedCandidates)
		if ranked > 0 && ranked < n {
			n = ranked
		}
	}
	cap := amt * float64(n)
	return &portfoliolayer.AllocationBudget{
		AvailableCash:    cap,
		AvailableCapital: cap,
		Binding:          "legacy_fixed_amount",
	}
}

func mergeIndustry(day DayCase) map[string]string {
	out := map[string]string{}
	for k, v := range day.IndustryBySymbol {
		sk := strings.ToLower(strings.TrimSpace(k))
		if sk == "" || strings.TrimSpace(v) == "" {
			continue
		}
		out[sk] = strings.TrimSpace(v)
	}
	if day.Snapshot != nil {
		for _, p := range day.Snapshot.Positions {
			sk := strings.ToLower(strings.TrimSpace(p.StockCode))
			ind := strings.TrimSpace(p.Industry)
			if sk == "" || ind == "" {
				continue
			}
			if _, ok := out[sk]; !ok {
				out[sk] = ind
			}
		}
	}
	if day.Candidates != nil {
		for _, c := range day.Candidates.RankedCandidates {
			sk := strings.ToLower(strings.TrimSpace(c.StockCode))
			ind := strings.TrimSpace(c.Industry)
			if sk == "" || ind == "" {
				continue
			}
			if _, ok := out[sk]; !ok {
				out[sk] = ind
			}
		}
	}
	return out
}

func sideFromSim(
	identity string,
	res *portfoliosim.SimulatedPortfolioDecisionResult,
	snap *portfoliolayer.PortfolioSnapshot,
	industry map[string]string,
	legacyAmt float64,
	isLegacy bool,
) SideDecision {
	side := SideDecision{
		ProviderIdentity: identity,
		Lines:            []AmountLine{},
		FilterRejects:    []FilterReject{},
		TightenReasons:   []string{},
	}
	if res == nil {
		return side
	}

	// Lines / name count from allocation set.
	if isLegacy && res.LegacyCompare != nil && res.LegacyCompare.SelectedCount > 0 {
		side.NameCount = res.LegacyCompare.SelectedCount
		side.BuyNotionalSum = res.LegacyCompare.SumAllocationSet
		// Prefer concrete allocation items when present.
	}
	if res.Allocation != nil {
		sum := 0.0
		n := 0
		lines := []AmountLine{}
		for _, it := range res.Allocation.Items {
			sym := strings.ToLower(strings.TrimSpace(it.StockCode))
			role := "waitlist"
			if it.InAllocationSet {
				role = "selected"
				if it.TargetAmount > 0 {
					n++
					sum += it.TargetAmount
				}
			}
			if it.InAllocationSet || it.TargetAmount > 0 {
				lines = append(lines, AmountLine{Symbol: sym, TargetAmount: it.TargetAmount, Role: role})
			}
		}
		if len(lines) > 0 {
			side.Lines = lines
			side.NameCount = n
			side.BuyNotionalSum = sum
		}
	}
	if isLegacy && side.BuyNotionalSum == 0 && legacyAmt > 0 && res.Evaluation.SelectedCount > 0 {
		side.NameCount = res.Evaluation.SelectedCount
		side.BuyNotionalSum = legacyAmt * float64(side.NameCount)
	}
	if !isLegacy && side.BuyNotionalSum == 0 {
		side.NameCount = res.Evaluation.SelectedCount
		side.BuyNotionalSum = res.Evaluation.AllocationSetNotional
	}

	// Cash ratio: projected remaining cash / equity (observation assumption).
	if snap != nil && snap.Found && snap.Equity > 0 {
		remain := snap.Cash - side.BuyNotionalSum
		if remain < 0 {
			remain = 0
		}
		cr := remain / snap.Equity
		side.CashRatio = &cr
	} else {
		side.CashRatioNote = "cash_ratio_unavailable"
	}

	// Sector exposure post-buy projection.
	side.SectorAvailable, side.SectorExposure, side.SectorNote = projectSectors(snap, side.Lines, industry)

	// Tighten
	if res.RiskConstraintTrace != nil {
		side.TightenApplied = res.RiskConstraintTrace.Applied || res.RiskConstraintTrace.HasPatches
		side.TightenNoteCount = len(res.RiskConstraintTrace.Notes)
		for _, n := range res.RiskConstraintTrace.Notes {
			r := strings.TrimSpace(n.Reason)
			if r == "" {
				r = strings.TrimSpace(n.Field)
			}
			if r != "" {
				side.TightenReasons = append(side.TightenReasons, r)
			}
		}
		sort.Strings(side.TightenReasons)
	}

	// Filter rejects
	side.FilterRan = res.PotentialFilter.Ran && res.PotentialFilter.Status == portfoliosim.FilterStatusCalled
	side.FilterRiskStatus = res.PotentialFilter.RiskStatus
	for _, sk := range res.PotentialFilter.Skipped {
		code := strings.TrimSpace(sk.RiskCode)
		if code == "" {
			code = "unknown"
		}
		side.FilterRejects = append(side.FilterRejects, FilterReject{
			Symbol:   strings.ToLower(strings.TrimSpace(sk.StockCode)),
			RiskCode: code,
			Amount:   sk.TargetAmount,
		})
	}
	return side
}

func projectSectors(
	snap *portfoliolayer.PortfolioSnapshot,
	lines []AmountLine,
	industry map[string]string,
) (available bool, weights []SectorWeight, note string) {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return false, nil, "snapshot_unavailable"
	}
	// Require industry coverage for all selected buy lines + existing positions that will remain.
	need := map[string]struct{}{}
	for _, p := range snap.Positions {
		need[strings.ToLower(strings.TrimSpace(p.StockCode))] = struct{}{}
	}
	for _, ln := range lines {
		if ln.Role == "selected" && ln.TargetAmount > 0 {
			need[ln.Symbol] = struct{}{}
		}
	}
	for sym := range need {
		if sym == "" {
			continue
		}
		if strings.TrimSpace(industry[sym]) == "" {
			return false, nil, "industry_incomplete"
		}
	}
	if len(need) == 0 {
		return false, nil, "no_names"
	}

	mv := map[string]float64{}
	for _, p := range snap.Positions {
		sym := strings.ToLower(strings.TrimSpace(p.StockCode))
		mv[sym] += p.MarketValue
	}
	for _, ln := range lines {
		if ln.Role != "selected" || ln.TargetAmount <= 0 {
			continue
		}
		mv[ln.Symbol] += ln.TargetAmount
	}
	agg := map[string]float64{}
	for sym, v := range mv {
		ind := industry[sym]
		agg[ind] += v / snap.Equity
	}
	keys := make([]string, 0, len(agg))
	for k := range agg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]SectorWeight, 0, len(keys))
	for _, k := range keys {
		out = append(out, SectorWeight{Industry: k, Weight: agg[k]})
	}
	return true, out, ""
}

func enrichFromV2(legacy, port *SideDecision, v2 *decisionshadowv2.Report) {
	if v2 == nil {
		return
	}
	if v2.LegacyPlanProjection.BookProjection != nil && v2.LegacyPlanProjection.BookProjection.SectorAvailable {
		legacy.SectorAvailable = true
		legacy.SectorNote = ""
		legacy.SectorExposure = toSectorWeights(v2.LegacyPlanProjection.BookProjection.PostSectorExposure)
		if v2.LegacyPlanProjection.BookProjection.PostCashRatio >= 0 {
			cr := v2.LegacyPlanProjection.BookProjection.PostCashRatio
			legacy.CashRatio = &cr
		}
	}
	if v2.PortfolioPlanProjection.BookProjection != nil && v2.PortfolioPlanProjection.BookProjection.SectorAvailable {
		port.SectorAvailable = true
		port.SectorNote = ""
		port.SectorExposure = toSectorWeights(v2.PortfolioPlanProjection.BookProjection.PostSectorExposure)
		if v2.PortfolioPlanProjection.BookProjection.PostCashRatio >= 0 {
			cr := v2.PortfolioPlanProjection.BookProjection.PostCashRatio
			port.CashRatio = &cr
		}
	}
}

func toSectorWeights(in []decisionshadowv2.SectorExposure) []SectorWeight {
	out := make([]SectorWeight, 0, len(in))
	for _, s := range in {
		out = append(out, SectorWeight{Industry: s.Industry, Weight: s.Weight})
	}
	return out
}

func diffSides(legacy, port SideDecision) DayDifference {
	d := DayDifference{
		NameCountLegacy:              legacy.NameCount,
		NameCountPortfolio:           port.NameCount,
		NameCountDelta:               port.NameCount - legacy.NameCount,
		AmountDiffs:                  []AmountDiffRow{},
		FilterRejectReasonsLegacy:    countRejects(legacy.FilterRejects),
		FilterRejectReasonsPortfolio: countRejects(port.FilterRejects),
		RiskTightenCountLegacy:       legacy.TightenNoteCount,
		RiskTightenCountPortfolio:    port.TightenNoteCount,
		RiskTightenCountDelta:        port.TightenNoteCount - legacy.TightenNoteCount,
	}
	if legacy.TightenApplied && d.RiskTightenCountLegacy == 0 {
		d.RiskTightenCountLegacy = 1
	}
	if port.TightenApplied && d.RiskTightenCountPortfolio == 0 {
		d.RiskTightenCountPortfolio = 1
		d.RiskTightenCountDelta = d.RiskTightenCountPortfolio - d.RiskTightenCountLegacy
	}

	syms := map[string]struct{}{}
	legAmt := map[string]float64{}
	portAmt := map[string]float64{}
	for _, ln := range legacy.Lines {
		if ln.Role == "waitlist" {
			continue
		}
		syms[ln.Symbol] = struct{}{}
		legAmt[ln.Symbol] = ln.TargetAmount
	}
	for _, ln := range port.Lines {
		if ln.Role == "waitlist" {
			continue
		}
		syms[ln.Symbol] = struct{}{}
		portAmt[ln.Symbol] = ln.TargetAmount
	}
	keys := make([]string, 0, len(syms))
	for s := range syms {
		keys = append(keys, s)
	}
	sort.Strings(keys)
	for _, s := range keys {
		d.AmountDiffs = append(d.AmountDiffs, AmountDiffRow{
			Symbol: s, LegacyAmount: legAmt[s], PortfolioAmount: portAmt[s],
			Delta: portAmt[s] - legAmt[s],
		})
	}

	if legacy.SectorAvailable && port.SectorAvailable {
		d.SectorAvailable = true
		d.SectorLegacy = legacy.SectorExposure
		d.SectorPortfolio = port.SectorExposure
	} else {
		d.SectorAvailable = false
		note := "sector_unavailable"
		if legacy.SectorNote != "" {
			note = legacy.SectorNote
		}
		if port.SectorNote != "" && port.SectorNote != legacy.SectorNote {
			note = fmt.Sprintf("%s; portfolio:%s", note, port.SectorNote)
		}
		d.SectorUnavailableNote = note
	}

	d.CashRatioLegacy = legacy.CashRatio
	d.CashRatioPortfolio = port.CashRatio
	if legacy.CashRatio != nil && port.CashRatio != nil {
		delta := *port.CashRatio - *legacy.CashRatio
		d.CashRatioDelta = &delta
	}
	return d
}

func countRejects(rs []FilterReject) map[string]int {
	out := map[string]int{}
	for _, r := range rs {
		out[r.RiskCode]++
	}
	return out
}
