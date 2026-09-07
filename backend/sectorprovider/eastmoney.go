package sectorprovider

import "go-stock/backend/data"

// NewEastmoneyCachedProvider wraps StockDataApi.LookupStockIndustrySectorCached.
//
// Mapping (taxonomy eastmoney_industry_v1):
//   - Industry = 东财 INDUSTRY
//   - Sector   = Industry（板块 BKName 仅写入 Board，不混作 sector）
//
// Fail-closed: empty / sentinel industry → Lookup ok=false（不填 unknown）。
// This constructor is for offline / Shadow / Sim injection only — not wired to Draft.
func NewEastmoneyCachedProvider(version string) *FuncProvider {
	api := data.NewStockDataApi()
	return &FuncProvider{
		Fn: func(symbol string) (industry, board string) {
			return api.LookupStockIndustrySectorCached(symbol)
		},
		Source:   SourceEastmoney,
		Version:  version,
		Taxonomy: TaxonomyEastmoneyIndustry,
	}
}

// WrapEastmoneyLookup adapts an existing StockDataApi instance.
func WrapEastmoneyLookup(api *data.StockDataApi, version string) *FuncProvider {
	if api == nil {
		api = data.NewStockDataApi()
	}
	return &FuncProvider{
		Fn: func(symbol string) (industry, board string) {
			return api.LookupStockIndustrySectorCached(symbol)
		},
		Source:   SourceEastmoney,
		Version:  version,
		Taxonomy: TaxonomyEastmoneyIndustry,
	}
}
