package selection

import (
	"fmt"
	"testing"
)

func TestSelect_ThirtyPickFive(t *testing.T) {
	t.Parallel()
	cands := make([]Candidate, 0, 30)
	for i := 30; i >= 1; i-- {
		cands = append(cands, Candidate{
			StockCode: fmt.Sprintf("sz%06d", i),
			StockName: fmt.Sprintf("N%d", i),
			Rank:      i,
			Score:     float64(100 - i),
		})
	}
	got := Select(cands, SelectionContext{MaxSelectedNames: 5})
	if got.SelectionLimit != 5 {
		t.Fatalf("selection_limit=%d want 5", got.SelectionLimit)
	}
	if len(got.RankedCandidates) != 30 {
		t.Fatalf("ranked=%d want 30 (waitlist stays in ranked_candidates)", len(got.RankedCandidates))
	}
	picks := got.PrimaryPicks()
	if len(picks) != 5 {
		t.Fatalf("primary=%d want 5", len(picks))
	}
	if len(got.Waitlist()) != 25 {
		t.Fatalf("waitlist=%d want 25", len(got.Waitlist()))
	}
	for i, d := range picks {
		wantRank := i + 1
		if d.Rank != wantRank || d.SelectionRank != wantRank {
			t.Fatalf("selected[%d] rank=%d selection_rank=%d want %d", i, d.Rank, d.SelectionRank, wantRank)
		}
		if !d.Selected || d.SelectionReason != ReasonRankTop {
			t.Fatalf("selected[%d] selected=%v reason=%q", i, d.Selected, d.SelectionReason)
		}
		if d.SkippedReason != "" {
			t.Fatalf("selected[%d] unexpected skipped_reason=%q", i, d.SkippedReason)
		}
	}
	for i, c := range got.Waitlist() {
		if c.Rank != i+6 {
			t.Fatalf("waitlist[%d] rank=%d want %d", i, c.Rank, i+6)
		}
	}

	legacy := got.ToSelectedCandidates()
	if len(legacy.Selected) != 5 || len(legacy.Skipped) != 25 {
		t.Fatalf("adapter selected=%d skipped=%d", len(legacy.Selected), len(legacy.Skipped))
	}
	for i, d := range legacy.Skipped {
		if d.Selected || d.SkippedReason != ReasonOverNameLimit {
			t.Fatalf("adapter skipped[%d] selected=%v reason=%q", i, d.Selected, d.SkippedReason)
		}
	}
}

func TestSelect_RankOrderIgnoresInputShuffle(t *testing.T) {
	t.Parallel()
	cands := []Candidate{
		{StockCode: "sz000003", Rank: 3, Score: 10},
		{StockCode: "sz000001", Rank: 1, Score: 90},
		{StockCode: "sz000005", Rank: 5, Score: 1},
		{StockCode: "sz000002", Rank: 2, Score: 80},
		{StockCode: "sz000004", Rank: 4, Score: 5},
	}
	got := Select(cands, SelectionContext{MaxSelectedNames: 3})
	if len(got.RankedCandidates) != 5 {
		t.Fatalf("ranked=%d", len(got.RankedCandidates))
	}
	want := []string{"sz000001", "sz000002", "sz000003", "sz000004", "sz000005"}
	for i, code := range want {
		if got.RankedCandidates[i].StockCode != code {
			t.Fatalf("ranked[%d]=%s want %s", i, got.RankedCandidates[i].StockCode, code)
		}
	}
	picks := got.PrimaryPicks()
	if len(picks) != 3 {
		t.Fatalf("selected=%d", len(picks))
	}
	for i, code := range want[:3] {
		if picks[i].Candidate.StockCode != code {
			t.Fatalf("selected[%d]=%s want %s", i, picks[i].Candidate.StockCode, code)
		}
		if picks[i].Rank != i+1 {
			t.Fatalf("selected[%d] rank=%d", i, picks[i].Rank)
		}
	}
}

func TestSelect_RankedOrderMatchesLegacySelectWalk(t *testing.T) {
	t.Parallel()
	cands := []Candidate{
		{StockCode: "sz000004", Rank: 4, Score: 1},
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000003", Rank: 3, Score: 3},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}
	got := Select(cands, SelectionContext{MaxSelectedNames: 2})
	legacy := SelectAsSelectedCandidates(cands, SelectionContext{MaxSelectedNames: 2})
	wantOrder := append(codesOf(legacy.Selected), codesOf(legacy.Skipped)...)
	gotOrder := make([]string, 0, len(got.RankedCandidates))
	for _, c := range got.RankedCandidates {
		gotOrder = append(gotOrder, c.StockCode)
	}
	if len(wantOrder) != len(gotOrder) {
		t.Fatalf("legacy walk=%v ranked=%v", wantOrder, gotOrder)
	}
	for i := range wantOrder {
		if wantOrder[i] != gotOrder[i] {
			t.Fatalf("index %d legacy=%s ranked=%s", i, wantOrder[i], gotOrder[i])
		}
	}
}

func TestSelect_SkippedReasonOverNameLimit(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}, SelectionContext{MaxSelectedNames: 1})
	picks := got.PrimaryPicks()
	if len(picks) != 1 || picks[0].Candidate.StockCode != "sz000001" {
		t.Fatalf("selected=%+v", picks)
	}
	if len(got.RankedCandidates) != 2 {
		t.Fatalf("ranked=%d want 2", len(got.RankedCandidates))
	}
	d := waitlistDecision(t, got, "sz000002")
	if d.SelectionReason != ReasonOverNameLimit || d.SkippedReason != ReasonOverNameLimit {
		t.Fatalf("waitlist reason=%q skip=%q", d.SelectionReason, d.SkippedReason)
	}
	if d.Selected {
		t.Fatalf("waitlist must not be selected: %+v", d)
	}
}

func TestSelect_EmptyCandidates(t *testing.T) {
	t.Parallel()
	got := Select(nil, SelectionContext{MaxSelectedNames: 5})
	if got == nil {
		t.Fatal("nil result")
	}
	if got.SelectionLimit != 5 {
		t.Fatalf("limit=%d", got.SelectionLimit)
	}
	if len(got.RankedCandidates) != 0 || len(got.CandidateDecisions) != 0 {
		t.Fatalf("ranked=%d decisions=%d", len(got.RankedCandidates), len(got.CandidateDecisions))
	}
	legacy := got.ToSelectedCandidates()
	if len(legacy.Selected) != 0 || len(legacy.Skipped) != 0 {
		t.Fatalf("adapter selected=%d skipped=%d", len(legacy.Selected), len(legacy.Skipped))
	}
	got = Select([]Candidate{}, SelectionContext{MaxSelectedNames: 5})
	if len(got.RankedCandidates) != 0 || len(got.CandidateDecisions) != 0 {
		t.Fatalf("empty slice ranked=%d decisions=%d", len(got.RankedCandidates), len(got.CandidateDecisions))
	}
}

func TestSelect_ScoreBreaksUnranked(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000002", Rank: 0, Score: 10},
		{StockCode: "sz000001", Rank: 1, Score: 1},
		{StockCode: "sz000003", Rank: 0, Score: 50},
	}, SelectionContext{MaxSelectedNames: 2})
	if got.RankedCandidates[0].StockCode != "sz000001" {
		t.Fatalf("rank 1 should win first, got %s", got.RankedCandidates[0].StockCode)
	}
	if got.RankedCandidates[1].StockCode != "sz000003" {
		t.Fatalf("higher score among unranked, got %s", got.RankedCandidates[1].StockCode)
	}
	if got.PrimaryPicks()[1].Candidate.StockCode != "sz000003" {
		t.Fatalf("basket second=%s", got.PrimaryPicks()[1].Candidate.StockCode)
	}
}

func TestSelect_DoesNotSkipExistingHoldings(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 90},
		{StockCode: "SZ000001", Rank: 2, Score: 80}, // dup of holding name
		{StockCode: "sz000002", Rank: 3, Score: 70},
		{StockCode: "sz000003", Rank: 4, Score: 60},
	}, SelectionContext{
		MaxSelectedNames:  2,
		ExistingPositions: []ExistingPosition{{StockCode: "SZ000001"}},
	})
	if len(got.RankedCandidates) != 3 {
		t.Fatalf("ranked=%d want 3 (holding kept, dup dropped)", len(got.RankedCandidates))
	}
	if got.RankedCandidates[0].StockCode != "sz000001" {
		t.Fatalf("held name must remain first in ranked, got %s", got.RankedCandidates[0].StockCode)
	}
	picks := got.PrimaryPicks()
	if len(picks) != 2 {
		t.Fatalf("selected=%d want 2", len(picks))
	}
	if picks[0].Candidate.StockCode != "sz000001" || picks[1].Candidate.StockCode != "sz000002" {
		t.Fatalf("selected=%s %s", picks[0].Candidate.StockCode, picks[1].Candidate.StockCode)
	}
	if picks[0].SelectionReason != ReasonRankTop {
		t.Fatalf("holding must not emit already_holding, got %q", picks[0].SelectionReason)
	}
}

func TestSelect_DuplicateKeepsBetterRank(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000002", Rank: 2, Score: 50},
		{StockCode: "SZ000002", Rank: 9, Score: 99},
		{StockCode: "sz000001", Rank: 1, Score: 80},
	}, SelectionContext{MaxSelectedNames: 2})
	if len(got.RankedCandidates) != 2 {
		t.Fatalf("ranked=%d", len(got.RankedCandidates))
	}
	if got.RankedCandidates[0].StockCode != "sz000001" {
		t.Fatalf("first=%s", got.RankedCandidates[0].StockCode)
	}
	if got.RankedCandidates[1].StockCode != "sz000002" || got.RankedCandidates[1].Rank != 2 {
		t.Fatalf("kept worse-score better-rank, got %+v", got.RankedCandidates[1])
	}
	var dup *CandidateDecision
	for i := range got.CandidateDecisions {
		if got.CandidateDecisions[i].SelectionReason == ReasonDuplicateSymbol {
			dup = &got.CandidateDecisions[i]
			break
		}
	}
	if dup == nil || dup.SkipReason != ReasonDuplicateSymbol {
		t.Fatalf("dup skip=%+v", got.CandidateDecisions)
	}
	if containsCode(got.RankedCandidates, "SZ000002") {
		t.Fatal("duplicate row must not appear in ranked_candidates")
	}
	if !rankedHasNormalized(got, "sz000002") {
		t.Fatal("kept symbol missing from ranked_candidates")
	}
}

func TestSelect_CashLimitNotApplied(t *testing.T) {
	t.Parallel()
	cands := []Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
		{StockCode: "sz000003", Rank: 3, Score: 7},
	}
	got := Select(cands, SelectionContext{
		MaxSelectedNames:       5,
		AvailableCash:          150_000,
		EstimatedAmountPerName: 100_000,
	})
	if len(got.RankedCandidates) != 3 || len(got.PrimaryPicks()) != 3 {
		t.Fatalf("cash_limit must not apply: ranked=%d selected=%d", len(got.RankedCandidates), len(got.PrimaryPicks()))
	}
	for _, d := range got.CandidateDecisions {
		if d.SelectionReason == ReasonCashLimit {
			t.Fatalf("unexpected cash_limit for %s", d.Candidate.StockCode)
		}
	}

	none := Select(cands, SelectionContext{
		MaxSelectedNames:       5,
		AvailableCash:          0,
		EstimatedAmountPerName: 100_000,
	})
	if len(none.PrimaryPicks()) != 3 {
		t.Fatalf("zero cash must not drop basket, selected=%d", len(none.PrimaryPicks()))
	}
}

func TestSelect_CashOffWhenAmountUnset(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}, SelectionContext{
		MaxSelectedNames: 2,
		AvailableCash:    0,
	})
	if len(got.PrimaryPicks()) != 2 {
		t.Fatalf("without estimated_amount_per_name, cash must not apply, selected=%d", len(got.PrimaryPicks()))
	}
}

func TestSelect_SelectionLimitMarksBasketOnly(t *testing.T) {
	t.Parallel()
	cands := []Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
		{StockCode: "sz000003", Rank: 3, Score: 7},
	}
	got := Select(cands, SelectionContext{MaxSelectedNames: 1})
	if got.SelectionLimit != 1 {
		t.Fatalf("limit=%d", got.SelectionLimit)
	}
	trueCount := 0
	for _, d := range got.CandidateDecisions {
		if d.Selected {
			trueCount++
			if d.SelectionReason != ReasonRankTop {
				t.Fatalf("in-basket reason=%q", d.SelectionReason)
			}
		}
	}
	if trueCount != 1 {
		t.Fatalf("selected=true count=%d want 1", trueCount)
	}
	if len(got.RankedForPlanFilter()) != 3 {
		t.Fatalf("filter scan list must keep waitlist, got %d", len(got.RankedForPlanFilter()))
	}
}

func TestSelect_SelectedIsBasketNotTradeEligibility(t *testing.T) {
	t.Parallel()
	// Holding + tiny cash would have been skipped in E.2. E.6 still marks basket.
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 90, Reason: "pool"},
		{StockCode: "sz000002", Rank: 2, Score: 80},
	}, SelectionContext{
		MaxSelectedNames:       2,
		ExistingPositions:      []ExistingPosition{{StockCode: "sz000001"}},
		AvailableCash:          0,
		EstimatedAmountPerName: 100_000,
		PortfolioSnapshot:      &PortfolioSnapshot{Cash: 0, Equity: 1},
	})
	picks := got.PrimaryPicks()
	if len(picks) != 2 {
		t.Fatalf("selected=%d want 2", len(picks))
	}
	if !picks[0].Selected || picks[0].SelectionReason != ReasonRankTop {
		t.Fatalf("sz000001 is in-basket only, not a trade gate: %+v", picks[0])
	}
	if picks[0].SelectionReason == ReasonAlreadyHolding || picks[0].SelectionReason == ReasonCashLimit || picks[0].SelectionReason == ReasonRiskLimit {
		t.Fatalf("eligibility reasons must not come from Select: %q", picks[0].SelectionReason)
	}
}

func TestSelect_InvalidEmptyCodeNotRanked(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "  ", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}, SelectionContext{MaxSelectedNames: 5})
	if len(got.RankedCandidates) != 1 || got.RankedCandidates[0].StockCode != "sz000002" {
		t.Fatalf("ranked=%+v", got.RankedCandidates)
	}
	if got.CandidateDecisions[0].SelectionReason != ReasonInvalidCandidate {
		t.Fatalf("first decision=%q", got.CandidateDecisions[0].SelectionReason)
	}
	if got.CandidateDecisions[0].Selected {
		t.Fatal("invalid must not be selected")
	}
}

func TestSelect_DefaultLimitWhenUnset(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{{StockCode: "sz000001", Rank: 1}}, SelectionContext{})
	if got.SelectionLimit != DefaultMaxSelectedNames {
		t.Fatalf("limit=%d want %d", got.SelectionLimit, DefaultMaxSelectedNames)
	}
}

func codesOf(ds []CandidateDecision) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Candidate.StockCode)
	}
	return out
}

func waitlistDecision(t *testing.T, got *CandidateSelectionResult, code string) CandidateDecision {
	t.Helper()
	for _, d := range got.CandidateDecisions {
		if d.Candidate.StockCode == code {
			return d
		}
	}
	t.Fatalf("no decision for %s", code)
	return CandidateDecision{}
}

func containsCode(cands []Candidate, code string) bool {
	for _, c := range cands {
		if c.StockCode == code {
			return true
		}
	}
	return false
}

func rankedHasNormalized(got *CandidateSelectionResult, code string) bool {
	want := normalizeCode(code)
	for _, c := range got.RankedCandidates {
		if normalizeCode(c.StockCode) == want {
			return true
		}
	}
	return false
}
