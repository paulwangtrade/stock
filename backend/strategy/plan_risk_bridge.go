package strategy

import (
	"fmt"
	"strings"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/risk"
)

// loadPlanFilterContext 组装计划风控快照（strategy 调 data；risk 不访问 DB）。
func loadPlanFilterContext(amount float64, maxNames int) risk.PlanContext {
	cfg := data.GetPaperOpenBuyConfig()
	ctx := risk.PlanContext{
		Enabled:             cfg.EnableRiskFilter,
		MarketLevel:         cfg.PlanMarketLevel,
		BlockNewEntries:     cfg.BlockNewEntriesOnDefense,
		MaxGrossExposurePct: cfg.MaxGrossExposurePct,
		MaxSingleNamePct:    cfg.MaxSingleNamePct,
		MaxDailyLossPct:     cfg.MaxDailyLossPct,
		CurrentDailyPnlPct:  cfg.CurrentDailyPnlPct,
		AmountPerStock:      amount,
		MaxNames:            maxNames,
		ScanLimit:           defaultMaxCandidates,
	}
	if ctx.MarketLevel <= 0 {
		ctx.MarketLevel = 3
	}

	paper := data.NewPaperTradingApi()
	snap, err := paper.GetSnapshot(0)
	if err != nil || snap == nil {
		logger.SugaredLogger.Warnf("plan risk context: paper snapshot unavailable: %v", err)
		ctx.Cash = 0
		ctx.EquityBase = 0
		return ctx
	}
	ctx.Cash = snap.Account.Cash
	ctx.EquityBase = snap.Account.Equity
	nameMV := map[string]float64{}
	longMV := 0.0
	for _, p := range snap.Positions {
		px := p.MarkPrice
		if px <= 0 {
			px = p.AvgCost
		}
		mv := px * float64(p.Volume)
		longMV += mv
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code != "" {
			nameMV[code] = mv
		}
	}
	ctx.LongMarketValue = longMV
	ctx.NameMarketValue = nameMV
	if ctx.EquityBase <= 0 {
		ctx.EquityBase = ctx.Cash + longMV
	}
	return ctx
}

func poolItemsToPlanCandidates(items []models.CandidatePoolItem, amount float64) []risk.PlanCandidate {
	out := make([]risk.PlanCandidate, 0, len(items))
	for _, it := range items {
		out = append(out, risk.PlanCandidate{
			StockCode:       it.StockCode,
			StockName:       it.StockName,
			Rank:            it.Rank,
			Score:           it.Score,
			Reason:          it.Reason,
			StrategyName:    it.StrategyName,
			StrategyVersion: it.StrategyVersion,
			TargetAmount:    amount,
		})
	}
	return out
}

// FilterPoolForTradePlan CandidatePool → risk.PlanFilter（strategy 只编排，不实现规则）。
func FilterPoolForTradePlan(pool *models.CandidatePool, amount float64, maxNames int) (*risk.PlanFilterResult, error) {
	if pool == nil {
		return nil, fmt.Errorf("pool is nil")
	}
	if amount <= 0 {
		amount = 100_000
	}
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}
	ctx := loadPlanFilterContext(amount, maxNames)
	cands := poolItemsToPlanCandidates(pool.Items, amount)
	return risk.PlanFilter(cands, ctx), nil
}
