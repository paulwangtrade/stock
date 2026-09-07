package portfolioreplay

import (
	"math"
	"strings"

	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliosim"
)

const defaultLegacyReplayAmount = 100_000.0

// ScalarDeltaStats aggregates port − legacy deltas across successful cases.
type ScalarDeltaStats struct {
	Values []float64 `json:"values"`
	Min    float64   `json:"min"`
	Max    float64   `json:"max"`
	Mean   float64   `json:"mean"`
	Sum    float64   `json:"sum"`
}

// CandidateCountStats is Legacy vs Portfolio selected/waitlist counts.
type CandidateCountStats struct {
	LegacySelectedMean    float64         `json:"legacy_selected_mean"`
	PortfolioSelectedMean float64         `json:"portfolio_selected_mean"`
	SelectedCountDelta    ScalarDeltaStats `json:"selected_count_delta"` // port − legacy
	LegacyWaitlistMean    float64         `json:"legacy_waitlist_mean"`
	PortfolioWaitlistMean float64         `json:"portfolio_waitlist_mean"`
	WaitlistCountDelta    ScalarDeltaStats `json:"waitlist_count_delta"`
}

// AllocationAmountStats compares allocation-set notionals.
type AllocationAmountStats struct {
	LegacyNotionalMean    float64         `json:"legacy_notional_mean"`
	PortfolioNotionalMean float64         `json:"portfolio_notional_mean"`
	NotionalDelta         ScalarDeltaStats `json:"notional_delta"` // port − legacy
}

// CashUsageStats compares cash occupancy (allocation_set_notional and optional / cash).
type CashUsageStats struct {
	LegacyNotionalMean      float64         `json:"legacy_notional_mean"`
	PortfolioNotionalMean   float64         `json:"portfolio_notional_mean"`
	NotionalDelta           ScalarDeltaStats `json:"notional_delta"`
	LegacyCashRatioMean     float64         `json:"legacy_cash_ratio_mean"`
	PortfolioCashRatioMean  float64         `json:"portfolio_cash_ratio_mean"`
	CashRatioDelta          ScalarDeltaStats `json:"cash_ratio_delta"`
	PortfolioReserveMean    float64         `json:"portfolio_reserve_mean"`
}

// ConcentrationStats compares projected top-1 weight after allocation-set envelopes.
type ConcentrationStats struct {
	LegacyTop1Mean    float64         `json:"legacy_top1_mean"`
	PortfolioTop1Mean float64         `json:"portfolio_top1_mean"`
	Top1Delta         ScalarDeltaStats `json:"top1_delta"` // port − legacy
}

// SectorExposureStats is only filled when industry is available on the snapshot.
type SectorExposureStats struct {
	Available           bool            `json:"available"`
	Note                string          `json:"note,omitempty"`
	LegacyMaxSectorMean float64         `json:"legacy_max_sector_mean,omitempty"`
	PortfolioMaxSectorMean float64      `json:"portfolio_max_sector_mean,omitempty"`
	MaxSectorDelta      ScalarDeltaStats `json:"max_sector_delta,omitempty"`
}

// DecisionBehaviorStats evaluates decision behavior only (not returns / backtest).
type DecisionBehaviorStats struct {
	CandidateCounts   CandidateCountStats   `json:"candidate_counts"`
	AllocationAmounts AllocationAmountStats `json:"allocation_amounts"`
	CashUsage         CashUsageStats        `json:"cash_usage"`
	RiskConstraintHits map[string]int       `json:"risk_constraint_hits"`
	Concentration     ConcentrationStats    `json:"concentration"`
	SectorExposure    SectorExposureStats   `json:"sector_exposure"`
}

// ReplayRow is one case dual-sim summary for spot checks (not a PnL curve).
type ReplayRow struct {
	CaseID                 string  `json:"case_id"`
	TradeDate              string  `json:"trade_date,omitempty"`
	LegacySelectedCount    int     `json:"legacy_selected_count"`
	PortfolioSelectedCount int     `json:"portfolio_selected_count"`
	LegacyNotional         float64 `json:"legacy_notional"`
	PortfolioNotional      float64 `json:"portfolio_notional"`
	LegacyTop1Weight       float64 `json:"legacy_top1_weight"`
	PortfolioTop1Weight    float64 `json:"portfolio_top1_weight"`
	SectorAvailable        bool    `json:"sector_available"`
}

type casePairMetrics struct {
	legacySelected, portSelected int
	legacyWaitlist, portWaitlist int
	legacyNotional, portNotional float64
	legacyCashRatio, portCashRatio float64
	portReserve                  float64
	legacyTop1, portTop1         float64
	sectorAvailable              bool
	legacyMaxSector, portMaxSector float64
	riskHits                     map[string]int
	row                          ReplayRow
}

func collectPairMetrics(c ReplayCase, legacy, port *portfoliosim.SimulatedPortfolioDecisionResult) casePairMetrics {
	m := casePairMetrics{riskHits: map[string]int{}}
	if legacy != nil {
		m.legacySelected = legacy.Evaluation.SelectedCount
		m.legacyWaitlist = legacy.Evaluation.WaitlistCount
		m.legacyNotional = legacy.Evaluation.AllocationSetNotional
		if m.legacyNotional == 0 && legacy.LegacyCompare != nil {
			m.legacyNotional = legacy.LegacyCompare.SumAllocationSet
			m.legacySelected = legacy.LegacyCompare.SelectedCount
		}
	}
	if port != nil {
		m.portSelected = port.Evaluation.SelectedCount
		m.portWaitlist = port.Evaluation.WaitlistCount
		m.portNotional = port.Evaluation.AllocationSetNotional
		m.portReserve = port.Evaluation.ReserveCash
	}
	cash := 0.0
	if c.Snapshot != nil && c.Snapshot.Cash > 0 {
		cash = c.Snapshot.Cash
		m.legacyCashRatio = m.legacyNotional / cash
		m.portCashRatio = m.portNotional / cash
	}
	m.legacyTop1 = projectTop1Weight(c.Snapshot, legacy)
	m.portTop1 = projectTop1Weight(c.Snapshot, port)
	m.sectorAvailable, m.legacyMaxSector, m.portMaxSector = projectMaxSector(c.Snapshot, legacy, port)
	accumulateRiskHits(m.riskHits, port)
	m.row = ReplayRow{
		CaseID:                 c.CaseID,
		TradeDate:              c.TradeDate,
		LegacySelectedCount:    m.legacySelected,
		PortfolioSelectedCount: m.portSelected,
		LegacyNotional:         m.legacyNotional,
		PortfolioNotional:      m.portNotional,
		LegacyTop1Weight:       m.legacyTop1,
		PortfolioTop1Weight:    m.portTop1,
		SectorAvailable:        m.sectorAvailable,
	}
	return m
}

func accumulateRiskHits(dst map[string]int, port *portfoliosim.SimulatedPortfolioDecisionResult) {
	if port == nil {
		return
	}
	for _, r := range port.Rejected {
		reason := strings.TrimSpace(strings.ToLower(r.Reason))
		if reason == "" {
			reason = "rejected_unknown"
		}
		dst[reason]++
	}
	if port.RiskConstraintTrace != nil {
		for _, n := range port.RiskConstraintTrace.Notes {
			key := strings.TrimSpace(n.Reason)
			if key == "" {
				key = strings.TrimSpace(n.Field)
			}
			if key == "" {
				continue
			}
			dst["tighten:"+key]++
		}
	}
	for _, t := range port.ResolvedConstraints.Trace {
		note := strings.TrimSpace(t.Note)
		if note == "ceiling" || note == "tightened_within_ceiling" || note == "default" {
			dst["trace:"+t.Field+":"+note]++
		}
	}
	for _, sk := range port.PotentialFilter.Skipped {
		code := strings.TrimSpace(sk.RiskCode)
		if code == "" {
			continue
		}
		dst["filter:"+code]++
	}
}

func projectTop1Weight(snap *portfoliolayer.PortfolioSnapshot, res *portfoliosim.SimulatedPortfolioDecisionResult) float64 {
	if snap == nil || !snap.Found || snap.Equity <= 0 || res == nil {
		return 0
	}
	mv := map[string]float64{}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" {
			continue
		}
		mv[code] += p.MarketValue
	}
	if res.Allocation != nil {
		for _, it := range res.Allocation.Items {
			if !it.InAllocationSet || it.TargetAmount <= 0 {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(it.StockCode))
			mv[code] += it.TargetAmount
		}
	}
	top := 0.0
	for _, v := range mv {
		w := v / snap.Equity
		if w > top {
			top = w
		}
	}
	return top
}

func projectMaxSector(snap *portfoliolayer.PortfolioSnapshot, legacy, port *portfoliosim.SimulatedPortfolioDecisionResult) (available bool, legacyMax, portMax float64) {
	if snap == nil || !snap.Found || snap.Equity <= 0 {
		return false, 0, 0
	}
	hasIndustry := false
	for _, p := range snap.Positions {
		if strings.TrimSpace(p.Industry) != "" {
			hasIndustry = true
			break
		}
	}
	if !hasIndustry {
		return false, 0, 0
	}
	return true, maxSectorWeight(snap, legacy), maxSectorWeight(snap, port)
}

func maxSectorWeight(snap *portfoliolayer.PortfolioSnapshot, res *portfoliosim.SimulatedPortfolioDecisionResult) float64 {
	if snap == nil || snap.Equity <= 0 {
		return 0
	}
	industryOf := map[string]string{}
	secMV := map[string]float64{}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		ind := strings.TrimSpace(p.Industry)
		if ind == "" {
			ind = "unknown"
		}
		industryOf[code] = ind
		secMV[ind] += p.MarketValue
	}
	if res != nil && res.Allocation != nil {
		for _, it := range res.Allocation.Items {
			if !it.InAllocationSet || it.TargetAmount <= 0 {
				continue
			}
			code := strings.ToLower(strings.TrimSpace(it.StockCode))
			ind := industryOf[code]
			if ind == "" {
				ind = "unknown"
			}
			secMV[ind] += it.TargetAmount
		}
	}
	top := 0.0
	for _, v := range secMV {
		w := v / snap.Equity
		if w > top {
			top = w
		}
	}
	return top
}

func aggregateBehavior(pairs []casePairMetrics) DecisionBehaviorStats {
	out := DecisionBehaviorStats{
		RiskConstraintHits: map[string]int{},
		SectorExposure: SectorExposureStats{
			Available: false,
			Note:      "sector_unavailable_no_industry",
		},
	}
	if len(pairs) == 0 {
		return out
	}
	var (
		legSel, portSel, legWait, portWait []float64
		legNot, portNot, notDelta          []float64
		legRatio, portRatio, ratioDelta    []float64
		reserves                           []float64
		legTop, portTop, topDelta          []float64
		legSec, portSec, secDelta          []float64
		sectorOK                           bool
	)
	for _, p := range pairs {
		legSel = append(legSel, float64(p.legacySelected))
		portSel = append(portSel, float64(p.portSelected))
		legWait = append(legWait, float64(p.legacyWaitlist))
		portWait = append(portWait, float64(p.portWaitlist))
		legNot = append(legNot, p.legacyNotional)
		portNot = append(portNot, p.portNotional)
		notDelta = append(notDelta, p.portNotional-p.legacyNotional)
		legRatio = append(legRatio, p.legacyCashRatio)
		portRatio = append(portRatio, p.portCashRatio)
		ratioDelta = append(ratioDelta, p.portCashRatio-p.legacyCashRatio)
		reserves = append(reserves, p.portReserve)
		legTop = append(legTop, p.legacyTop1)
		portTop = append(portTop, p.portTop1)
		topDelta = append(topDelta, p.portTop1-p.legacyTop1)
		if p.sectorAvailable {
			sectorOK = true
			legSec = append(legSec, p.legacyMaxSector)
			portSec = append(portSec, p.portMaxSector)
			secDelta = append(secDelta, p.portMaxSector-p.legacyMaxSector)
		}
		for k, v := range p.riskHits {
			out.RiskConstraintHits[k] += v
		}
	}
	out.CandidateCounts = CandidateCountStats{
		LegacySelectedMean:    meanOf(legSel),
		PortfolioSelectedMean: meanOf(portSel),
		SelectedCountDelta:    deltaStats(diffSlices(portSel, legSel)),
		LegacyWaitlistMean:    meanOf(legWait),
		PortfolioWaitlistMean: meanOf(portWait),
		WaitlistCountDelta:    deltaStats(diffSlices(portWait, legWait)),
	}
	out.AllocationAmounts = AllocationAmountStats{
		LegacyNotionalMean:    meanOf(legNot),
		PortfolioNotionalMean: meanOf(portNot),
		NotionalDelta:         deltaStats(notDelta),
	}
	out.CashUsage = CashUsageStats{
		LegacyNotionalMean:     meanOf(legNot),
		PortfolioNotionalMean:  meanOf(portNot),
		NotionalDelta:          deltaStats(notDelta),
		LegacyCashRatioMean:    meanOf(legRatio),
		PortfolioCashRatioMean: meanOf(portRatio),
		CashRatioDelta:         deltaStats(ratioDelta),
		PortfolioReserveMean:   meanOf(reserves),
	}
	out.Concentration = ConcentrationStats{
		LegacyTop1Mean:    meanOf(legTop),
		PortfolioTop1Mean: meanOf(portTop),
		Top1Delta:         deltaStats(topDelta),
	}
	if sectorOK {
		out.SectorExposure = SectorExposureStats{
			Available:              true,
			LegacyMaxSectorMean:    meanOf(legSec),
			PortfolioMaxSectorMean: meanOf(portSec),
			MaxSectorDelta:         deltaStats(secDelta),
		}
	}
	return out
}

func diffSlices(a, b []float64) []float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = a[i] - b[i]
	}
	return out
}

func meanOf(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func deltaStats(vals []float64) ScalarDeltaStats {
	d := ScalarDeltaStats{Values: append([]float64(nil), vals...)}
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
	d.Sum = sum
	d.Mean = sum / float64(len(vals))
	return d
}

func legacyBasketBudget(c ReplayCase, amount float64) *portfoliolayer.AllocationBudget {
	if amount <= 0 || math.IsNaN(amount) {
		amount = defaultLegacyReplayAmount
	}
	n := 0
	if c.Candidates != nil {
		n = c.Candidates.SelectionLimit
		if n <= 0 {
			n = len(c.Candidates.RankedCandidates)
		}
		if n > len(c.Candidates.RankedCandidates) {
			n = len(c.Candidates.RankedCandidates)
		}
	}
	if n <= 0 {
		n = 1
	}
	return &portfoliolayer.AllocationBudget{
		AvailableCapital: amount * float64(n),
		Binding:          "legacy_fixed_projection",
	}
}
