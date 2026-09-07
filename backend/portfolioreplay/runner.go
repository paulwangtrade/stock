package portfolioreplay

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/portfoliosim"
)

// Run replays cases with Legacy Simulation + Portfolio Simulation, then aggregates
// PortfolioReplayReport decision-behavior stats. No DB writes, no TradePlans, no returns.
func Run(cases []ReplayCase) *PortfolioReplayReport {
	rep := &PortfolioReplayReport{
		RecordOnly:    true,
		NotATradePlan: true,
		NotABacktest:  true,
		CaseCount:     len(cases),
		Errors:        []ReplayError{},
		Rows:          []ReplayRow{},
		PotentialFilterSummary: PotentialFilterSummary{
			RiskStatus:    map[string]int{},
			RejectReasons: map[string]int{},
		},
		DecisionBehavior: DecisionBehaviorStats{
			RiskConstraintHits: map[string]int{},
			SectorExposure: SectorExposureStats{
				Available: false,
				Note:      "sector_unavailable_no_industry",
			},
		},
	}
	selectedHist := map[int]int{}
	var notionals []float64
	var pairs []casePairMetrics
	allDet := true

	for _, c := range cases {
		portA := simulatePortfolio(c)
		portB := simulatePortfolio(c)
		legacyA := simulateLegacy(c)
		legacyB := simulateLegacy(c)

		if portA == nil || portB == nil || legacyA == nil || legacyB == nil {
			allDet = false
			rep.Errors = append(rep.Errors, ReplayError{CaseID: c.CaseID, Reason: "simulate_nil"})
			continue
		}
		if !portA.NotATradePlan || !legacyA.NotATradePlan {
			allDet = false
			rep.Errors = append(rep.Errors, ReplayError{CaseID: c.CaseID, Reason: "not_a_trade_plan_false"})
			continue
		}
		if fingerprint(portA) != fingerprint(portB) || fingerprint(legacyA) != fingerprint(legacyB) {
			allDet = false
			rep.Errors = append(rep.Errors, ReplayError{CaseID: c.CaseID, Reason: "nondeterministic"})
			continue
		}

		rep.SuccessCount++
		rep.DecisionCount++
		n := portA.Evaluation.SelectedCount
		selectedHist[n]++
		notionals = append(notionals, portA.Evaluation.AllocationSetNotional)
		accumulateFilter(&rep.PotentialFilterSummary, portA.PotentialFilter)

		pair := collectPairMetrics(c, legacyA, portA)
		pairs = append(pairs, pair)
		rep.Rows = append(rep.Rows, pair.row)
	}

	rep.DeterministicCheck = allDet && rep.SuccessCount == rep.CaseCount && len(rep.Errors) == 0
	rep.SelectedCountDistribution = binsFrom(selectedHist)
	rep.AllocationDistribution = allocationDist(notionals)
	rep.DecisionBehavior = aggregateBehavior(pairs)
	return rep
}

func simulatePortfolio(c ReplayCase) *portfoliosim.SimulatedPortfolioDecisionResult {
	return portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:     c.Snapshot,
		Candidates:   c.Candidates,
		Constraints:  c.Constraints,
		Budget:       c.Budget,
		Filter:       c.Filter,
		DecisionTime: c.DecisionTime,
	})
}

func simulateLegacy(c ReplayCase) *portfoliosim.SimulatedPortfolioDecisionResult {
	// Legacy fixed_amount projection: SkipRiskTighten + injected capital N×100000 → equal-weight ≈ 100000.
	amt := defaultLegacyReplayAmount
	return portfoliosim.Simulate(portfoliosim.Input{
		Snapshot:            c.Snapshot,
		Candidates:          c.Candidates,
		Constraints:         c.Constraints,
		Budget:              legacyBasketBudget(c, amt),
		Filter:              c.Filter,
		DecisionTime:        c.DecisionTime,
		SkipRiskTighten:     true,
		LegacyAmountPerName: amt,
	})
}

func fingerprint(r *portfoliosim.SimulatedPortfolioDecisionResult) string {
	type line struct {
		Code   string
		Amount float64
		Status string
		Risk   string
	}
	type fp struct {
		Selected []string
		Waitlist []string
		Rejected []string
		Alloc    []line
		Pending  []line
		Skipped  []line
		Uniform  float64
		RiskStat string
	}
	out := fp{RiskStat: r.PotentialFilter.RiskStatus}
	for _, p := range r.Selected {
		out.Selected = append(out.Selected, p.Candidate.StockCode)
	}
	for _, p := range r.Waitlist {
		out.Waitlist = append(out.Waitlist, p.Candidate.StockCode)
	}
	for _, p := range r.Rejected {
		out.Rejected = append(out.Rejected, p.Candidate.StockCode)
	}
	if r.Allocation != nil {
		out.Uniform = r.Allocation.UniformAmount
		for _, it := range r.Allocation.Items {
			out.Alloc = append(out.Alloc, line{Code: it.StockCode, Amount: it.TargetAmount})
		}
	}
	for _, l := range r.PotentialFilter.Pending {
		out.Pending = append(out.Pending, line{Code: l.StockCode, Amount: l.TargetAmount, Status: l.Status})
	}
	for _, l := range r.PotentialFilter.Skipped {
		out.Skipped = append(out.Skipped, line{Code: l.StockCode, Amount: l.TargetAmount, Status: l.Status, Risk: l.RiskCode})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func accumulateFilter(sum *PotentialFilterSummary, f portfoliosim.SimulatedFilterResult) {
	if !f.Ran || f.Status == portfoliosim.FilterStatusNotRun {
		sum.NotRunCount++
		return
	}
	sum.CalledCount++
	sum.PendingTotal += len(f.Pending)
	sum.SkippedTotal += len(f.Skipped)
	if f.RiskStatus != "" {
		sum.RiskStatus[f.RiskStatus]++
	}
	for _, l := range f.Skipped {
		reason := strings.TrimSpace(l.RiskCode)
		if reason == "" {
			reason = "unknown"
		}
		sum.RejectReasons[reason]++
	}
}

func binsFrom(h map[int]int) []CountBin {
	keys := make([]int, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	out := make([]CountBin, 0, len(keys))
	for _, k := range keys {
		out = append(out, CountBin{Value: k, Count: h[k]})
	}
	return out
}

func allocationDist(vals []float64) AllocationDistribution {
	d := AllocationDistribution{Values: vals}
	if len(vals) == 0 {
		return d
	}
	d.Min, d.Max = vals[0], vals[0]
	sum := 0.0
	for _, v := range vals {
		if v < d.Min {
			d.Min = v
		}
		if v > d.Max {
			d.Max = v
		}
		sum += v
	}
	d.Mean = sum / float64(len(vals))
	return d
}

func (r *PortfolioReplayReport) String() string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("replay cases=%d success=%d det=%v", r.CaseCount, r.SuccessCount, r.DeterministicCheck)
}
