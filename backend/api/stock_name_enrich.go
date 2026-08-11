package api

import (
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/papertrading"
	"go-stock/backend/stockname"
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

// defaultStockNameLookup uses stockname.Resolve (not tushare-only).
// Unknown sentinel is omitted so the UI can show 未知名称(code).
func defaultStockNameLookup(codes []string) map[string]string {
	out := map[string]string{}
	if db.Dao == nil || len(codes) == 0 {
		return out
	}
	for _, raw := range codes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		res := stockname.ResolveWithDB(db.Dao, raw, stockname.Hint{})
		name := strings.TrimSpace(res.Name)
		if name == "" || name == stockname.UnknownName {
			continue
		}
		out[raw] = name
	}
	return out
}
