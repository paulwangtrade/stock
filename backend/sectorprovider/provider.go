package sectorprovider

import (
	"strings"
	"time"
)

// TableEntry is one static classification row.
type TableEntry struct {
	Industry string
	Sector   string // optional; empty → Industry
	Board    string
}

// TableProvider is an inject/fixture/manual table (real data may be loaded into it offline).
type TableProvider struct {
	// Entries keyed by normalized or raw symbol.
	Entries map[string]TableEntry
	Source  string
	Version string
	Taxonomy string
	AsOf    time.Time
}

// Meta implements Provider.
func (p *TableProvider) Meta() ProviderMeta {
	src := strings.TrimSpace(p.Source)
	if src == "" {
		src = SourceInject
	}
	tax := strings.TrimSpace(p.Taxonomy)
	if tax == "" {
		tax = TaxonomyFixture
	}
	return ProviderMeta{
		Source:   src,
		Version:  strings.TrimSpace(p.Version),
		Taxonomy: tax,
	}
}

// Lookup implements Provider.
func (p *TableProvider) Lookup(symbol string) (Classification, bool) {
	if p == nil || len(p.Entries) == 0 {
		return Classification{}, false
	}
	code := NormSymbol(symbol)
	if code == "" {
		return Classification{}, false
	}
	e, ok := p.lookupEntry(code)
	if !ok {
		return Classification{}, false
	}
	ind := strings.TrimSpace(e.Industry)
	sec := strings.TrimSpace(e.Sector)
	if sec == "" {
		sec = ind
	}
	if IsSentinelLabel(ind) || IsSentinelLabel(sec) {
		return Classification{}, false
	}
	m := p.Meta()
	return Classification{
		Symbol:   code,
		Industry: ind,
		Sector:   sec,
		Board:    strings.TrimSpace(e.Board),
		Source:   m.Source,
		Version:  m.Version,
	}, true
}

// Resolve implements Provider.
func (p *TableProvider) Resolve(codes []string) (map[string]Classification, error) {
	out := map[string]Classification{}
	for _, c := range uniqueNorm(codes) {
		info, ok := p.Lookup(c)
		if !ok {
			continue
		}
		out[c] = info
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func (p *TableProvider) lookupEntry(code string) (TableEntry, bool) {
	if e, ok := p.Entries[code]; ok {
		return e, true
	}
	// also try raw keys normalized once
	for k, e := range p.Entries {
		if NormSymbol(k) == code {
			return e, true
		}
	}
	return TableEntry{}, false
}

// NewTableFromIndustryMap builds a TableProvider from symbol→industry (sector=industry).
func NewTableFromIndustryMap(m map[string]string, source, version, taxonomy string) *TableProvider {
	entries := map[string]TableEntry{}
	for k, v := range m {
		code := NormSymbol(k)
		ind := strings.TrimSpace(v)
		if code == "" || IsSentinelLabel(ind) {
			continue
		}
		entries[code] = TableEntry{Industry: ind, Sector: ind}
	}
	if taxonomy == "" {
		taxonomy = TaxonomyFixture
	}
	if source == "" {
		source = SourceInject
	}
	return &TableProvider{
		Entries:  entries,
		Source:   source,
		Version:  version,
		Taxonomy: taxonomy,
	}
}

// LookupFunc resolves industry + optional board for one symbol.
type LookupFunc func(symbol string) (industry, board string)

// FuncProvider adapts an injected lookup (tests / cache / eastmoney wrapper).
type FuncProvider struct {
	Fn       LookupFunc
	Source   string
	Version  string
	Taxonomy string
	// UseBoardAsSector when true maps board→sector (taxonomy should be eastmoney_bk).
	// Default false: sector = industry only (eastmoney_industry_v1).
	UseBoardAsSector bool
}

// Meta implements Provider.
func (p *FuncProvider) Meta() ProviderMeta {
	src := strings.TrimSpace(p.Source)
	if src == "" {
		src = SourceCache
	}
	tax := strings.TrimSpace(p.Taxonomy)
	if tax == "" {
		if p.UseBoardAsSector {
			tax = "eastmoney_bk_v1"
		} else {
			tax = TaxonomyEastmoneyIndustry
		}
	}
	return ProviderMeta{Source: src, Version: strings.TrimSpace(p.Version), Taxonomy: tax}
}

// Lookup implements Provider.
func (p *FuncProvider) Lookup(symbol string) (Classification, bool) {
	if p == nil || p.Fn == nil {
		return Classification{}, false
	}
	code := NormSymbol(symbol)
	if code == "" {
		return Classification{}, false
	}
	ind, board := p.Fn(code)
	ind = strings.TrimSpace(ind)
	board = strings.TrimSpace(board)
	sec := ind
	if p.UseBoardAsSector {
		sec = board
		if sec == "" {
			sec = ind
		}
	}
	if IsSentinelLabel(ind) && !p.UseBoardAsSector {
		return Classification{}, false
	}
	if IsSentinelLabel(sec) {
		return Classification{}, false
	}
	if sec == "" {
		return Classification{}, false
	}
	// Prefer non-sentinel industry label when sector comes from industry path.
	if !p.UseBoardAsSector && ind == "" {
		return Classification{}, false
	}
	m := p.Meta()
	return Classification{
		Symbol:   code,
		Industry: ind,
		Sector:   sec,
		Board:    board,
		Source:   m.Source,
		Version:  m.Version,
	}, true
}

// Resolve implements Provider.
func (p *FuncProvider) Resolve(codes []string) (map[string]Classification, error) {
	out := map[string]Classification{}
	for _, c := range uniqueNorm(codes) {
		info, ok := p.Lookup(c)
		if !ok {
			continue
		}
		out[c] = info
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}
