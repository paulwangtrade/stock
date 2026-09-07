package selection

import (
	"sort"
	"strings"
)

// Select ranks candidates and annotates an in-basket prefix of SelectionLimit.
// E.6: Rank ascending (1 best), then Score descending; drop empty codes; drop
// duplicate symbols (keep better Rank). Does not skip holdings, cash, or risk.
// Does not load DB or call Portfolio APIs. Does not replace PlanFilter.
func Select(candidates []Candidate, ctx SelectionContext) *CandidateSelectionResult {
	maxN := ctx.MaxSelectedNames
	if maxN <= 0 {
		maxN = DefaultMaxSelectedNames
	}
	out := &CandidateSelectionResult{
		RankedCandidates:   []Candidate{},
		SelectionLimit:     maxN,
		CandidateDecisions: []CandidateDecision{},
	}
	if len(candidates) == 0 {
		return out
	}

	indexed := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		indexed = append(indexed, cloneCandidate(c))
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		return lessCandidate(indexed[i], indexed[j])
	})

	seen := map[string]bool{}
	taken := 0
	for _, c := range indexed {
		key := normalizeCode(c.StockCode)
		if key == "" {
			out.CandidateDecisions = append(out.CandidateDecisions, skippedDecision(c, ReasonInvalidCandidate))
			continue
		}
		if DeduplicateSymbols && seen[key] {
			out.CandidateDecisions = append(out.CandidateDecisions, skippedDecision(c, ReasonDuplicateSymbol))
			continue
		}
		seen[key] = true
		out.RankedCandidates = append(out.RankedCandidates, c)
		if taken < maxN {
			taken++
			out.CandidateDecisions = append(out.CandidateDecisions, CandidateDecision{
				Candidate:       c,
				Selected:        true,
				Rank:            c.Rank,
				SelectionReason: ReasonRankTop,
				SelectionRank:   taken,
			})
			continue
		}
		out.CandidateDecisions = append(out.CandidateDecisions, skippedDecision(c, ReasonOverNameLimit))
	}
	return out
}

// SelectAsSelectedCandidates is the E.1/E.2 compatibility adapter.
// Same ranking/dedup as Select; does not apply already_holding or cash_limit.
func SelectAsSelectedCandidates(candidates []Candidate, ctx SelectionContext) *SelectedCandidates {
	return Select(candidates, ctx).ToSelectedCandidates()
}

func normalizeCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func cloneCandidate(c Candidate) Candidate {
	c.StockCode = strings.TrimSpace(c.StockCode)
	c.StockName = strings.TrimSpace(c.StockName)
	c.Industry = strings.TrimSpace(c.Industry)
	c.Reason = strings.TrimSpace(c.Reason)
	return c
}

func lessCandidate(a, b Candidate) bool {
	ar, br := a.Rank, b.Rank
	aRanked := ar > 0
	bRanked := br > 0
	switch {
	case aRanked && bRanked && ar != br:
		return ar < br
	case aRanked != bRanked:
		return aRanked
	case a.Score != b.Score:
		return a.Score > b.Score
	default:
		return strings.ToLower(a.StockCode) < strings.ToLower(b.StockCode)
	}
}

func skippedDecision(c Candidate, reason string) CandidateDecision {
	return CandidateDecision{
		Candidate:       c,
		Selected:        false,
		Rank:            c.Rank,
		SelectionReason: reason,
		SkippedReason:   reason,
		SkipReason:      reason,
	}
}
