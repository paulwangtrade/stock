// Package sectorclassification is Phase12-H0.3 SectorClassificationProvider (read-only).
//
// symbol → SectorInfo for injection into portfoliorisk.BuildInput.IndustryBySymbol.
// Never invents default sector "0"/"unknown"; never touches trade write paths.
package sectorclassification

import (
	"strings"
	"time"
)

const (
	SourceInject  = "inject"
	SourceFixture = "fixture"
	SourceManual  = "manual"
)

// SectorInfo is the per-symbol classification result.
type SectorInfo struct {
	SectorName string `json:"sector_name"`
	Source     string `json:"source"`
	Version    string `json:"version"`
}

// Meta is audit metadata for one Resolve call.
type Meta struct {
	Version        string    `json:"version,omitempty"`
	Source         string    `json:"source,omitempty"`
	AsOf           time.Time `json:"as_of,omitempty"`
	RequestedCount int       `json:"requested_count"`
	ResolvedCount  int       `json:"resolved_count"`
	Coverage       float64   `json:"coverage"`
	MissingSymbols []string  `json:"missing_symbols,omitempty"`
}

// Provider resolves stock codes to SectorInfo. Missing codes are omitted — never filled.
type Provider interface {
	// Lookup returns SectorInfo for one symbol. ok=false when unknown / empty.
	Lookup(symbol string) (info SectorInfo, ok bool)
	// Resolve returns only successfully classified codes.
	Resolve(codes []string) (bySymbol map[string]SectorInfo, meta Meta, err error)
}

// StaticProvider is an inject/fixture table: symbol → sector_name.
type StaticProvider struct {
	// Sectors maps normalized or raw codes to sector display names.
	Sectors map[string]string
	Source  string // inject | fixture | manual
	Version string // taxonomy / version label → Build IndustryTaxonomy
	AsOf    time.Time
}

// Lookup implements Provider.
func (p StaticProvider) Lookup(symbol string) (SectorInfo, bool) {
	code := norm(symbol)
	if code == "" || len(p.Sectors) == 0 {
		return SectorInfo{}, false
	}
	base := normalizeTable(p.Sectors)
	sec, ok := base[code]
	if !ok || sec == "" {
		return SectorInfo{}, false
	}
	return SectorInfo{
		SectorName: sec,
		Source:     p.sourceOrDefault(),
		Version:    strings.TrimSpace(p.Version),
	}, true
}

// Resolve implements Provider.
func (p StaticProvider) Resolve(codes []string) (map[string]SectorInfo, Meta, error) {
	meta := Meta{
		Version: strings.TrimSpace(p.Version),
		Source:  p.sourceOrDefault(),
		AsOf:    p.AsOf,
	}
	uniq := uniqueNormCodes(codes)
	meta.RequestedCount = len(uniq)
	if len(uniq) == 0 {
		return nil, meta, nil
	}
	out := make(map[string]SectorInfo)
	var missing []string
	for _, c := range uniq {
		info, ok := p.Lookup(c)
		if !ok {
			missing = append(missing, c)
			continue
		}
		out[c] = info
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

func (p StaticProvider) sourceOrDefault() string {
	s := strings.TrimSpace(p.Source)
	if s == "" {
		return SourceInject
	}
	return s
}

// IndustryBySymbol converts Resolve output into portfoliorisk.BuildInput.IndustryBySymbol.
// Empty / nil → nil (Build keeps sector.available=false).
func IndustryBySymbol(by map[string]SectorInfo) map[string]string {
	if len(by) == 0 {
		return nil
	}
	out := make(map[string]string, len(by))
	for code, info := range by {
		k := norm(code)
		sec := strings.TrimSpace(info.SectorName)
		if k == "" || sec == "" {
			continue
		}
		if isSentinelSector(sec) {
			continue
		}
		out[k] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ForBuildInput resolves holdings (and optional candidates) and returns map + version taxonomy
// for portfoliorisk.BuildInput. Partial holding coverage still returns the partial map;
// Build itself fail-closes sector.available when any Volume>0 holding is missing.
func ForBuildInput(p Provider, holdings []string, candidates ...[]string) (industryBySymbol map[string]string, version string, meta Meta) {
	if p == nil {
		return nil, "", Meta{}
	}
	codes := append([]string{}, holdings...)
	for _, c := range candidates {
		codes = append(codes, c...)
	}
	by, meta, err := p.Resolve(codes)
	if err != nil || len(by) == 0 {
		return nil, "", meta
	}
	return IndustryBySymbol(by), strings.TrimSpace(meta.Version), meta
}

func normalizeTable(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		code := norm(k)
		sec := strings.TrimSpace(v)
		if code == "" || sec == "" || isSentinelSector(sec) {
			continue
		}
		out[code] = sec
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isSentinelSector(sec string) bool {
	return strings.EqualFold(sec, "unknown") || sec == "0" || strings.EqualFold(sec, "none")
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

func norm(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
