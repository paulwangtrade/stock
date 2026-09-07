package outcome

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"
	"go-stock/backend/papertrading"
)

func loadFillsForStock(accountID uint, stockCode string) ([]papertrading.PaperSimFill, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("outcome: db not initialized")
	}
	code := normalizeCode(stockCode)
	if code == "" {
		return nil, ErrInvalidStockCode
	}
	var fills []papertrading.PaperSimFill
	err := db.Dao.Where("account_id = ? AND LOWER(TRIM(stock_code)) = ?", accountID, code).
		Order("filled_at asc, id asc").
		Find(&fills).Error
	return fills, err
}

type fillContext struct {
	orders     map[uint]papertrading.PaperSimOrder
	plans      map[uint]*models.TradePlan
	items      map[uint]models.TradePlanItem
	poolItems  map[string]models.CandidatePoolItem
	pools      map[uint]*models.CandidatePool
	snapshots  map[uint]*models.SignalScanSnapshot
}

func loadFillContext(fills []papertrading.PaperSimFill) (*fillContext, error) {
	ctx := &fillContext{
		orders:    map[uint]papertrading.PaperSimOrder{},
		plans:     map[uint]*models.TradePlan{},
		items:     map[uint]models.TradePlanItem{},
		poolItems: map[string]models.CandidatePoolItem{},
		pools:     map[uint]*models.CandidatePool{},
		snapshots: map[uint]*models.SignalScanSnapshot{},
	}
	if db.Dao == nil || len(fills) == 0 {
		return ctx, nil
	}

	orderIDs := uniqueUintIDs(fills, func(f papertrading.PaperSimFill) uint { return f.OrderID })
	planIDs := uniqueUintIDs(fills, func(f papertrading.PaperSimFill) uint { return f.PlanID })
	itemIDs := uniqueUintIDs(fills, func(f papertrading.PaperSimFill) uint { return f.PlanItemID })

	if len(orderIDs) > 0 {
		var orders []papertrading.PaperSimOrder
		if err := db.Dao.Where("id IN ?", orderIDs).Find(&orders).Error; err != nil {
			return nil, err
		}
		for _, o := range orders {
			ctx.orders[o.ID] = o
		}
	}
	if len(planIDs) > 0 {
		planRepo := data.NewTradePlanRepo()
		for _, pid := range planIDs {
			plan, err := planRepo.GetByID(pid)
			if err != nil || plan == nil {
				continue
			}
			ctx.plans[pid] = plan
			if plan.PoolID > 0 {
				if _, ok := ctx.pools[plan.PoolID]; !ok {
					pool, perr := data.NewCandidatePoolRepo().GetByID(plan.PoolID)
					if perr == nil && pool != nil {
						ctx.pools[plan.PoolID] = pool
						for _, it := range pool.Items {
							code := normalizeCode(it.StockCode)
							if code != "" {
								ctx.poolItems[code] = it
							}
						}
					}
				}
			}
		}
	}
	if len(itemIDs) > 0 {
		var items []models.TradePlanItem
		if err := db.Dao.Where("id IN ?", itemIDs).Find(&items).Error; err != nil {
			return nil, err
		}
		for _, it := range items {
			ctx.items[it.ID] = it
		}
	}
	return ctx, nil
}

func (c *fillContext) entryDate(buy papertrading.PaperSimFill) string {
	if o, ok := c.orders[buy.OrderID]; ok && strings.TrimSpace(o.TradeDate) != "" {
		return strings.TrimSpace(o.TradeDate)
	}
	if p, ok := c.plans[buy.PlanID]; ok && strings.TrimSpace(p.TradeDate) != "" {
		return strings.TrimSpace(p.TradeDate)
	}
	return buy.FilledAt.Format("2006-01-02")
}

func (c *fillContext) exitDate(sell papertrading.PaperSimFill) string {
	if o, ok := c.orders[sell.OrderID]; ok && strings.TrimSpace(o.TradeDate) != "" {
		return strings.TrimSpace(o.TradeDate)
	}
	if p, ok := c.plans[sell.PlanID]; ok && strings.TrimSpace(p.TradeDate) != "" {
		return strings.TrimSpace(p.TradeDate)
	}
	return sell.FilledAt.Format("2006-01-02")
}

func uniqueUintIDs(fills []papertrading.PaperSimFill, fn func(papertrading.PaperSimFill) uint) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, f := range fills {
		id := fn(f)
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func normalizeCode(raw string) string {
	return opportunity.NormalizeReadStockCode(raw)
}
