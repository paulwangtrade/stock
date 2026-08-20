package selection

import (
	"sort"
	"strings"
)

// Select ranks candidates and keeps at most MaxSelectedNames.
// E.1: Rank ascending (1 best), then Score descending. Does not read holdings or cash.
func Select(candidates []Candidate, ctx SelectionContext) *SelectedCandidates {
	out := &SelectedCandidates{
		Selected: []CandidateDecision{},
		Skipped:  []CandidateDecision{},
	}
	if len(candidates) == 0 {
		return out
	}

	maxN := ctx.MaxSelectedNames
	if maxN <= 0 {
		maxN = DefaultMaxSelectedNames
	}

	indexed := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		indexed = append(indexed, cloneCandidate(c))
	}
	sort.SliceStable(indexed, func(i, j int) bool {
		return lessCandidate(indexed[i], indexed[j])
	})

	taken := 0
	for _, c := range indexed {
		if strings.TrimSpace(c.StockCode) == "" {
			out.Skipped = append(out.Skipped, skippedDecision(c, ReasonInvalidCandidate))
			continue
		}
		if taken >= maxN {
			out.Skipped = append(out.Skipped, skippedDecision(c, ReasonOverNameLimit))
			continue
		}
		taken++
		out.Selected = append(out.Selected, CandidateDecision{
			Candidate:       c,
			Selected:        true,
			Rank:            c.Rank,
			SelectionReason: ReasonRankTop,
			SelectionRank:   taken,
		})
	}
	return out
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
		Candidate:     c,
		Selected:      false,
		Rank:          c.Rank,
		SkippedReason: reason,
	}
}
