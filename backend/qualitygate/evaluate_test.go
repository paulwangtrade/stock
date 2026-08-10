package qualitygate

import (
	"testing"

	"go-stock/backend/models"
)

func findingByRule(r Result, rule string) (Finding, bool) {
	for _, f := range r.Findings {
		if f.RuleCode == rule {
			return f, true
		}
	}
	return Finding{}, false
}

func hasCode(r Result, code string) bool {
	for _, f := range r.Findings {
		if !f.Passed && f.Code == code {
			return true
		}
	}
	return false
}

// planId=3 shape from AfterClose simulation (2026-07-24): 5 buys, limit=0,
// 通信设备×3 → G1; also E1 on missing prices.
func planID3Fixture() PlanView {
	aps := 100_000.0
	return PlanView{
		ID:             3,
		TradeDate:      "2026-07-24",
		PlanVersion:    1,
		Status:         "draft",
		AmountPerStock: aps,
		Items: []ItemView{
			{StockCode: "000977", StockName: "德明利", Side: "buy", TargetAmount: aps, LimitPrice: 0},
			{StockCode: "600487", StockName: "亨通光电", Side: "buy", TargetAmount: aps, LimitPrice: 0},
			{StockCode: "600522", StockName: "中天科技", Side: "buy", TargetAmount: aps, LimitPrice: 0},
			{StockCode: "600105", StockName: "永鼎股份", Side: "buy", TargetAmount: aps, LimitPrice: 0},
			{StockCode: "000988", StockName: "华工科技", Side: "buy", TargetAmount: aps, LimitPrice: 0},
		},
	}
}

func planID3Market() MarketDataSnapshot {
	return MarketDataSnapshot{
		IndustryByCode: map[string]string{
			"000977": "半导体",
			"600487": "通信设备",
			"600522": "通信设备",
			"600105": "通信设备",
			"000988": "专用机械",
		},
		NameByCode: map[string]string{
			"000977": "德明利",
			"600487": "亨通光电",
			"600522": "中天科技",
			"600105": "永鼎股份",
			"000988": "华工科技",
		},
		// no open / anchor → M1 WARN, P1 WARN
	}
}

func TestPlanID3TriggersG1(t *testing.T) {
	r := Evaluate(Input{
		Plan:       planID3Fixture(),
		Positions:  nil,
		MarketData: planID3Market(),
	})
	f, ok := findingByRule(r, RuleG1)
	if !ok {
		t.Fatal("expected G1 finding")
	}
	if f.Passed || f.Severity != SeverityWARN {
		t.Fatalf("G1 want WARN fail, got passed=%v severity=%s msg=%s", f.Passed, f.Severity, f.Message)
	}
	if f.Code != CodeSectorConcentration {
		t.Fatalf("G1 code=%s", f.Code)
	}
	if !hasCode(r, CodeSectorConcentration) {
		t.Fatal("aggregate missing SECTOR_CONCENTRATION")
	}
}

func TestMissingEntryPriceTriggersE1(t *testing.T) {
	plan := PlanView{
		ID: 10, TradeDate: "2026-07-24", AmountPerStock: 100_000,
		Items: []ItemView{
			{StockCode: "600000", StockName: "浦发银行", Side: "buy", Industry: "银行", LimitPrice: 0, TargetAmount: 100_000},
			{StockCode: "600519", StockName: "贵州茅台", Side: "buy", Industry: "白酒", LimitPrice: 0, TargetAmount: 100_000},
		},
	}
	md := MarketDataSnapshot{
		OpenPriceByCode:   map[string]float64{"600000": 10, "600519": 1800},
		AnchorPriceByCode: map[string]float64{"600000": 10, "600519": 1800},
	}
	r := Evaluate(Input{Plan: plan, MarketData: md})
	f, ok := findingByRule(r, RuleE1)
	if !ok || f.Passed || f.Severity != SeverityFAIL {
		t.Fatalf("E1 want FAIL, ok=%v passed=%v sev=%s", ok, f.Passed, f.Severity)
	}
	if f.Code != CodeEntryPriceMissing {
		t.Fatalf("code=%s", f.Code)
	}
	if r.Passed || r.Severity != SeverityFAIL {
		t.Fatalf("overall want FAIL passed=%v sev=%s", r.Passed, r.Severity)
	}
}

func TestPositionConflictTriggersI1(t *testing.T) {
	plan := PlanView{
		ID: 11, TradeDate: "2026-07-24", AmountPerStock: 100_000,
		Items: []ItemView{
			{StockCode: "600000", StockName: "浦发银行", Side: "buy", Industry: "银行", LimitPrice: 10.5, TargetAmount: 100_000},
			{StockCode: "600519", StockName: "贵州茅台", Side: "buy", Industry: "白酒", LimitPrice: 1800, TargetAmount: 100_000},
		},
	}
	md := MarketDataSnapshot{
		OpenPriceByCode:   map[string]float64{"600000": 10.2, "600519": 1810},
		AnchorPriceByCode: map[string]float64{"600000": 10.2, "600519": 1810},
	}
	pos := []AccountPosition{{StockCode: "600000", Volume: 1000}}
	r := Evaluate(Input{Plan: plan, Positions: pos, MarketData: md})
	f, ok := findingByRule(r, RuleI1)
	if !ok || f.Passed || f.Severity != SeverityFAIL {
		t.Fatalf("I1 want FAIL, ok=%v passed=%v sev=%s msg=%s", ok, f.Passed, f.Severity, f.Message)
	}
	if f.Code != CodePositionConflict {
		t.Fatalf("code=%s", f.Code)
	}
}

func TestGapPendingOutputsWARN(t *testing.T) {
	// Three distinct industries keep G1 quiet so overall severity reflects M1 WARN.
	plan := PlanView{
		ID: 12, TradeDate: "2026-07-24", AmountPerStock: 100_000,
		Items: []ItemView{
			{StockCode: "600000", StockName: "浦发银行", Side: "buy", Industry: "银行", LimitPrice: 10.5, TargetAmount: 100_000},
			{StockCode: "600519", StockName: "贵州茅台", Side: "buy", Industry: "白酒", LimitPrice: 1800, TargetAmount: 100_000},
			{StockCode: "601318", StockName: "中国平安", Side: "buy", Industry: "保险", LimitPrice: 50, TargetAmount: 100_000},
		},
	}
	md := MarketDataSnapshot{
		AnchorPriceByCode: map[string]float64{"600000": 10.2, "600519": 1810, "601318": 49},
		// OpenPriceByCode empty → M1 WARN
	}
	r := Evaluate(Input{Plan: plan, MarketData: md})
	f, ok := findingByRule(r, RuleM1)
	if !ok || f.Passed || f.Severity != SeverityWARN {
		t.Fatalf("M1 want WARN, ok=%v passed=%v sev=%s", ok, f.Passed, f.Severity)
	}
	if f.Code != CodeOpenGapPending {
		t.Fatalf("code=%s", f.Code)
	}
	// no blockers → overall WARN (not FAIL)
	if r.Severity != SeverityWARN {
		t.Fatalf("overall want WARN got %s blockers=%d", r.Severity, len(r.Blockers))
	}
	if r.Passed {
		t.Fatal("passed should be false when WARN")
	}
}

func TestCompletePlanPASS(t *testing.T) {
	plan := PlanView{
		ID: 13, TradeDate: "2026-07-24", AmountPerStock: 100_000,
		Items: []ItemView{
			{StockCode: "600000", StockName: "浦发银行", Side: "buy", Industry: "银行", LimitPrice: 10.5, TargetAmount: 100_000},
			{StockCode: "600519", StockName: "贵州茅台", Side: "buy", Industry: "白酒", LimitPrice: 1800, TargetAmount: 100_000},
			{StockCode: "601318", StockName: "中国平安", Side: "buy", Industry: "保险", LimitPrice: 50, TargetAmount: 100_000},
		},
	}
	md := MarketDataSnapshot{
		OpenPriceByCode: map[string]float64{"600000": 10.2, "600519": 1810, "601318": 49},
		AnchorPriceByCode: map[string]float64{"600000": 10.2, "600519": 1810, "601318": 49},
	}
	r := Evaluate(Input{Plan: plan, Positions: nil, MarketData: md})
	if !r.Passed || r.Severity != SeverityPASS {
		t.Fatalf("want PASS, passed=%v sev=%s findings=%+v", r.Passed, r.Severity, r.Findings)
	}
	for _, f := range r.Findings {
		if !f.Passed {
			t.Fatalf("finding not passed: %+v", f)
		}
	}
}

func TestEvaluateTradePlanWrapper(t *testing.T) {
	plan := &models.TradePlan{
		ID: 3, TradeDate: "2026-07-24", AmountPerStock: 100_000, Status: "draft",
		Items: []models.TradePlanItem{
			{StockCode: "600487", StockName: "亨通光电", Side: "buy", TargetAmount: 100_000},
			{StockCode: "600522", StockName: "中天科技", Side: "buy", TargetAmount: 100_000},
			{StockCode: "600105", StockName: "永鼎股份", Side: "buy", TargetAmount: 100_000},
		},
	}
	md := MarketDataSnapshot{
		IndustryByCode: map[string]string{
			"600487": "通信设备", "600522": "通信设备", "600105": "通信设备",
		},
	}
	r := EvaluateTradePlan(plan, nil, md, Config{})
	if r.PlanID != 3 {
		t.Fatalf("plan_id=%d", r.PlanID)
	}
	if !hasCode(r, CodeSectorConcentration) {
		t.Fatal("expected G1 on 3×通信设备")
	}
	if !hasCode(r, CodeEntryPriceMissing) {
		t.Fatal("expected E1 on missing limit")
	}
}
