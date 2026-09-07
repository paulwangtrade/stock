package readiness_test

import (
	"testing"

	"go-stock/backend/tradingplan/readiness"
)

func TestEvaluate_Case1_CashEnough_READY(t *testing.T) {
	items := []readiness.PlanItem{{
		StockCode: "sz000001", Side: "buy",
		TargetAmount: 200_000,
		IntentStatus: "priced",
	}}
	acc := readiness.AccountSnapshot{Cash: 545_329, Equity: 3_000_000}
	res := readiness.Evaluate(1, items, acc, nil)
	if res.Status != readiness.StatusReady {
		t.Fatalf("status=%s want READY", res.Status)
	}
	if !res.CashEnough {
		t.Fatal("expected cash enough")
	}
	if res.RequiredCash != 200_000 {
		t.Fatalf("required=%v want 200000", res.RequiredCash)
	}
	if res.Concentration != readiness.ConcentrationNormal {
		t.Fatalf("concentration=%s want NORMAL", res.Concentration)
	}
}

func TestEvaluate_Case2_CashInsufficient_BLOCKED(t *testing.T) {
	items := []readiness.PlanItem{{
		StockCode: "sz000001", Side: "buy",
		TargetAmount: 600_000, LimitPrice: 10, TargetVolume: 60_000,
	}}
	acc := readiness.AccountSnapshot{Cash: 545_329, Equity: 2_000_000}
	res := readiness.Evaluate(2, items, acc, nil)
	if res.Status != readiness.StatusBlocked {
		t.Fatalf("status=%s want BLOCKED", res.Status)
	}
	if res.CashEnough {
		t.Fatal("expected cash insufficient")
	}
}

func TestEvaluate_Case3_AlreadyHolding_WARNING(t *testing.T) {
	items := []readiness.PlanItem{{
		StockCode: "sh600363", Side: "buy",
		TargetAmount: 100_000, LimitPrice: 20, TargetVolume: 5000,
	}}
	acc := readiness.AccountSnapshot{Cash: 545_329, Equity: 2_000_000}
	positions := []readiness.PositionSnapshot{{
		StockCode: "sh600363", TotalVolume: 5000,
	}}
	res := readiness.Evaluate(37, items, acc, positions)
	if res.Status != readiness.StatusWarning {
		t.Fatalf("status=%s want WARNING", res.Status)
	}
	if len(res.ExistingPositions) != 1 {
		t.Fatalf("conflicts=%d want 1", len(res.ExistingPositions))
	}
	if res.ExistingPositions[0].StockCode != "sh600363" {
		t.Fatalf("conflict code=%s", res.ExistingPositions[0].StockCode)
	}
}

func TestEvaluate_Case4_HighConcentration(t *testing.T) {
	items := []readiness.PlanItem{{
		StockCode: "sh600363", Side: "buy",
		TargetAmount: 350_000, LimitPrice: 70, TargetVolume: 5000,
	}}
	acc := readiness.AccountSnapshot{Cash: 545_329, Equity: 2_000_000}
	res := readiness.Evaluate(4, items, acc, nil)
	if res.Concentration != readiness.ConcentrationHigh {
		t.Fatalf("concentration=%s want HIGH", res.Concentration)
	}
	if res.Status != readiness.StatusWarning {
		t.Fatalf("status=%s want WARNING", res.Status)
	}
}

func TestEvaluate_SkipsNonBuyAndGapSkip(t *testing.T) {
	items := []readiness.PlanItem{
		{StockCode: "a", Side: "sell", TargetAmount: 100_000},
		{StockCode: "b", Side: "buy", TargetAmount: 50_000, IntentStatus: "gap_skip"},
		{StockCode: "c", Side: "buy", TargetAmount: 30_000, IntentStatus: "priced"},
	}
	acc := readiness.AccountSnapshot{Cash: 100_000, Equity: 500_000}
	res := readiness.Evaluate(5, items, acc, nil)
	if res.RequiredCash != 30_000 {
		t.Fatalf("required=%v want 30000", res.RequiredCash)
	}
}

func TestEvaluate_UsesVolumeTimesLimitPrice(t *testing.T) {
	items := []readiness.PlanItem{{
		StockCode: "x", Side: "buy",
		TargetAmount: 999_999, TargetVolume: 100, LimitPrice: 47,
	}}
	acc := readiness.AccountSnapshot{Cash: 1_000_000, Equity: 2_000_000}
	res := readiness.Evaluate(6, items, acc, nil)
	if res.RequiredCash != 4700 {
		t.Fatalf("required=%v want 4700", res.RequiredCash)
	}
}
