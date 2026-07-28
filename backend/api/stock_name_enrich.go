package api

import (
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"
)

// stockNameLookup resolves stock_code → display name (read-only).
type stockNameLookup func(codes []string) map[string]string

// enrichObservationPositionNames fills empty StockName on dashboard position rows (read-only DTO).
// Does not write paper_sim_positions or change observation overlay metrics.
func enrichObservationPositionNames(view *papertrading.DashboardPositions, lookup stockNameLookup) {
	if view == nil || len(view.Positions) == 0 {
		return
	}
	if lookup == nil {
		lookup = defaultStockNameLookup
	}
	need := make([]string, 0, len(view.Positions))
	for i := range view.Positions {
		if strings.TrimSpace(view.Positions[i].StockName) == "" &&
			strings.TrimSpace(view.Positions[i].StockCode) != "" {
			need = append(need, view.Positions[i].StockCode)
		}
	}
	if len(need) == 0 {
		return
	}
	names := lookup(need)
	if len(names) == 0 {
		return
	}
	for i := range view.Positions {
		if strings.TrimSpace(view.Positions[i].StockName) != "" {
			continue
		}
		code := strings.TrimSpace(view.Positions[i].StockCode)
		if name := strings.TrimSpace(names[code]); name != "" {
			view.Positions[i].StockName = name
		}
	}
}

// enrichUpcomingItemNames fills empty StockName fields via lookup. Existing names are kept.
func enrichUpcomingItemNames(plan *UpcomingTradePlanDTO, lookup stockNameLookup) {
	if plan == nil || len(plan.Items) == 0 {
		return
	}
	if lookup == nil {
		lookup = defaultStockNameLookup
	}
	need := make([]string, 0, len(plan.Items))
	for i := range plan.Items {
		if strings.TrimSpace(plan.Items[i].StockName) == "" && strings.TrimSpace(plan.Items[i].StockCode) != "" {
			need = append(need, plan.Items[i].StockCode)
		}
	}
	if len(need) == 0 {
		return
	}
	names := lookup(need)
	if len(names) == 0 {
		return
	}
	for i := range plan.Items {
		if strings.TrimSpace(plan.Items[i].StockName) != "" {
			continue
		}
		code := strings.TrimSpace(plan.Items[i].StockCode)
		if name := strings.TrimSpace(names[code]); name != "" {
			plan.Items[i].StockName = name
		}
	}
}

// defaultStockNameLookup batches CN names from tushare_stock_basic by symbol.
// Silent degrade: nil Dao / query errors → empty map.
func defaultStockNameLookup(codes []string) map[string]string {
	out := map[string]string{}
	if db.Dao == nil || len(codes) == 0 {
		return out
	}

	type codeKey struct {
		code   string
		symbol string
	}
	keys := make([]codeKey, 0, len(codes))
	symbols := make([]string, 0, len(codes))
	seenSym := map[string]bool{}
	for _, raw := range codes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		norm, err := data.NormalizeStockCode(raw)
		if err != nil || norm.Symbol == "" {
			continue
		}
		keys = append(keys, codeKey{code: raw, symbol: norm.Symbol})
		if !seenSym[norm.Symbol] {
			seenSym[norm.Symbol] = true
			symbols = append(symbols, norm.Symbol)
		}
	}
	if len(symbols) == 0 {
		return out
	}

	var rows []data.StockBasic
	if err := db.Dao.Model(&data.StockBasic{}).
		Select("symbol", "name").
		Where("symbol IN ?", symbols).
		Find(&rows).Error; err != nil {
		return out
	}
	bySymbol := map[string]string{}
	for _, row := range rows {
		sym := strings.TrimSpace(row.Symbol)
		name := strings.TrimSpace(row.Name)
		if sym == "" || name == "" {
			continue
		}
		if _, exists := bySymbol[sym]; !exists {
			bySymbol[sym] = name
		}
	}
	for _, k := range keys {
		if name := bySymbol[k.symbol]; name != "" {
			out[k.code] = name
		}
	}
	return out
}
