package projection

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/research"
)

// ProjectOne builds a read-only projection for one stock on a trade date.
func ProjectOne(stockCode string, opts ProjectOptions) (*OpportunityProjection, error) {
	code := normalizeCode(stockCode)
	if code == "" {
		return nil, ErrInvalidStockCode
	}
	ctx, err := loadContext(opts)
	if err != nil {
		return nil, err
	}
	p := buildProjection(ctx, code)
	return &p, nil
}

// ProjectList builds projections for CandidatePool items on a trade date (rank order).
func ProjectList(opts ProjectOptions) ([]OpportunityProjection, error) {
	ctx, err := loadContext(opts)
	if err != nil {
		return nil, err
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	}
	if ctx.pool == nil || len(ctx.pool.Items) == 0 {
		return []OpportunityProjection{}, nil
	}
	out := make([]OpportunityProjection, 0, min(limit, len(ctx.pool.Items)))
	for _, item := range ctx.pool.Items {
		if len(out) >= limit {
			break
		}
		code := normalizeCode(item.StockCode)
		if code == "" {
			continue
		}
		out = append(out, buildProjection(ctx, code))
	}
	return out, nil
}

func loadContext(opts ProjectOptions) (*loadedContext, error) {
	tradeDate := normalizeTradeDate(opts.TradeDate)
	asOf := time.Now()

	ctx := &loadedContext{
		tradeDate:      tradeDate,
		asOf:           asOf,
		poolByCode:     map[string]models.CandidatePoolItem{},
		planByCode:     map[string]models.TradePlanItem{},
		snapshotByID:   map[uint]*models.SignalScanSnapshot{},
		hitsBySnap:     map[uint][]models.SignalScanHit{},
		holdings:       map[string]portfolio.Position{},
		researchByCode: map[string]research.Candidate{},
	}

	poolRepo := data.NewCandidatePoolRepo()
	if pool, err := poolRepo.GetLatestByTradeDate(tradeDate); err == nil && pool != nil {
		ctx.pool = pool
		for _, it := range pool.Items {
			code := normalizeCode(it.StockCode)
			if code != "" {
				ctx.poolByCode[code] = it
			}
		}
	}

	planRepo := data.NewTradePlanRepo()
	var plan *models.TradePlan
	var err error
	if opts.PlanID > 0 {
		plan, err = planRepo.GetByID(opts.PlanID)
	} else {
		plan, err = planRepo.GetLatestByTradeDate(tradeDate)
	}
	if err == nil && plan != nil && isBuyPlan(plan) {
		ctx.plan = plan
		for _, it := range plan.Items {
			code := normalizeCode(it.StockCode)
			if code != "" {
				ctx.planByCode[code] = it
			}
		}
	}

	loadPortfolioHoldings(ctx)
	if opts.IncludeResearch {
		loadResearchOverlay(ctx, tradeDate)
	}
	return ctx, nil
}

func loadPortfolioHoldings(ctx *loadedContext) {
	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{AsOf: ctx.asOf})
	if err != nil || snap == nil || !snap.Found {
		return
	}
	for _, p := range snap.Positions {
		code := normalizeCode(p.StockCode)
		if code == "" || p.Volume <= 0 {
			continue
		}
		ctx.holdings[code] = p
	}
}

func loadResearchOverlay(ctx *loadedContext, tradeDate string) {
	res, err := research.ListCandidates(research.ListQuery{TradeDate: tradeDate})
	if err != nil {
		return
	}
	for _, c := range res.Items {
		code := normalizeCode(c.StockCode)
		if code != "" {
			ctx.researchByCode[code] = c
		}
	}
}

func isBuyPlan(plan *models.TradePlan) bool {
	if plan == nil {
		return false
	}
	side := strings.ToLower(strings.TrimSpace(plan.Side))
	return side == "" || side == "buy"
}

func normalizeTradeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		return raw
	}
	return time.Now().Format("2006-01-02")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func enrichSignalSnapshotMeta(block *SignalBlock, snap *models.SignalScanSnapshot) {
	if block == nil || snap == nil {
		return
	}
	block.SnapshotID = snap.ID
	block.Session = strings.TrimSpace(snap.Session)
	block.StrategyID = strings.TrimSpace(snap.StrategyID)
	if block.SchemaVersion == "" {
		block.SchemaVersion = models.SignalSchemaVersionV1
	}
}

func fetchSnapshot(snapshotID uint) *models.SignalScanSnapshot {
	if db.Dao == nil || snapshotID == 0 {
		return nil
	}
	var snap models.SignalScanSnapshot
	if err := db.Dao.First(&snap, snapshotID).Error; err != nil {
		return nil
	}
	return &snap
}

func fetchSnapshotByTradeDate(tradeDate string) *models.SignalScanSnapshot {
	if db.Dao == nil || strings.TrimSpace(tradeDate) == "" {
		return nil
	}
	var snap models.SignalScanSnapshot
	err := db.Dao.Where("status = ? AND trade_date = ?", "done", strings.TrimSpace(tradeDate)).
		Order("id DESC").
		First(&snap).Error
	if err != nil {
		return nil
	}
	return &snap
}
