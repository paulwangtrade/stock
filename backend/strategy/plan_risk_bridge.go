package strategy

import (
	"fmt"
	"strings"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/risk"
	"go-stock/backend/selection"
	"go-stock/backend/tradingconfig"
)

// legacyDecisionProvider is the OFF-path envelope source (G.10). Not Portfolio.
var legacyDecisionProvider decisionprovider.DecisionProvider = decisionprovider.NewLegacyDecisionProvider(nil)

// loadPlanFilterContext 组装计划风控快照（strategy 调 Provider；risk 不访问 DB）。
// Phase6.5-B: Risk thresholds via TradingConfig Provider (legacy_paper_config).
// Phase14-P3-A: account cash/positions via portfolio.Snapshot (paper_sim_*), not paper_accounts.
func loadPlanFilterContext(amount float64, maxNames int) risk.PlanContext {
	tradingconfig.LogRiskInitialized()
	rv := tradingconfig.Default().Risk()
	ctx := risk.PlanContext{
		Enabled:             rv.Enabled,
		MarketLevel:         rv.MarketLevel,
		BlockNewEntries:     rv.BlockNewEntriesOnDefense,
		MaxGrossExposurePct: rv.MaxGrossExposurePct,
		MaxSingleNamePct:    rv.MaxSingleNamePct,
		MaxDailyLossPct:     rv.MaxDailyLossPct,
		CurrentDailyPnlPct:  rv.CurrentDailyPnlPct,
		AmountPerStock:      amount,
		MaxNames:            maxNames,
		ScanLimit:           defaultMaxCandidates,
	}
	if ctx.MarketLevel <= 0 {
		ctx.MarketLevel = 3
	}

	snap, err := portfolio.NewService().Snapshot(portfolio.SnapshotOptions{})
	if err != nil || snap == nil || !snap.Found {
		logger.SugaredLogger.Warnf("plan risk context: portfolio snapshot unavailable: %v", err)
		ctx.Cash = 0
		ctx.EquityBase = 0
		return ctx
	}
	ctx.Cash = snap.Cash
	ctx.EquityBase = snap.TotalEquity
	nameMV := map[string]float64{}
	for _, p := range snap.Positions {
		code := strings.ToLower(strings.TrimSpace(p.StockCode))
		if code == "" || p.Volume <= 0 {
			continue
		}
		nameMV[code] = p.MarketValue
	}
	ctx.LongMarketValue = snap.MarketValue
	ctx.NameMarketValue = nameMV
	if ctx.EquityBase <= 0 {
		ctx.EquityBase = ctx.Cash + ctx.LongMarketValue
	}
	return ctx
}

func poolItemsToSelectionCandidates(items []models.CandidatePoolItem) []selection.Candidate {
	out := make([]selection.Candidate, 0, len(items))
	for _, it := range items {
		out = append(out, selection.Candidate{
			StockCode: it.StockCode,
			StockName: it.StockName,
			Industry:  it.Industry,
			Rank:      it.Rank,
			Score:     it.Score,
			Reason:    it.Reason,
		})
	}
	return out
}

func matchPoolItem(items []models.CandidatePoolItem, c selection.Candidate) models.CandidatePoolItem {
	key := strings.ToLower(strings.TrimSpace(c.StockCode))
	var fallback models.CandidatePoolItem
	found := false
	for _, it := range items {
		if strings.ToLower(strings.TrimSpace(it.StockCode)) != key {
			continue
		}
		if it.Rank == c.Rank {
			return it
		}
		if !found {
			fallback = it
			found = true
		}
	}
	if found {
		return fallback
	}
	return models.CandidatePoolItem{
		StockCode: c.StockCode,
		StockName: c.StockName,
		Rank:      c.Rank,
		Score:     c.Score,
		Reason:    c.Reason,
	}
}

// selectPoolForTradePlan is Phase12-E.8 R1: rank+dedup only. Does not pass PrimaryPicks.
func selectPoolForTradePlan(items []models.CandidatePoolItem, maxNames int) *selection.CandidateSelectionResult {
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}
	return selection.Select(poolItemsToSelectionCandidates(items), selection.SelectionContext{
		MaxSelectedNames: maxNames,
	})
}

// rankedPlanCandidatesFromPool is the compatibility helper: Select → Legacy envelope → PlanCandidate.
// Amount is the OFF-path scalar (Builder Sizer or FilterPool argument). Does not call PlanFilter.
func rankedPlanCandidatesFromPool(items []models.CandidatePoolItem, amount float64, maxNames int) ([]risk.PlanCandidate, int, error) {
	sel := selectPoolForTradePlan(items, maxNames)
	env, err := legacyDecideForPool(sel, amount)
	if err != nil {
		return nil, 0, err
	}
	return planCandidatesFromLegacyEnvelope(items, sel, env), sel.SelectionLimit, nil
}

func legacyDecideForPool(sel *selection.CandidateSelectionResult, amount float64) (*decisionprovider.DecisionEnvelope, error) {
	env, err := legacyDecisionProvider.Decide(decisionprovider.DecisionContext{
		Selection:     sel,
		DecisionTime:  time.Now(),
		Version:       decisionprovider.DecisionVersion{Contract: decisionprovider.ContractG21, Provider: decisionprovider.ProviderFixedAmount},
		UniformAmount: amount,
	})
	if err != nil {
		return nil, err
	}
	if env == nil || !env.OK {
		return nil, fmt.Errorf("legacy decision envelope is not ok")
	}
	if sel != nil && len(env.Lines) != len(sel.RankedCandidates) {
		return nil, fmt.Errorf("legacy envelope lines=%d ranked=%d", len(env.Lines), len(sel.RankedCandidates))
	}
	return env, nil
}

// planCandidatesFromLegacyEnvelope maps a G.2 envelope onto risk.PlanCandidate.
// StockCode/Name/Reason/Strategy* still come from the pool row (matchPoolItem) so
// TradePlan text stays identical to the pre-extraction path. TargetAmount comes
// from the envelope line (same scalar on OFF).
func planCandidatesFromLegacyEnvelope(items []models.CandidatePoolItem, sel *selection.CandidateSelectionResult, env *decisionprovider.DecisionEnvelope) []risk.PlanCandidate {
	if sel == nil || env == nil {
		return nil
	}
	ranked := sel.RankedForPlanFilter()
	out := make([]risk.PlanCandidate, 0, len(ranked))
	for i, c := range ranked {
		src := matchPoolItem(items, c)
		code := strings.TrimSpace(src.StockCode)
		if code == "" {
			code = c.StockCode
		}
		name := strings.TrimSpace(src.StockName)
		if name == "" {
			name = c.StockName
		}
		reason := strings.TrimSpace(src.Reason)
		if reason == "" {
			reason = c.Reason
		}
		amount := 0.0
		if i < len(env.Lines) {
			amount = env.Lines[i].TargetAmount
		}
		out = append(out, risk.PlanCandidate{
			StockCode:       code,
			StockName:       name,
			Rank:            c.Rank,
			Score:           c.Score,
			Reason:          reason,
			StrategyName:    src.StrategyName,
			StrategyVersion: src.StrategyVersion,
			TargetAmount:    amount,
		})
	}
	return out
}

// FilterPoolForTradePlan CandidatePool → Select(R1) → Legacy envelope → risk.PlanFilter.
// strategy 只编排，不实现规则。PlanFilter 公式不变。
// ScanLimit stays on PlanContext (defaultMaxCandidates); selected=true is not a gate.
// This debug/shared entry does not run Provider Shadow (after-close Draft uses filterPoolWithSelection after Select).
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
	sel := selectPoolForTradePlan(pool.Items, maxNames)
	return filterPoolWithSelection(pool.Items, sel, amount, maxNames)
}

// filterPoolWithSelection maps the already-selected ranked list through Legacy
// and PlanFilter. Callers that need G.12 shadow must Observe between Select and here.
func filterPoolWithSelection(items []models.CandidatePoolItem, sel *selection.CandidateSelectionResult, amount float64, maxNames int) (*risk.PlanFilterResult, error) {
	if amount <= 0 {
		amount = 100_000
	}
	if maxNames <= 0 {
		maxNames = defaultMaxPlanNames
	}
	ctx := planFilterContextFn(amount, maxNames)
	env, err := legacyDecideForPool(sel, amount)
	if err != nil {
		return nil, err
	}
	cands := planCandidatesFromLegacyEnvelope(items, sel, env)
	if sel != nil {
		ctx.MaxNames = sel.SelectionLimit
	}
	return risk.PlanFilter(cands, ctx), nil
}
