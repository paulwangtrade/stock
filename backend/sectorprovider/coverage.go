package sectorprovider

import (
	"sort"
	"strings"
	"time"
)

// EvaluateCoverage builds CoverageReport for holdings (gate) and optional candidates.
// AllowSectorConstraint is true only when holdings are fully classified with real labels.
func EvaluateCoverage(p Provider, in CoverageInput) CoverageReport {
	rep := CoverageReport{
		SchemaVersion:       SchemaVersion,
		AsOf:                in.AsOf,
		MinCoverageRequired: in.MinHoldingsCoverage,
		HoldingsMissing:     []string{},
		CandidatesMissing:   []string{},
	}
	if rep.AsOf.IsZero() {
		rep.AsOf = time.Now().UTC()
	}
	if rep.MinCoverageRequired <= 0 {
		rep.MinCoverageRequired = DefaultMinHoldingsCoverage
	}
	if p != nil {
		m := p.Meta()
		rep.Source = m.Source
		rep.Version = m.Version
		rep.Taxonomy = m.Taxonomy
	}

	holdings := uniqueNorm(in.Holdings)
	cands := uniqueNorm(in.Candidates)
	rep.HoldingsRequested = len(holdings)
	rep.CandidatesRequested = len(cands)

	if p == nil {
		rep.HoldingsMissing = append([]string(nil), holdings...)
		rep.CandidatesMissing = append([]string(nil), cands...)
		rep.Note = "provider_nil"
		rep.AllowSectorConstraint = false
		return rep
	}

	holdRes, holdMiss, holdSent := resolveSet(p, holdings)
	candRes, candMiss, candSent := resolveSet(p, cands)
	rep.HoldingsResolved = holdRes
	rep.HoldingsMissing = holdMiss
	rep.CandidatesResolved = candRes
	rep.CandidatesMissing = candMiss
	rep.RejectedSentinels = holdSent + candSent

	if rep.HoldingsRequested > 0 {
		rep.HoldingsCoverage = float64(rep.HoldingsResolved) / float64(rep.HoldingsRequested)
	}
	if rep.CandidatesRequested > 0 {
		rep.CandidatesCoverage = float64(rep.CandidatesResolved) / float64(rep.CandidatesRequested)
	}

	rep.HoldingsComplete = rep.HoldingsRequested == 0 ||
		(len(rep.HoldingsMissing) == 0 && rep.HoldingsCoverage+1e-12 >= rep.MinCoverageRequired)

	switch {
	case rep.HoldingsRequested == 0:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = true
		rep.Note = "empty_holdings_cannot_enable_sector_constraint"
	case len(rep.HoldingsMissing) > 0 || rep.HoldingsCoverage+1e-12 < rep.MinCoverageRequired:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = false
		rep.Note = "holdings_coverage_incomplete"
	case rep.RejectedSentinels > 0:
		rep.AllowSectorConstraint = false
		rep.HoldingsComplete = false
		rep.Note = "sentinel_labels_rejected"
	default:
		rep.AllowSectorConstraint = true
		rep.HoldingsComplete = true
		rep.Note = "holdings_fully_classified"
	}
	return rep
}

func resolveSet(p Provider, codes []string) (resolved int, missing []string, sentinels int) {
	missing = []string{}
	for _, c := range codes {
		info, ok := p.Lookup(c)
		if !ok {
			missing = append(missing, c)
			continue
		}
		ind := strings.TrimSpace(info.Industry)
		sec := info.EffectiveSector()
		if IsSentinelLabel(ind) || IsSentinelLabel(sec) {
			sentinels++
			missing = append(missing, c)
			continue
		}
		resolved++
	}
	sort.Strings(missing)
	return resolved, missing, sentinels
}

func uniqueNorm(codes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		k := NormSymbol(c)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
