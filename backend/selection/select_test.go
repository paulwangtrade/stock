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
	if len(got.Selected) != 5 {
		t.Fatalf("selected=%d want 5", len(got.Selected))
	}
	if len(got.Skipped) != 25 {
		t.Fatalf("skipped=%d want 25", len(got.Skipped))
	}
	for i, d := range got.Selected {
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
	for i, d := range got.Skipped {
		if d.Selected || d.SkippedReason != ReasonOverNameLimit {
			t.Fatalf("skipped[%d] selected=%v reason=%q", i, d.Selected, d.SkippedReason)
		}
		if d.Rank != i+6 {
			t.Fatalf("skipped[%d] rank=%d want %d", i, d.Rank, i+6)
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
	if len(got.Selected) != 3 {
		t.Fatalf("selected=%d", len(got.Selected))
	}
	want := []string{"sz000001", "sz000002", "sz000003"}
	for i, code := range want {
		if got.Selected[i].Candidate.StockCode != code {
			t.Fatalf("selected[%d]=%s want %s", i, got.Selected[i].Candidate.StockCode, code)
		}
		if got.Selected[i].Rank != i+1 {
			t.Fatalf("selected[%d] rank=%d", i, got.Selected[i].Rank)
		}
	}
	if got.Skipped[0].Candidate.StockCode != "sz000004" || got.Skipped[1].Candidate.StockCode != "sz000005" {
		t.Fatalf("skipped codes = %s %s", got.Skipped[0].Candidate.StockCode, got.Skipped[1].Candidate.StockCode)
	}
}

func TestSelect_SkippedReasonOverNameLimit(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}, SelectionContext{MaxSelectedNames: 1})
	if len(got.Selected) != 1 || got.Selected[0].Candidate.StockCode != "sz000001" {
		t.Fatalf("selected=%+v", got.Selected)
	}
	if len(got.Skipped) != 1 {
		t.Fatalf("skipped=%d", len(got.Skipped))
	}
	d := got.Skipped[0]
	if d.SkippedReason != ReasonOverNameLimit {
		t.Fatalf("skipped_reason=%q want %s", d.SkippedReason, ReasonOverNameLimit)
	}
	if d.SelectionReason != "" || d.Selected {
		t.Fatalf("skipped must not be selected: %+v", d)
	}
}

func TestSelect_EmptyCandidates(t *testing.T) {
	t.Parallel()
	got := Select(nil, SelectionContext{MaxSelectedNames: 5})
	if got == nil {
		t.Fatal("nil result")
	}
	if len(got.Selected) != 0 || len(got.Skipped) != 0 {
		t.Fatalf("selected=%d skipped=%d", len(got.Selected), len(got.Skipped))
	}
	got = Select([]Candidate{}, SelectionContext{MaxSelectedNames: 5})
	if len(got.Selected) != 0 || len(got.Skipped) != 0 {
		t.Fatalf("empty slice selected=%d skipped=%d", len(got.Selected), len(got.Skipped))
	}
}

func TestSelect_ScoreBreaksUnranked(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000002", Rank: 0, Score: 10},
		{StockCode: "sz000001", Rank: 1, Score: 1},
		{StockCode: "sz000003", Rank: 0, Score: 50},
	}, SelectionContext{MaxSelectedNames: 2})
	if got.Selected[0].Candidate.StockCode != "sz000001" {
		t.Fatalf("rank 1 should win first, got %s", got.Selected[0].Candidate.StockCode)
	}
	if got.Selected[1].Candidate.StockCode != "sz000003" {
		t.Fatalf("higher score among unranked, got %s", got.Selected[1].Candidate.StockCode)
	}
}

func TestSelect_DoesNotReadReservedHoldingsOrCash(t *testing.T) {
	t.Parallel()
	got := Select([]Candidate{
		{StockCode: "sz000001", Rank: 1, Score: 9},
		{StockCode: "sz000002", Rank: 2, Score: 8},
	}, SelectionContext{
		MaxSelectedNames:  2,
		AvailableCash:     0,
		ExistingPositions: []ExistingPosition{{StockCode: "sz000001"}},
		PortfolioSnapshot: &PortfolioSnapshot{Cash: 0, Equity: 0},
	})
	if len(got.Selected) != 2 {
		t.Fatalf("E.1 must not skip holdings or cash, selected=%d", len(got.Selected))
	}
}
