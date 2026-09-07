package portfoliolayer

import (
	"sort"
	"strings"

	"go-stock/backend/selection"
)

const (
	EvaluationStatusEvaluated = "evaluated"

	SectorNoteUnavailable = "sector_model_unavailable_no_industry"
	SectorNoteAvailable   = "sector_model_from_candidate_or_position_industry"
)

// CandidateEvaluation compares intended buy-set and waitlist shape. Record only.
type CandidateEvaluation struct {
	CandidateCountDelta int `json:"candidate_count_delta"`
	SelectedChangeCount int `json:"selected_change_count"`
	WaitlistCountDelta  int `json:"waitlist_count_delta"`
	WaitlistChangeCount int `json:"waitlist_change_count"`
}

// AllocationEvaluation compares planned envelopes and cash/reserve. Record only.
type AllocationEvaluation struct {
	AmountDelta       float64 `json:"amount_delta"`
	CashUsageDelta    float64 `json:"cash_usage_delta"`
	ReserveDelta      float64 `json:"reserve_delta"`
	LegacyBuyNotional float64 `json:"legacy_buy_notional"`
	ShadowBuyNotional float64 `json:"shadow_buy_notional"`
	LegacyReserve     float64 `json:"legacy_reserve"`
	ShadowReserve     float64 `json:"shadow_reserve"`
}

// PortfolioEvaluation projects book-shape deltas from envelopes. Not a live Snapshot write.
type PortfolioEvaluation struct {
	PositionCountDelta    int     `json:"position_count_delta"`
	LegacyProjectedNames  int     `json:"legacy_projected_new_names"`
	ShadowProjectedNames  int     `json:"shadow_projected_new_names"`
	ConcentrationDelta    float64 `json:"concentration_delta"`
	LegacyMaxNameWeight   float64 `json:"legacy_max_name_weight"`
	ShadowMaxNameWeight   float64 `json:"shadow_max_name_weight"`
	SectorExposureDelta   float64 `json:"sector_exposure_delta"`
	LegacyMaxSectorWeight float64 `json:"legacy_max_sector_weight"`
	ShadowMaxSectorWeight float64 `json:"shadow_max_sector_weight"`
	SectorModelAvailable  bool    `json:"sector_model_available"`
	SectorNote            string  `json:"sector_note,omitempty"`
}

// ConstraintImpact is a hit restated as an effect label. Not a trade instruction.
type ConstraintImpact struct {
	Field    string `json:"field"`
	Consumer string `json:"consumer"`
	Reason   string `json:"reason"`
	Source   string `json:"source,omitempty"`
	Count    int    `json:"count"`
	Effect   string `json:"effect"`
}

// ReasonCount is a stable selection-reason tally.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// ExplainabilityEvaluation summarizes why the two paths diverged.
type ExplainabilityEvaluation struct {
	ConstraintImpact       []ConstraintImpact `json:"constraint_impact"`
	SelectionReasonSummary []ReasonCount      `json:"selection_reason_summary"`
}

// ShadowEvaluationReport scores nothing and recommends nothing. Deltas only.
type ShadowEvaluationReport struct {
	Status         string                   `json:"status"`
	RecordOnly     bool                     `json:"record_only"`
	Candidate      CandidateEvaluation      `json:"candidate"`
	Allocation     AllocationEvaluation     `json:"allocation"`
	Portfolio      PortfolioEvaluation      `json:"portfolio"`
	Explainability ExplainabilityEvaluation `json:"explainability"`
}

// EvaluateShadow derives F.6 metrics from an F.5 observation. Nil in → nil out.
// Does not call PlanFilter, TradePlan, Sizer, Execution, or materialization.
func EvaluateShadow(in DecisionInput, report *PortfolioShadowReport) *ShadowEvaluationReport {
	if report == nil {
		return nil
	}
	out := &ShadowEvaluationReport{
		Status:     EvaluationStatusEvaluated,
		RecordOnly: true,
	}

	legacyBasket := len(report.Legacy.Basket)
	legacyWait := len(report.Legacy.Waitlist)
	selN, waitN := 0, 0
	if report.Selection != nil {
		selN = len(report.Selection.Selected)
		waitN = len(report.Selection.Waitlist)
	}
	buyFlips, waitFlips := 0, 0
	for _, d := range report.CandidateDiffs {
		if isBuyRole(d.LegacyRole) != isBuyRole(d.PortfolioRole) {
			buyFlips++
		}
		if isWaitRole(d.LegacyRole) != isWaitRole(d.PortfolioRole) {
			waitFlips++
		}
	}
	out.Candidate = CandidateEvaluation{
		CandidateCountDelta: selN - legacyBasket,
		SelectedChangeCount: buyFlips,
		WaitlistCountDelta:  waitN - legacyWait,
		WaitlistChangeCount: waitFlips,
	}

	legacyNotional := report.Legacy.AmountPerName * float64(legacyBasket)
	shadowNotional := 0.0
	shadowReserve := 0.0
	if report.Allocation != nil {
		for _, item := range report.Allocation.Items {
			if item.InAllocationSet {
				shadowNotional += item.TargetAmount
			}
		}
		shadowReserve = report.Allocation.Budget.ReserveCash
	}
	legacyReserve := 0.0 // FixedAmount path does not apply a generate-time reserve
	out.Allocation = AllocationEvaluation{
		AmountDelta:       shadowNotional - legacyNotional,
		CashUsageDelta:    shadowNotional - legacyNotional,
		ReserveDelta:      shadowReserve - legacyReserve,
		LegacyBuyNotional: legacyNotional,
		ShadowBuyNotional: shadowNotional,
		LegacyReserve:     legacyReserve,
		ShadowReserve:     shadowReserve,
	}

	held := in.Ledger.HoldingCodes()
	legacyNew := countNewNames(candidateViews(report.Legacy.Basket), held)
	shadowNew := 0
	if report.Selection != nil {
		shadowNew = countNewNames(picksToCandidates(report.Selection.Selected), held)
	}
	legacyBuys := buyAmountMapLegacy(report)
	shadowBuys := buyAmountMapShadow(report)
	equity := 0.0
	if isUsableSnapshot(in.Ledger) {
		equity = in.Ledger.Equity
	}
	legacyMaxName := maxNameWeight(in.Ledger, legacyBuys, equity)
	shadowMaxName := maxNameWeight(in.Ledger, shadowBuys, equity)
	industryBy := industryIndex(in)
	sectorOK := len(industryBy) > 0
	legacyMaxSec, shadowMaxSec := 0.0, 0.0
	note := SectorNoteUnavailable
	if sectorOK {
		note = SectorNoteAvailable
		legacyMaxSec = maxSectorWeight(in.Ledger, legacyBuys, industryBy, equity)
		shadowMaxSec = maxSectorWeight(in.Ledger, shadowBuys, industryBy, equity)
	}
	out.Portfolio = PortfolioEvaluation{
		PositionCountDelta:    shadowNew - legacyNew,
		LegacyProjectedNames:  legacyNew,
		ShadowProjectedNames:  shadowNew,
		ConcentrationDelta:    shadowMaxName - legacyMaxName,
		LegacyMaxNameWeight:   legacyMaxName,
		ShadowMaxNameWeight:   shadowMaxName,
		SectorExposureDelta:   shadowMaxSec - legacyMaxSec,
		LegacyMaxSectorWeight: legacyMaxSec,
		ShadowMaxSectorWeight: shadowMaxSec,
		SectorModelAvailable:  sectorOK,
		SectorNote:            note,
	}

	out.Explainability = ExplainabilityEvaluation{
		ConstraintImpact:       constraintImpacts(report.ConstraintHits),
		SelectionReasonSummary: selectionReasonSummary(report.Selection),
	}
	return out
}

func isBuyRole(role string) bool {
	return role == LegacyRoleBasket || role == PortfolioRoleSelected
}

func isWaitRole(role string) bool {
	return role == LegacyRoleWaitlist || role == PortfolioRoleWaitlist
}

func countNewNames(cands []selectionCandidateView, held map[string]struct{}) int {
	n := 0
	seen := map[string]struct{}{}
	for _, c := range cands {
		k := normCode(c.StockCode)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		if _, ok := held[k]; ok {
			continue
		}
		n++
	}
	return n
}

type selectionCandidateView struct {
	StockCode string
	Industry  string
}

func picksToCandidates(picks []PortfolioPick) []selectionCandidateView {
	out := make([]selectionCandidateView, len(picks))
	for i, p := range picks {
		out[i] = selectionCandidateView{StockCode: p.Candidate.StockCode, Industry: p.Candidate.Industry}
	}
	return out
}

func candidateViews(cands []selection.Candidate) []selectionCandidateView {
	out := make([]selectionCandidateView, len(cands))
	for i, c := range cands {
		out[i] = selectionCandidateView{StockCode: c.StockCode, Industry: c.Industry}
	}
	return out
}

func buyAmountMapLegacy(report *PortfolioShadowReport) map[string]float64 {
	out := map[string]float64{}
	if report == nil {
		return out
	}
	amt := report.Legacy.AmountPerName
	for _, c := range report.Legacy.Basket {
		out[normCode(c.StockCode)] = amt
	}
	return out
}

func buyAmountMapShadow(report *PortfolioShadowReport) map[string]float64 {
	out := map[string]float64{}
	if report == nil || report.Allocation == nil {
		return out
	}
	for _, item := range report.Allocation.Items {
		if !item.InAllocationSet {
			continue
		}
		out[normCode(item.StockCode)] = item.TargetAmount
	}
	return out
}

func maxNameWeight(snap *PortfolioSnapshot, buys map[string]float64, equity float64) float64 {
	if equity <= 0 {
		return 0
	}
	mv := map[string]float64{}
	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			k := normCode(p.StockCode)
			if k == "" {
				continue
			}
			mv[k] = p.MarketValue
		}
	}
	for code, amt := range buys {
		mv[code] += amt
	}
	maxW := 0.0
	for _, v := range mv {
		w := v / equity
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func industryIndex(in DecisionInput) map[string]string {
	out := map[string]string{}
	if in.Ledger != nil {
		for _, p := range in.Ledger.Positions {
			ind := strings.TrimSpace(p.Industry)
			if ind == "" {
				continue
			}
			k := normCode(p.StockCode)
			if k != "" {
				out[k] = ind
			}
		}
	}
	for _, c := range in.RankedCandidates {
		ind := strings.TrimSpace(c.Industry)
		if ind == "" {
			continue
		}
		k := normCode(c.StockCode)
		if k != "" {
			out[k] = ind
		}
	}
	return out
}

func maxSectorWeight(snap *PortfolioSnapshot, buys map[string]float64, industryBy map[string]string, equity float64) float64 {
	if equity <= 0 {
		return 0
	}
	secMV := map[string]float64{}
	if snap != nil && snap.Found {
		for _, p := range snap.Positions {
			ind := strings.TrimSpace(p.Industry)
			if ind == "" {
				ind = industryBy[normCode(p.StockCode)]
			}
			if ind == "" {
				continue
			}
			secMV[ind] += p.MarketValue
		}
	}
	for code, amt := range buys {
		ind := industryBy[code]
		if ind == "" {
			continue
		}
		secMV[ind] += amt
	}
	maxW := 0.0
	for _, v := range secMV {
		w := v / equity
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func constraintImpacts(hits []ConstraintHit) []ConstraintImpact {
	out := make([]ConstraintImpact, 0, len(hits))
	for _, h := range hits {
		out = append(out, ConstraintImpact{
			Field:    h.Field,
			Consumer: h.Consumer,
			Reason:   h.Reason,
			Source:   h.Source,
			Count:    h.Count,
			Effect:   constraintEffect(h.Reason),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Consumer != out[j].Consumer {
			return out[i].Consumer < out[j].Consumer
		}
		if out[i].Reason != out[j].Reason {
			return out[i].Reason < out[j].Reason
		}
		return out[i].Field < out[j].Field
	})
	return out
}

func constraintEffect(reason string) string {
	switch reason {
	case ReasonNameLimit, "candidate_diff":
		return "reduced_or_moved_selected"
	case ReasonAlreadyHolding:
		return "removed_from_buy_set"
	case ReasonSectorLimit:
		return "diverted_to_waitlist"
	case ReasonNoSnapshot:
		return "blocked_allocation_set"
	case AllocReasonBelowMinOrder:
		return "zeroed_amount"
	case AllocReasonCappedSingleWeight:
		return "capped_name_weight"
	case "tightened_within_ceiling":
		return "preference_tightened_within_risk"
	case "ceiling":
		return "risk_ceiling_applied"
	case "tightest":
		return "preference_tightest"
	case "stricter_true":
		return "boolean_tightened"
	default:
		return "recorded"
	}
}

func selectionReasonSummary(sel *PortfolioSelectionResult) []ReasonCount {
	counts := map[string]int{}
	add := func(reason string) {
		if reason == "" {
			return
		}
		counts[reason]++
	}
	if sel != nil {
		for _, p := range sel.Selected {
			add(p.Reason)
		}
		for _, p := range sel.Waitlist {
			add(p.Reason)
		}
		for _, p := range sel.Rejected {
			add(p.Reason)
		}
	}
	out := make([]ReasonCount, 0, len(counts))
	for r, n := range counts {
		out = append(out, ReasonCount{Reason: r, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}
