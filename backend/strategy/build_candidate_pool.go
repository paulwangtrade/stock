package strategy

import (
	"encoding/json"
	"sort"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"go-stock/backend/strategy/enhancer"
)

// BuildCandidatePoolOption extends the persisted build snapshot without changing
// the CandidatePool model or the scoring pipeline.
type BuildCandidatePoolOption func(map[string]any)

// WithCandidatePoolConfig merges observability metadata into ConfigJSON.
func WithCandidatePoolConfig(config map[string]any) BuildCandidatePoolOption {
	return func(target map[string]any) {
		for key, value := range config {
			target[key] = value
		}
	}
}

// BuildCandidatePool 按交易日生成候选池并落库（同日可多次，每次新建版本）。
// 流程：Universe → items → strategyScore → SignalSnapshotEnhancer → Score → 排序 → Rank → 截断 → 保存
func BuildCandidatePool(tradeDate string, options ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
	tradeDate = normalizeTradeDate(tradeDate)
	uni := collectUniverse()

	items := make([]enhancer.CandidateItem, 0, len(uni.Items))
	n := len(uni.Items)
	for i, c := range uni.Items {
		items = append(items, enhancer.CandidateItem{
			StockCode:       c.StockCode,
			StockName:       c.StockName,
			Industry:        c.Industry,
			Reason:          c.Reason,
			StrategyName:    c.StrategyName,
			StrategyVersion: c.StrategyVersion,
			StrategyScore:   strategyScoreFromRank(i+1, n),
		})
	}

	enh := enhancer.NewSignalSnapshotEnhancer()
	enhanced, err := enh.Enhance(items, enhancer.EnhanceContext{TradeDate: tradeDate})
	if err != nil {
		logger.SugaredLogger.Warnf("signal enhancer error (continue without signal): %v", err)
		enhanced = items
		for i := range enhanced {
			enhanced[i].SignalTag = ""
			enhanced[i].SignalSnapshotID = 0
			enhanced[i].ApplyScore(enhancer.ComposeCandidateScore(enhanced[i].StrategyScore, 0))
		}
	}

	sort.SliceStable(enhanced, func(i, j int) bool {
		if enhanced[i].Score == enhanced[j].Score {
			return enhanced[i].StrategyScore > enhanced[j].StrategyScore
		}
		return enhanced[i].Score > enhanced[j].Score
	})
	for i := range enhanced {
		enhanced[i].Rank = i + 1
	}

	beforeCut := len(enhanced)
	enhanced = truncateEnhancerItems(enhanced, defaultMaxCandidates)

	var snapshotID uint
	enhancedFlag := false
	for _, it := range enhanced {
		if it.SignalSnapshotID > 0 {
			snapshotID = it.SignalSnapshotID
			enhancedFlag = true
			break
		}
	}

	config := map[string]any{
		"maxCandidates":    defaultMaxCandidates,
		"excludeST":        true,
		"source":           uni.Source,
		"signalEnhanced":   enhancedFlag,
		"signalSnapshotId": snapshotID,
		"strategyWeight":   enhancer.StrategyWeight,
		"signalWeight":     enhancer.SignalWeight,
	}
	for _, option := range options {
		if option != nil {
			option(config)
		}
	}

	// M1-B: directed universe signal snapshot AFTER rank/truncate, BEFORE persist.
	// Failure → Warn only; score/rank already frozen and unchanged.
	_ = applyUniverseSignalSnapshot(uni, tradeDate, config)

	cfgSnap, _ := json.Marshal(config)

	pool := &models.CandidatePool{
		TradeDate:   tradeDate,
		GeneratedAt: time.Now(),
		Source:      uni.Source,
		SourceRef:   uni.SourceRef,
		Status:      models.CandidatePoolStatusReady,
		Message:     uni.Message,
		ConfigJSON:  string(cfgSnap),
	}

	poolItems := make([]models.CandidatePoolItem, 0, len(enhanced))
	for _, c := range enhanced {
		poolItems = append(poolItems, models.CandidatePoolItem{
			TradeDate:        tradeDate,
			StockCode:        c.StockCode,
			StockName:        c.StockName,
			Rank:             c.Rank,
			Score:            c.Score,
			Reason:           c.Reason,
			StrategyName:     c.StrategyName,
			StrategyVersion:  c.StrategyVersion,
			Industry:         c.Industry,
			SignalTag:        c.SignalTag,
			SignalScore:      c.SignalScore,
			SignalSnapshotID: c.SignalSnapshotID,
		})
	}

	if len(poolItems) == 0 {
		pool.Status = models.CandidatePoolStatusFailed
		pool.Message = "no candidates after filter"
	}

	if err := data.NewCandidatePoolRepo().CreatePoolWithItems(pool, poolItems); err != nil {
		return nil, err
	}

	applyPoolProvenanceLink(pool, poolItems)

	logCandidatePoolSummary(pool, uni.Source, snapshotID, enhancedFlag, beforeCut, enhanced)
	return pool, nil
}

func strategyScoreFromRank(rank, total int) float64 {
	if total <= 0 || rank <= 0 {
		return 0
	}
	// rank1=1.0, 末位趋近 1/total
	return float64(total-rank+1) / float64(total)
}

func truncateEnhancerItems(items []enhancer.CandidateItem, max int) []enhancer.CandidateItem {
	if max <= 0 {
		max = defaultMaxCandidates
	}
	if len(items) <= max {
		return items
	}
	return items[:max]
}

func logCandidatePoolSummary(pool *models.CandidatePool, source string, snapshotID uint, enhanced bool, beforeCut int, top []enhancer.CandidateItem) {
	total := pool.ItemCount
	if total == 0 {
		total = len(top)
	}
	logger.SugaredLogger.Infof("candidate pool: total=%d source=%s snapshot_id=%d enhanced=%v before_cut=%d poolId=%d",
		total, source, snapshotID, enhanced, beforeCut, pool.ID)
	limit := 5
	if len(top) < limit {
		limit = len(top)
	}
	for i := 0; i < limit; i++ {
		it := top[i]
		sig := it.SignalTag
		if sig == "" {
			sig = "-"
		}
		strat := it.StrategyName
		if strat == "" {
			strat = "-"
		}
		logger.SugaredLogger.Infof("top: %d. %s strategy=%s signal=%s score=%.2f",
			i+1, it.StockCode, strat, sig, it.Score)
	}
}
