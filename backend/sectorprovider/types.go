// Package sectorprovider is the Phase12-H0.3 SectorClassificationProvider data layer.
//
// Real data access: symbol → industry → sector → source → version.
// Produces CoverageReport to decide when sector constraints may be enabled.
//
// Fail-closed: "unknown"/empty never counts as a valid sector; incomplete coverage
// never sets AllowSectorConstraint=true.
//
// Does not modify PlanFilter, TradePlan, or Execution. Does not wire the buy/sell chain.
package sectorprovider

import (
	"strings"
	"time"
)

const (
	SchemaVersion = "sectorprovider.h0-3-v1"

	SourceInject   = "inject"
	SourceFixture  = "fixture"
	SourceManual   = "manual"
	SourceCache    = "cache"
	SourceBasicTable = "basic_table"
	SourceEastmoney = "eastmoney_cached"

	TaxonomyEastmoneyIndustry = "eastmoney_industry_v1"
	TaxonomyFixture           = "fixture"
	TaxonomyManual            = "manual"

	// DefaultMinHoldingsCoverage is fail-closed: holdings must be fully classified.
	DefaultMinHoldingsCoverage = 1.0
)

// Classification is symbol → industry → sector with provenance.
type Classification struct {
	Symbol   string `json:"symbol"`
	Industry string `json:"industry"` // fine label (e.g. 东财 INDUSTRY)
	Sector   string `json:"sector"`   // exposure bucket; defaults to Industry when empty
	Board    string `json:"board,omitempty"` // optional BKName; NOT used as sector unless taxonomy says so
	Source   string `json:"source"`
	Version  string `json:"version"`
}

// EffectiveSector returns the sector bucket for exposure (Industry fallback).
func (c Classification) EffectiveSector() string {
	s := strings.TrimSpace(c.Sector)
	if s != "" {
		return s
	}
	return strings.TrimSpace(c.Industry)
}

// CoverageReport judges whether sector constraints may be turned on.
// Missing symbols are listed; they are never treated as weight 0.
type CoverageReport struct {
	SchemaVersion string    `json:"schema_version"`
	AsOf          time.Time `json:"as_of,omitempty"`
	Taxonomy      string    `json:"taxonomy,omitempty"`
	Source        string    `json:"source,omitempty"`
	Version       string    `json:"version,omitempty"`

	HoldingsRequested   int      `json:"holdings_requested"`
	HoldingsResolved    int      `json:"holdings_resolved"`
	HoldingsCoverage    float64  `json:"holdings_coverage"` // resolved/requested; 0 when requested=0
	HoldingsMissing     []string `json:"holdings_missing,omitempty"`
	HoldingsComplete    bool     `json:"holdings_complete"`

	CandidatesRequested int      `json:"candidates_requested,omitempty"`
	CandidatesResolved  int      `json:"candidates_resolved,omitempty"`
	CandidatesCoverage  float64  `json:"candidates_coverage,omitempty"`
	CandidatesMissing   []string `json:"candidates_missing,omitempty"`

	MinCoverageRequired   float64 `json:"min_coverage_required"`
	AllowSectorConstraint bool    `json:"allow_sector_constraint"`
	Note                  string  `json:"note,omitempty"`

	// RejectedSentinels counts codes whose labels were refused (unknown/0/…).
	RejectedSentinels int `json:"rejected_sentinels,omitempty"`
}

// CoverageInput scopes holdings (gate) vs candidates (informational).
type CoverageInput struct {
	Holdings   []string
	Candidates []string
	// MinHoldingsCoverage defaults to DefaultMinHoldingsCoverage (1.0).
	MinHoldingsCoverage float64
	AsOf                time.Time
}

// Provider is the SectorClassificationProvider data-layer contract.
type Provider interface {
	// Lookup returns one classification. ok=false when missing / sentinel / empty.
	Lookup(symbol string) (Classification, bool)
	// Resolve returns only successful classifications (never fills unknown).
	Resolve(codes []string) (bySymbol map[string]Classification, err error)
	// Meta returns provider provenance defaults.
	Meta() ProviderMeta
}

// ProviderMeta is static provenance for a Provider instance.
type ProviderMeta struct {
	Source   string `json:"source"`
	Version  string `json:"version"`
	Taxonomy string `json:"taxonomy"`
}

// IndustryBySymbol projects classifications into portfoliorisk.BuildInput shape.
// Empty / all-sentinel → nil (caller keeps sector.available=false).
func IndustryBySymbol(by map[string]Classification) map[string]string {
	if len(by) == 0 {
		return nil
	}
	out := make(map[string]string, len(by))
	for code, c := range by {
		k := NormSymbol(code)
		sec := c.EffectiveSector()
		if k == "" || sec == "" || IsSentinelLabel(sec) {
			continue
		}
		if IsSentinelLabel(c.Industry) {
			continue
		}
		out[k] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ForBuildInput resolves holdings∪candidates for Build injection.
// Partial maps are returned as-is; CoverageReport / Build fail-close on incomplete holdings.
func ForBuildInput(p Provider, holdings []string, candidates ...[]string) (map[string]string, ProviderMeta, error) {
	meta := ProviderMeta{}
	if p == nil {
		return nil, meta, nil
	}
	meta = p.Meta()
	codes := append([]string{}, holdings...)
	for _, c := range candidates {
		codes = append(codes, c...)
	}
	by, err := p.Resolve(codes)
	if err != nil || len(by) == 0 {
		return nil, meta, err
	}
	return IndustryBySymbol(by), meta, nil
}

// IsSentinelLabel rejects labels that must never open sector constraints / fake zeros.
func IsSentinelLabel(s string) bool {
	v := strings.TrimSpace(s)
	if v == "" {
		return true
	}
	switch strings.ToLower(v) {
	case "unknown", "0", "none", "null", "n/a", "na", "undefined", "-":
		return true
	default:
		return false
	}
}

// NormSymbol lowercases and trims stock codes.
func NormSymbol(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
