package opportunity

import (
	"strings"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// Watchlist trade_plan_status display enum (API projection; not persisted).
const (
	WatchlistTradePlanNone     = "NONE"
	WatchlistTradePlanDraft    = "DRAFT"
	WatchlistTradePlanReady    = "READY"
	WatchlistTradePlanExecuted = "EXECUTED"
)

type watchlistPlanHit struct {
	plan models.TradePlan
	item models.TradePlanItem
}

// enrichWatchlistTradePlan sets in_trade_plan / trade_plan_status / trade_plan_id via stock_code match.
// Skipped items are ignored. Multiple plans → prefer non-terminal, then trade_date, then plan id.
// Read-only projection only; does not mutate TradePlan rows.
func enrichWatchlistTradePlan(items []WatchlistItem) {
	if len(items) == 0 || db.Dao == nil {
		return
	}
	for i := range items {
		if items[i].TradePlanStatus == "" {
			items[i].TradePlanStatus = WatchlistTradePlanNone
		}
		items[i].TradePlanID = 0
	}

	codeSet := map[string]struct{}{}
	codes := make([]string, 0, len(items))
	for _, it := range items {
		c := NormalizeStockCode(it.StockCode)
		if c == "" {
			continue
		}
		if _, ok := codeSet[c]; ok {
			continue
		}
		codeSet[c] = struct{}{}
		codes = append(codes, c)
	}
	if len(codes) == 0 {
		return
	}

	var planItems []models.TradePlanItem
	err := db.Dao.Where("stock_code IN ? AND status <> ?", codes, models.TradePlanItemSkipped).
		Find(&planItems).Error
	if err != nil || len(planItems) == 0 {
		return
	}

	planIDSet := map[uint]struct{}{}
	planIDs := make([]uint, 0)
	for _, it := range planItems {
		if it.PlanID == 0 {
			continue
		}
		if _, ok := planIDSet[it.PlanID]; ok {
			continue
		}
		planIDSet[it.PlanID] = struct{}{}
		planIDs = append(planIDs, it.PlanID)
	}
	if len(planIDs) == 0 {
		return
	}

	var plans []models.TradePlan
	if err := db.Dao.Where("id IN ?", planIDs).Find(&plans).Error; err != nil {
		return
	}
	planByID := make(map[uint]models.TradePlan, len(plans))
	for _, p := range plans {
		planByID[p.ID] = p
	}

	bestByCode := map[string]watchlistPlanHit{}
	for _, it := range planItems {
		plan, ok := planByID[it.PlanID]
		if !ok {
			continue
		}
		code := NormalizeStockCode(it.StockCode)
		if code == "" {
			continue
		}
		hit := watchlistPlanHit{plan: plan, item: it}
		prev, exists := bestByCode[code]
		if !exists || watchlistPlanHitBetter(hit, prev) {
			bestByCode[code] = hit
		}
	}

	for i := range items {
		code := NormalizeStockCode(items[i].StockCode)
		hit, ok := bestByCode[code]
		if !ok {
			items[i].InTradePlan = false
			items[i].TradePlanStatus = WatchlistTradePlanNone
			items[i].TradePlanID = 0
			continue
		}
		items[i].InTradePlan = true
		items[i].TradePlanStatus = mapWatchlistTradePlanStatus(hit.plan.Status)
		items[i].TradePlanID = hit.plan.ID
	}
}

// FindRepresentativeTradePlanForCode returns the same representative plan used by watchlist enrich.
// ok=false when no non-skipped item matches. Read-only.
func FindRepresentativeTradePlanForCode(stockCode string) (models.TradePlan, bool) {
	code := NormalizeStockCode(stockCode)
	if code == "" || db.Dao == nil {
		return models.TradePlan{}, false
	}
	items := []WatchlistItem{{StockCode: code, TradePlanStatus: WatchlistTradePlanNone}}
	enrichWatchlistTradePlan(items)
	if !items[0].InTradePlan || items[0].TradePlanID == 0 {
		return models.TradePlan{}, false
	}
	var plan models.TradePlan
	if err := db.Dao.First(&plan, items[0].TradePlanID).Error; err != nil {
		return models.TradePlan{}, false
	}
	return plan, true
}

func watchlistPlanHitBetter(a, b watchlistPlanHit) bool {
	aActive := !a.plan.IsTerminal()
	bActive := !b.plan.IsTerminal()
	if aActive != bActive {
		return aActive
	}
	aDate := strings.TrimSpace(a.plan.TradeDate)
	bDate := strings.TrimSpace(b.plan.TradeDate)
	if aDate != bDate {
		return aDate > bDate
	}
	return a.plan.ID > b.plan.ID
}

func mapWatchlistTradePlanStatus(raw string) string {
	switch strings.TrimSpace(raw) {
	case models.TradePlanStatusDraft:
		return WatchlistTradePlanDraft
	case models.TradePlanStatusReady:
		return WatchlistTradePlanReady
	case models.TradePlanStatusExecuting,
		models.TradePlanStatusDone,
		models.TradePlanStatusPartial,
		models.TradePlanStatusFailed,
		models.TradePlanStatusSuperseded,
		models.TradePlanStatusSkipped:
		return WatchlistTradePlanExecuted
	default:
		if raw == "" {
			return WatchlistTradePlanNone
		}
		return WatchlistTradePlanExecuted
	}
}
