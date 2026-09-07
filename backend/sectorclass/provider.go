// Package sectorclass is Phase12-H0.3 Sector Classification Provider.
//
// It maps symbol → sector into IndustryBySymbol for portfoliorisk.BuildInput only.
// Standalone: no trade-plan persist, no execution, no allocation engine, no freeze path.
// Missing symbols are omitted — never invent "unknown" or zero-weight fakes.
package sectorclass

import (
	"strings"
	"time"
)

const (
	SourceInject  = "inject"
	SourceFixture = "fixture"
)

// Meta is audit metadata for one Resolve / Classify call.
type Meta struct {
	Taxonomy          string    `json:"taxonomy,omitempty"`
	Source            string    `json:"source,omitempty"`
	AsOf              time.Time `json:"as_of,omitempty"`
	RequestedCount    int       `json:"requested_count"`
	ResolvedCount     int       `json:"resolved_count"`
	Coverage          float64   `json:"coverage"` // resolved/requested; 0 when requested=0
	MissingSymbols    []string  `json:"missing_symbols,omitempty"`
	HoldingsComplete  bool      `json:"holdings_complete"`  // every holding symbol resolved
	CandidatesPartial bool      `json:"candidates_partial"` // some candidates missing (informational)
}

// Provider resolves symbol → sector. Missing codes are omitted — never filled with "unknown".
type Provider interface {
	Resolve(codes []string) (industryBySymbol map[string]string, meta Meta, err error)
}

// StaticMap is an inject/fixture provider backed by an explicit symbol→sector table.
type StaticMap struct {
	IndustryBySymbol map[string]string
	Taxonomy         string
	Source           string
	AsOf             time.Time
}

// Resolve returns IndustryBySymbol for requested codes only. Empty sector values omitted.
func (p StaticMap) Resolve(codes []string) (map[string]string, Meta, error) {
	meta := Meta{
		Taxonomy: strings.TrimSpace(p.Taxonomy),
		Source:   strings.TrimSpace(p.Source),
		AsOf:     p.AsOf,
	}
	if meta.Source == "" {
		meta.Source = SourceInject
	}
	base := Normalize(p.IndustryBySymbol)
	uniq := uniqueNormCodes(codes)
	meta.RequestedCount = len(uniq)
	if len(base) == 0 || len(uniq) == 0 {
		meta.Coverage = 0
		if len(uniq) > 0 {
			meta.MissingSymbols = append([]string(nil), uniq...)
		}
		return nil, meta, nil
	}
	out := make(map[string]string, len(uniq))
	var missing []string
	for _, k := range uniq {
		if sec, ok := base[k]; ok && sec != "" {
			out[k] = sec
			continue
		}
		missing = append(missing, k)
	}
	meta.ResolvedCount = len(out)
	meta.MissingSymbols = missing
	if meta.RequestedCount > 0 {
		meta.Coverage = float64(meta.ResolvedCount) / float64(meta.RequestedCount)
	}
	if len(out) == 0 {
		return nil, meta, nil
	}
	return out, meta, nil
}

// ClassifyInput is Holdings + Candidate symbols for one classification pass.
type ClassifyInput struct {
	Holdings   []string
	Candidates []string
}

// ClassifyResult is the map intended for portfoliorisk.BuildInput.IndustryBySymbol.
type ClassifyResult struct {
	IndustryBySymbol map[string]string
	Meta             Meta
}

// Classify resolves Holdings ∪ Candidates via Provider.
// Does not invent sectors for missing symbols. Empty holdings+candidates → empty map.
func Classify(p Provider, in ClassifyInput) (ClassifyResult, error) {
	out := ClassifyResult{}
	if p == nil {
		return out, nil
	}
	hold := uniqueNormCodes(in.Holdings)
	cand := uniqueNormCodes(in.Candidates)
	codes := mergeUnique(hold, cand)
	if len(codes) == 0 {
		return out, nil
	}
	m, meta, err := p.Resolve(codes)
	if err != nil {
		return out, err
	}
	meta.HoldingsComplete = holdingsFullyCovered(hold, m)
	meta.CandidatesPartial = candidatesHaveGaps(cand, m)
	out.IndustryBySymbol = Normalize(m)
	out.Meta = meta
	return out, nil
}

// ForBuildInput returns IndustryBySymbol + taxonomy suitable for portfoliorisk.BuildInput.
// On empty/error returns nil map (Build keeps sector.available=false).
func ForBuildInput(p Provider, in ClassifyInput) (industryBySymbol map[string]string, taxonomy string) {
	res, err := Classify(p, in)
	if err != nil || len(res.IndustryBySymbol) == 0 {
		return nil, ""
	}
	return res.IndustryBySymbol, strings.TrimSpace(res.Meta.Taxonomy)
}

// Normalize lowercases symbol keys and trims sectors; drops empty values (no "unknown").
func Normalize(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		code := norm(k)
		sec := strings.TrimSpace(v)
		if code == "" || sec == "" {
			continue
		}
		// Refuse sentinel fakes.
		if strings.EqualFold(sec, "unknown") || sec == "0" {
			continue
		}
		out[code] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func holdingsFullyCovered(holdings []string, m map[string]string) bool {
	if len(holdings) == 0 {
		return true
	}
	for _, h := range holdings {
		if strings.TrimSpace(m[h]) == "" {
			return false
		}
	}
	return true
}

func candidatesHaveGaps(candidates []string, m map[string]string) bool {
	for _, c := range candidates {
		if strings.TrimSpace(m[c]) == "" {
			return true
		}
	}
	return false
}

func uniqueNormCodes(codes []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		k := norm(c)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

func mergeUnique(a, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, xs := range [][]string{a, b} {
		for _, x := range xs {
			if _, ok := seen[x]; ok {
				continue
			}
			seen[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
