package strategy

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/risk"
	"go-stock/backend/strategy/enhancer"
)

// 流水线单元测试：Candidate → SignalEnhancer → RiskFilter → TradePlan items（不落库、不改规则）。

type pipelineSnapStub struct {
	snap *models.SignalScanSnapshot
	hits []models.SignalScanHit
}

func (s *pipelineSnapStub) GetLatestCloseSignalSnapshot(asOfDate string) (*models.SignalScanSnapshot, error) {
	return s.snap, nil
}

func (s *pipelineSnapStub) ParseSnapshotHits(snap *models.SignalScanSnapshot) []models.SignalScanHit {
	return s.hits
}

func enhanceThenFilter(t *testing.T, items []enhancer.CandidateItem, stub *pipelineSnapStub, ctx risk.PlanContext) *risk.PlanFilterResult {
	t.Helper()
	enh := &enhancer.SignalSnapshotEnhancer{Source: stub}
	enhanced, err := enh.Enhance(items, enhancer.EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatalf("enhance: %v", err)
	}
	cands := make([]risk.PlanCandidate, 0, len(enhanced))
	for _, it := range enhanced {
		cands = append(cands, risk.PlanCandidate{
			StockCode: it.StockCode, StockName: it.StockName,
			Score: it.Score, StrategyName: it.StrategyName, StrategyVersion: it.StrategyVersion,
			TargetAmount: ctx.AmountPerStock,
		})
	}
	return risk.PlanFilter(cands, ctx)
}

func TestPipeline_Case1_NormalStockEntersPlan(t *testing.T) {
	items := []enhancer.CandidateItem{
		{StockCode: "sz000001", StrategyName: "s1", StrategyVersion: "v1", StrategyScore: 0.9},
		{StockCode: "sh600519", StrategyName: "s1", StrategyVersion: "v1", StrategyScore: 0.8},
	}
	stub := &pipelineSnapStub{snap: nil}
	ctx := risk.PlanContext{
		Enabled: true, MarketLevel: 3, BlockNewEntries: true,
		Cash: 1_000_000, EquityBase: 1_000_000,
		AmountPerStock: 100_000, MaxNames: 2,
		MaxGrossExposurePct: 0.85, MaxSingleNamePct: 0.20, ScanLimit: 2,
	}
	res := enhanceThenFilter(t, items, stub, ctx)
	if res.AcceptedCount != 2 || res.FilteredCount != 0 {
		t.Fatalf("accepted=%d filtered=%d", res.AcceptedCount, res.FilteredCount)
	}
	for _, it := range res.Items {
		if it.Status != "pending" {
			t.Fatalf("want pending got %+v", it)
		}
	}
}

func TestPipeline_Case2_RiskRejectsStock(t *testing.T) {
	items := []enhancer.CandidateItem{
		{StockCode: "sz000001", StrategyName: "s1", StrategyVersion: "v1", StrategyScore: 1.0},
	}
	stub := &pipelineSnapStub{snap: nil}
	ctx := risk.PlanContext{
		Enabled: true, MarketLevel: 4, BlockNewEntries: true,
		Cash: 500_000, EquityBase: 1_000_000,
		LongMarketValue: 150_000,
		NameMarketValue: map[string]float64{"sz000001": 150_000},
		AmountPerStock: 100_000, MaxNames: 1,
		MaxGrossExposurePct: 0.90, MaxSingleNamePct: 0.20, ScanLimit: 1,
	}
	res := enhanceThenFilter(t, items, stub, ctx)
	if res.AcceptedCount != 0 || len(res.Items) != 1 {
		t.Fatalf("accepted=%d n=%d", res.AcceptedCount, len(res.Items))
	}
	if res.Items[0].Status != "skipped" || res.Items[0].RiskCode != risk.ReasonSingleNameExceeded {
		t.Fatalf("got %+v", res.Items[0])
	}
}

func TestPipeline_Case3_SignalRaisesRank(t *testing.T) {
	items := []enhancer.CandidateItem{
		{StockCode: "sz000001", StrategyName: "s1", StrategyVersion: "v1", StrategyScore: 0.9},
		{StockCode: "sh600519", StrategyName: "s1", StrategyVersion: "v1", StrategyScore: 0.5},
	}
	stub := &pipelineSnapStub{
		snap: &models.SignalScanSnapshot{ID: 9, TradeDate: "2026-07-16", Session: "close", Status: "done"},
		hits: []models.SignalScanHit{
			{SECUCODE: "sh600519", Tag: "强"},
		},
	}
	enh := &enhancer.SignalSnapshotEnhancer{Source: stub}
	enhanced, err := enh.Enhance(items, enhancer.EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	// sh600519 final = 0.6*0.5 + 0.4*1 = 0.7；sz000001 = 0.6*0.9 = 0.54 → 600519 应更高
	if enhanced[0].StockCode == "sz000001" && enhanced[1].StockCode == "sh600519" {
		// 尚未排序；手动比分
	}
	var scoreA, scoreB float64
	for _, it := range enhanced {
		switch it.StockCode {
		case "sz000001":
			scoreA = it.Score
		case "sh600519":
			scoreB = it.Score
		}
	}
	if !(scoreB > scoreA) {
		t.Fatalf("signal should raise sh600519: a=%.4f b=%.4f", scoreA, scoreB)
	}
}

func TestPipeline_Case4_NoSnapshotRuns(t *testing.T) {
	items := []enhancer.CandidateItem{
		{StockCode: "sz000001", StrategyScore: 0.8},
	}
	stub := &pipelineSnapStub{snap: nil}
	enh := &enhancer.SignalSnapshotEnhancer{Source: stub}
	enhanced, err := enh.Enhance(items, enhancer.EnhanceContext{TradeDate: "2026-07-17"})
	if err != nil {
		t.Fatal(err)
	}
	if enhanced[0].SignalSnapshotID != 0 || enhanced[0].SignalTag != "" {
		t.Fatalf("expected no signal: %+v", enhanced[0])
	}
	want := enhancer.StrategyWeight * 0.8
	if enhanced[0].Score != want {
		t.Fatalf("score=%.4f want=%.4f", enhanced[0].Score, want)
	}
}

func TestPipeline_Case5_NoRiskConfigBypass(t *testing.T) {
	// Enable=false 等同无风控配置旁路，与 Phase1 TopN 一致
	items := []enhancer.CandidateItem{
		{StockCode: "a", StrategyScore: 1},
		{StockCode: "b", StrategyScore: 0.9},
		{StockCode: "c", StrategyScore: 0.8},
	}
	stub := &pipelineSnapStub{snap: nil}
	ctx := risk.PlanContext{
		Enabled: false, MarketLevel: 1, BlockNewEntries: true,
		Cash: 1, AmountPerStock: 100_000, MaxNames: 2,
	}
	res := enhanceThenFilter(t, items, stub, ctx)
	if res.RiskStatus != risk.PlanRiskStatusBypassed || res.AcceptedCount != 2 {
		t.Fatalf("status=%s accepted=%d", res.RiskStatus, res.AcceptedCount)
	}
	for _, it := range res.Items {
		if it.Status != "pending" {
			t.Fatalf("%+v", it)
		}
	}
}
