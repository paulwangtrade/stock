package risk

import (
	"testing"
)

func TestPlanFilter_MarketLevelBlocked_AllSkipped(t *testing.T) {
	cands := []PlanCandidate{
		{StockCode: "sz000001", Score: 1},
		{StockCode: "sh600519", Score: 0.9},
		{StockCode: "sz000002", Score: 0.8},
	}
	ctx := PlanContext{
		Enabled:             true,
		MarketLevel:         2,
		BlockNewEntries:     true,
		Cash:                1_000_000,
		EquityBase:          1_000_000,
		AmountPerStock:      100_000,
		MaxNames:            3,
		MaxGrossExposurePct: 0.85,
		MaxSingleNamePct:    0.20,
		ScanLimit:           3,
	}
	res := PlanFilter(cands, ctx)
	if res.RiskStatus != PlanRiskStatusBlocked {
		t.Fatalf("status=%s want blocked", res.RiskStatus)
	}
	if res.AcceptedCount != 0 || res.FilteredCount != 3 {
		t.Fatalf("accepted=%d filtered=%d", res.AcceptedCount, res.FilteredCount)
	}
	for _, it := range res.Items {
		if it.Status != "skipped" || it.RiskCode != ReasonMarketLevelBlocked {
			t.Fatalf("item %+v", it)
		}
	}
}

func TestPlanFilter_CashInsufficient_PartialAccept(t *testing.T) {
	cands := []PlanCandidate{
		{StockCode: "sz000001", Score: 1},
		{StockCode: "sh600519", Score: 0.9},
		{StockCode: "sz000002", Score: 0.8},
	}
	// 只能买 1 只 100k
	ctx := PlanContext{
		Enabled:             true,
		MarketLevel:         3,
		BlockNewEntries:     true,
		Cash:                150_000,
		EquityBase:          1_000_000,
		LongMarketValue:     0,
		AmountPerStock:      100_000,
		MaxNames:            3,
		MaxGrossExposurePct: 0.85,
		MaxSingleNamePct:    0.50,
		ScanLimit:           3,
	}
	res := PlanFilter(cands, ctx)
	if res.AcceptedCount != 1 || res.FilteredCount != 2 {
		t.Fatalf("accepted=%d filtered=%d items=%d", res.AcceptedCount, res.FilteredCount, len(res.Items))
	}
	if res.RiskStatus != PlanRiskStatusPartial {
		t.Fatalf("status=%s", res.RiskStatus)
	}
	if res.Items[0].Status != "pending" {
		t.Fatalf("first=%+v", res.Items[0])
	}
	for i := 1; i < len(res.Items); i++ {
		if res.Items[i].RiskCode != ReasonCashInsufficient || res.Items[i].Status != "skipped" {
			t.Fatalf("item[%d]=%+v", i, res.Items[i])
		}
	}
}

func TestPlanFilter_SingleNameExceeded(t *testing.T) {
	cands := []PlanCandidate{
		{StockCode: "sz000001", Score: 1},
	}
	ctx := PlanContext{
		Enabled:             true,
		MarketLevel:         4,
		BlockNewEntries:     true,
		Cash:                500_000,
		EquityBase:          1_000_000,
		LongMarketValue:     150_000,
		NameMarketValue:     map[string]float64{"sz000001": 150_000},
		AmountPerStock:      100_000, // post name 250k / 1M = 25% > 20%
		MaxNames:            1,
		MaxGrossExposurePct: 0.90,
		MaxSingleNamePct:    0.20,
		ScanLimit:           1,
	}
	res := PlanFilter(cands, ctx)
	if res.AcceptedCount != 0 || len(res.Items) != 1 {
		t.Fatalf("accepted=%d items=%d", res.AcceptedCount, len(res.Items))
	}
	if res.Items[0].RiskCode != ReasonSingleNameExceeded {
		t.Fatalf("code=%s msg=%s", res.Items[0].RiskCode, res.Items[0].RiskMessage)
	}
}

func TestPlanFilter_Disabled_Phase1Behavior(t *testing.T) {
	cands := []PlanCandidate{
		{StockCode: "a", Score: 1},
		{StockCode: "b", Score: 0.9},
		{StockCode: "c", Score: 0.8},
		{StockCode: "d", Score: 0.7},
		{StockCode: "e", Score: 0.6},
		{StockCode: "f", Score: 0.5},
	}
	ctx := PlanContext{
		Enabled:        false,
		MarketLevel:    1, // 即使防守也不应拦截
		BlockNewEntries: true,
		Cash:           1,
		AmountPerStock: 100_000,
		MaxNames:       5,
	}
	res := PlanFilter(cands, ctx)
	if res.RiskStatus != PlanRiskStatusBypassed {
		t.Fatalf("status=%s", res.RiskStatus)
	}
	if res.AcceptedCount != 5 || res.FilteredCount != 0 || len(res.Items) != 5 {
		t.Fatalf("accepted=%d filtered=%d n=%d", res.AcceptedCount, res.FilteredCount, len(res.Items))
	}
	for _, it := range res.Items {
		if it.Status != "pending" || !it.Allowed {
			t.Fatalf("item %+v", it)
		}
	}
}

func TestPlanFilter_DailyLossHalt_AllSkipped(t *testing.T) {
	cands := []PlanCandidate{{StockCode: "sz000001"}, {StockCode: "sh600519"}}
	ctx := PlanContext{
		Enabled:            true,
		MarketLevel:        4,
		BlockNewEntries:    true,
		Cash:               1_000_000,
		EquityBase:         1_000_000,
		AmountPerStock:     100_000,
		MaxNames:           2,
		MaxDailyLossPct:    0.03,
		CurrentDailyPnlPct: -0.05,
		ScanLimit:          2,
	}
	res := PlanFilter(cands, ctx)
	if res.AcceptedCount != 0 || res.FilteredCount != 2 {
		t.Fatalf("accepted=%d filtered=%d", res.AcceptedCount, res.FilteredCount)
	}
	if res.Items[0].RiskCode != ReasonDailyLossHalt {
		t.Fatalf("code=%s", res.Items[0].RiskCode)
	}
}

func TestPlanFilter_GrossExposureExceeded(t *testing.T) {
	cands := []PlanCandidate{{StockCode: "sz000001"}}
	ctx := PlanContext{
		Enabled:             true,
		MarketLevel:         4,
		Cash:                500_000,
		EquityBase:          1_000_000,
		LongMarketValue:     800_000,
		AmountPerStock:      100_000, // post 900k/1M=90% > 85%
		MaxNames:            1,
		MaxGrossExposurePct: 0.85,
		MaxSingleNamePct:    0.50,
		ScanLimit:           1,
	}
	res := PlanFilter(cands, ctx)
	if res.Items[0].RiskCode != ReasonGrossExposureExceeded {
		t.Fatalf("code=%s msg=%s", res.Items[0].RiskCode, res.Items[0].RiskMessage)
	}
}

func TestPlanCheck_DoesNotRequirePriceVolume(t *testing.T) {
	ctx := PlanContext{
		Enabled:             true,
		MarketLevel:         3,
		Cash:                200_000,
		EquityBase:          1_000_000,
		MaxGrossExposurePct: 0.85,
		MaxSingleNamePct:    0.20,
	}
	d := PlanCheck(ctx, "sz000001", 100_000, 100_000, 100_000, 200_000)
	if !d.Allowed || d.Code != ReasonApproved {
		t.Fatalf("decision=%+v", d)
	}
}
