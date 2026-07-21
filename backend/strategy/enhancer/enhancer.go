package enhancer

// EnhanceContext 增强上下文（只读查询结果由调用方注入，避免 enhancer 直连扫描）。
type EnhanceContext struct {
	TradeDate string
}

// CandidateScore 评分内部结构（不落库独立列）；FinalScore 写入 CandidatePoolItem.Score。
type CandidateScore struct {
	StrategyScore float64
	SignalScore   float64
	FinalScore    float64
}

// ComposeCandidateScore 按固定权重合成最终分。
func ComposeCandidateScore(strategyScore, signalScore float64) CandidateScore {
	return CandidateScore{
		StrategyScore: strategyScore,
		SignalScore:   signalScore,
		FinalScore:    StrategyWeight*strategyScore + SignalWeight*signalScore,
	}
}

// CandidateItem 候选中间态（落库前）；StrategyScore 仅内存，不落库。
type CandidateItem struct {
	StockCode        string
	StockName        string
	Industry         string
	Reason           string
	StrategyName     string
	StrategyVersion  string
	StrategyScore    float64
	SignalTag        string
	SignalScore      float64
	SignalSnapshotID uint
	Score            float64 // = CandidateScore.FinalScore
	Rank             int
}

// ApplyScore 将 CandidateScore 写回 item（Score=FinalScore，SignalScore=分量）。
func (c *CandidateItem) ApplyScore(cs CandidateScore) {
	c.StrategyScore = cs.StrategyScore
	c.SignalScore = cs.SignalScore
	c.Score = cs.FinalScore
}

// Enhancer 可插拔候选增强（信号 / 未来基本面等）。
type Enhancer interface {
	Enhance(items []CandidateItem, ctx EnhanceContext) ([]CandidateItem, error)
}
