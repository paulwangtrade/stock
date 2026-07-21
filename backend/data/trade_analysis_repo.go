package data

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
)

// TradePlanAnalysis 单日交易链观测（只读，不改交易逻辑）。
type TradePlanAnalysis struct {
	TradeDate string              `json:"tradeDate"`
	Pool      *AnalysisPoolInfo   `json:"pool,omitempty"`
	Plan      *AnalysisPlanInfo   `json:"plan,omitempty"`
	Items     []AnalysisChainItem `json:"items"`
	Message   string              `json:"message,omitempty"`
}

type AnalysisPoolInfo struct {
	ID        uint   `json:"id"`
	Source    string `json:"source"`
	SourceRef string `json:"sourceRef"`
	Status    string `json:"status"`
	ItemCount int    `json:"itemCount"`
	Message   string `json:"message"`
}

type AnalysisPlanInfo struct {
	ID                uint    `json:"id"`
	Status            string  `json:"status"`
	PoolID            uint    `json:"poolId"`
	RiskStatus        string  `json:"riskStatus"`
	MarketLevel       int     `json:"marketLevel"`
	RiskAcceptedCount int     `json:"riskAcceptedCount"`
	RiskFilteredCount int     `json:"riskFilteredCount"`
	RiskSummary       string  `json:"riskSummary"`
	AmountPerStock    float64 `json:"amountPerStock"`
	EnableExecute     bool    `json:"enableExecute"`
	Message           string  `json:"message"`
}

// AnalysisChainItem 单票全链路：池评分 → 风控 → 执行/订单。
type AnalysisChainItem struct {
	StockCode        string  `json:"stockCode"`
	StockName        string  `json:"stockName"`
	InCandidatePool  bool    `json:"inCandidatePool"`
	PoolRank         int     `json:"poolRank,omitempty"`
	Score            float64 `json:"score"`
	StrategyName     string  `json:"strategyName"`
	StrategyVersion  string  `json:"strategyVersion"`
	SignalTag        string  `json:"signalTag"`
	SignalScore      float64 `json:"signalScore"`
	SignalSnapshotID uint    `json:"signalSnapshotId"`
	PlanPriority     int     `json:"planPriority,omitempty"`
	PlanStatus       string  `json:"planStatus,omitempty"`
	RiskCode         string  `json:"riskCode,omitempty"`
	RiskMessage      string  `json:"riskMessage,omitempty"`
	TargetAmount     float64 `json:"targetAmount,omitempty"`
	OrderID          uint    `json:"orderId,omitempty"`
	FillID           uint    `json:"fillId,omitempty"`
	FilledPrice      float64 `json:"filledPrice,omitempty"`
	FilledVolume     int64   `json:"filledVolume,omitempty"`
	FilledFee        float64 `json:"filledFee,omitempty"`
	Error            string  `json:"error,omitempty"`
	WhyNotBought     string  `json:"whyNotBought,omitempty"`
}

// StrategyPerformanceRow 策略×版本×信号 聚合（pnl 预留为空）。
type StrategyPerformanceRow struct {
	StrategyName    string  `json:"strategyName"`
	StrategyVersion string  `json:"strategyVersion"`
	SignalTag       string  `json:"signalTag"`
	CandidateCount  int     `json:"candidateCount"`
	PlanCount       int     `json:"planCount"`
	ExecutedCount   int     `json:"executedCount"`
	SkippedCount    int     `json:"skippedCount"`
	WinCount        int     `json:"winCount"`
	LossCount       int     `json:"lossCount"`
	PnL             float64 `json:"pnl"`
}

// TradeAnalysisRepo 交易链只读分析。
type TradeAnalysisRepo struct{}

func NewTradeAnalysisRepo() *TradeAnalysisRepo { return &TradeAnalysisRepo{} }

// GetTradePlanAnalysis 查询某日最新计划链路（池→计划→风控→订单关联）。
func (r *TradeAnalysisRepo) GetTradePlanAnalysis(date string) (*TradePlanAnalysis, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	date = strings.TrimSpace(date)
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	out := &TradePlanAnalysis{TradeDate: date, Items: []AnalysisChainItem{}}

	plan, err := NewTradePlanRepo().GetLatestByTradeDate(date)
	if err != nil {
		pool, perr := NewCandidatePoolRepo().GetLatestByTradeDate(date)
		if perr != nil {
			out.Message = fmt.Sprintf("no trade plan or candidate pool for %s", date)
			return out, nil
		}
		out.Pool = poolInfoFrom(pool)
		for _, c := range pool.Items {
			out.Items = append(out.Items, AnalysisChainItem{
				StockCode: c.StockCode, StockName: c.StockName, InCandidatePool: true,
				PoolRank: c.Rank, Score: c.Score,
				StrategyName: c.StrategyName, StrategyVersion: c.StrategyVersion,
				SignalTag: c.SignalTag, SignalScore: c.SignalScore, SignalSnapshotID: c.SignalSnapshotID,
				WhyNotBought: "未进入 TradePlan（当日无交易计划）",
			})
		}
		out.Message = "candidate pool only; no trade plan"
		return out, nil
	}

	out.Plan = &AnalysisPlanInfo{
		ID: plan.ID, Status: plan.Status, PoolID: plan.PoolID,
		RiskStatus: plan.RiskStatus, MarketLevel: plan.MarketLevel,
		RiskAcceptedCount: plan.RiskAcceptedCount, RiskFilteredCount: plan.RiskFilteredCount,
		RiskSummary: plan.RiskSummary, AmountPerStock: plan.AmountPerStock,
		EnableExecute: plan.EnableExecute, Message: plan.Message,
	}

	poolByCode := map[string]models.CandidatePoolItem{}
	if plan.PoolID > 0 {
		if pool, perr := NewCandidatePoolRepo().GetByID(plan.PoolID); perr == nil && pool != nil {
			out.Pool = poolInfoFrom(pool)
			for _, c := range pool.Items {
				poolByCode[strings.ToLower(strings.TrimSpace(c.StockCode))] = c
			}
		}
	}

	seen := map[string]bool{}
	for _, it := range plan.Items {
		key := strings.ToLower(strings.TrimSpace(it.StockCode))
		seen[key] = true
		row := AnalysisChainItem{
			StockCode: it.StockCode, StockName: it.StockName,
			PlanPriority: it.Priority, PlanStatus: it.Status,
			RiskCode: it.RiskCode, RiskMessage: it.RiskMessage,
			TargetAmount: it.TargetAmount,
			OrderID: it.OrderID, FillID: it.FillID,
			FilledPrice: it.FilledPrice, FilledVolume: it.FilledVolume, FilledFee: it.FilledFee,
			Error: it.Error,
			Score: it.Score, StrategyName: it.StrategyName, StrategyVersion: it.StrategyVersion,
		}
		if c, ok := poolByCode[key]; ok {
			row.InCandidatePool = true
			row.PoolRank = c.Rank
			row.Score = c.Score
			row.StrategyName = c.StrategyName
			row.StrategyVersion = c.StrategyVersion
			row.SignalTag = c.SignalTag
			row.SignalScore = c.SignalScore
			row.SignalSnapshotID = c.SignalSnapshotID
			if row.StockName == "" {
				row.StockName = c.StockName
			}
		}
		row.WhyNotBought = explainWhyNotBought(row)
		out.Items = append(out.Items, row)
	}

	for _, c := range poolByCode {
		key := strings.ToLower(strings.TrimSpace(c.StockCode))
		if seen[key] {
			continue
		}
		out.Items = append(out.Items, AnalysisChainItem{
			StockCode: c.StockCode, StockName: c.StockName, InCandidatePool: true,
			PoolRank: c.Rank, Score: c.Score,
			StrategyName: c.StrategyName, StrategyVersion: c.StrategyVersion,
			SignalTag: c.SignalTag, SignalScore: c.SignalScore, SignalSnapshotID: c.SignalSnapshotID,
			WhyNotBought: "在 CandidatePool 中但未进入 TradePlan（未录取/未扫描）",
		})
	}

	out.Message = fmt.Sprintf("planId=%d poolId=%d items=%d", plan.ID, plan.PoolID, len(out.Items))
	return out, nil
}

func poolInfoFrom(pool *models.CandidatePool) *AnalysisPoolInfo {
	if pool == nil {
		return nil
	}
	return &AnalysisPoolInfo{
		ID: pool.ID, Source: pool.Source, SourceRef: pool.SourceRef,
		Status: pool.Status, ItemCount: pool.ItemCount, Message: pool.Message,
	}
}

func explainWhyNotBought(row AnalysisChainItem) string {
	switch row.PlanStatus {
	case models.TradePlanItemFilled:
		return ""
	case models.TradePlanItemSkipped:
		if row.RiskCode != "" && row.RiskCode != "APPROVED" {
			msg := strings.TrimSpace(row.RiskMessage)
			if msg != "" {
				return fmt.Sprintf("RiskFilter: %s %s", row.RiskCode, msg)
			}
			return fmt.Sprintf("RiskFilter: %s", row.RiskCode)
		}
		if row.Error != "" {
			return row.Error
		}
		return "skipped"
	case models.TradePlanItemError:
		if row.Error != "" {
			return row.Error
		}
		return "execution error"
	case models.TradePlanItemPending:
		return "pending（尚未执行）"
	default:
		if !row.InCandidatePool {
			return "没有进入 CandidatePool"
		}
		return ""
	}
}

// GetStrategyPerformance 按策略名/版本/信号标签聚合（win/loss/pnl 预留 0）。
func (r *TradeAnalysisRepo) GetStrategyPerformance() ([]StrategyPerformanceRow, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}

	type key struct{ Name, Ver, Tag string }
	agg := map[key]*StrategyPerformanceRow{}
	ensure := func(name, ver, tag string) *StrategyPerformanceRow {
		k := key{name, ver, tag}
		if agg[k] == nil {
			agg[k] = &StrategyPerformanceRow{
				StrategyName: name, StrategyVersion: ver, SignalTag: tag,
			}
		}
		return agg[k]
	}

	var poolItems []models.CandidatePoolItem
	if err := db.Dao.Find(&poolItems).Error; err != nil {
		return nil, err
	}
	for _, it := range poolItems {
		ensure(it.StrategyName, it.StrategyVersion, it.SignalTag).CandidateCount++
	}

	var plans []models.TradePlan
	_ = db.Dao.Find(&plans)
	planPool := map[uint]uint{}
	for _, p := range plans {
		planPool[p.ID] = p.PoolID
	}

	poolByIDCode := map[string]models.CandidatePoolItem{}
	for _, c := range poolItems {
		k := fmt.Sprintf("%d|%s", c.PoolID, strings.ToLower(strings.TrimSpace(c.StockCode)))
		poolByIDCode[k] = c
	}

	var planItems []models.TradePlanItem
	if err := db.Dao.Find(&planItems).Error; err != nil {
		return nil, err
	}
	for _, it := range planItems {
		tag := ""
		if poolID := planPool[it.PlanID]; poolID > 0 {
			k := fmt.Sprintf("%d|%s", poolID, strings.ToLower(strings.TrimSpace(it.StockCode)))
			if c, ok := poolByIDCode[k]; ok {
				tag = c.SignalTag
			}
		}
		row := ensure(it.StrategyName, it.StrategyVersion, tag)
		row.PlanCount++
		switch it.Status {
		case models.TradePlanItemFilled:
			row.ExecutedCount++
		case models.TradePlanItemSkipped:
			row.SkippedCount++
		}
	}

	out := make([]StrategyPerformanceRow, 0, len(agg))
	for _, v := range agg {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		as := a.StrategyName + "|" + a.StrategyVersion + "|" + a.SignalTag
		bs := b.StrategyName + "|" + b.StrategyVersion + "|" + b.SignalTag
		return as < bs
	})
	return out, nil
}

// GetTodayTradeAnalysis 当日交易链（TEMP/观测）。
func GetTodayTradeAnalysis() (*TradePlanAnalysis, error) {
	return NewTradeAnalysisRepo().GetTradePlanAnalysis(todayTradeDateLocal())
}

// GetStrategyPerformance 包级便捷入口。
func GetStrategyPerformance() ([]StrategyPerformanceRow, error) {
	return NewTradeAnalysisRepo().GetStrategyPerformance()
}
