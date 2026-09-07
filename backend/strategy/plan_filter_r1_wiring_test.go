package strategy

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/risk"
)

func stubPlanFilterContext(t *testing.T, ctx risk.PlanContext) {
	t.Helper()
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		out := ctx
		if amount > 0 {
			out.AmountPerStock = amount
		}
		out.MaxNames = maxNames
		return out
	}
	t.Cleanup(func() { planFilterContextFn = prev })
}

func r1WideRisk(cash float64, maxNames, scanLimit int) risk.PlanContext {
	return risk.PlanContext{
		Enabled:             true,
		MarketLevel:         3,
		BlockNewEntries:     true,
		Cash:                cash,
		EquityBase:          1_000_000,
		AmountPerStock:      100_000,
		MaxNames:            maxNames,
		MaxGrossExposurePct: 0.95,
		MaxSingleNamePct:    0.20,
		ScanLimit:           scanLimit,
	}
}

func TestFilterPoolR1_RefillFromRankBeyondSelectionLimit(t *testing.T) {
	// selection_limit=2 → basket is rank 1..2. Rank 1 fails single-name;
	// PlanFilter must keep scanning ranked waitlist and take rank 3.
	ctx := r1WideRisk(1_000_000, 2, 5)
	ctx.LongMarketValue = 150_000
	ctx.NameMarketValue = map[string]float64{"sz000001": 150_000} // 150k+100k > 20% of 1e6
	stubPlanFilterContext(t, ctx)

	pool := &models.CandidatePool{
		Items: []models.CandidatePoolItem{
			{StockCode: "sz000005", StockName: "E", Rank: 5, Score: 10, StrategyName: "s", StrategyVersion: "v1"},
			{StockCode: "sz000001", StockName: "A", Rank: 1, Score: 90, StrategyName: "s", StrategyVersion: "v1"},
			{StockCode: "sz000003", StockName: "C", Rank: 3, Score: 50, StrategyName: "s", StrategyVersion: "v1"},
			{StockCode: "sz000002", StockName: "B", Rank: 2, Score: 80, StrategyName: "s", StrategyVersion: "v1"},
			{StockCode: "sz000004", StockName: "D", Rank: 4, Score: 20, StrategyName: "s", StrategyVersion: "v1"},
		},
	}
	res, err := FilterPoolForTradePlan(pool, 100_000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.AcceptedCount != 2 {
		t.Fatalf("accepted=%d want 2 (rank2 + rank3 refill)", res.AcceptedCount)
	}
	pending := pendingCodes(res)
	if len(pending) != 2 || pending[0] != "sz000002" || pending[1] != "sz000003" {
		t.Fatalf("pending=%v want [sz000002 sz000003]", pending)
	}
	if res.Items[0].Status != "skipped" || res.Items[0].RiskCode != risk.ReasonSingleNameExceeded {
		t.Fatalf("rank1 should fail single-name, got %+v", res.Items[0])
	}
	if res.Items[0].Candidate.Rank != 1 || res.Items[1].Candidate.Rank != 2 || res.Items[2].Candidate.Rank != 3 {
		t.Fatalf("filter must walk ranked order, ranks=%d %d %d",
			res.Items[0].Candidate.Rank, res.Items[1].Candidate.Rank, res.Items[2].Candidate.Rank)
	}
}

func TestFilterPoolR1_ScanLimitNotEqualSelectionLimit(t *testing.T) {
	// MaxNames/selection_limit=3 but ScanLimit=2 → only two names scanned even if all pass.
	stubPlanFilterContext(t, r1WideRisk(1_000_000, 3, 2))
	pool := &models.CandidatePool{
		Items: []models.CandidatePoolItem{
			{StockCode: "sz000001", Rank: 1, Score: 5},
			{StockCode: "sz000002", Rank: 2, Score: 4},
			{StockCode: "sz000003", Rank: 3, Score: 3},
			{StockCode: "sz000004", Rank: 4, Score: 2},
		},
	}
	res, err := FilterPoolForTradePlan(pool, 100_000, 3)
	if err != nil {
		t.Fatal(err)
	}
	if res.AcceptedCount != 2 {
		t.Fatalf("accepted=%d want 2 (ScanLimit=2, not selection_limit=3)", res.AcceptedCount)
	}
	if len(res.Items) != 2 {
		t.Fatalf("items=%d want 2", len(res.Items))
	}
	snapMax := 0
	for _, it := range res.Items {
		if it.Candidate.Rank > snapMax {
			snapMax = it.Candidate.Rank
		}
	}
	if snapMax != 2 {
		t.Fatalf("highest scanned rank=%d want 2", snapMax)
	}
}

func TestFilterPoolR1_RankedOrderIndependentOfPoolShuffle(t *testing.T) {
	stubPlanFilterContext(t, r1WideRisk(1_000_000, 5, 5))
	pool := &models.CandidatePool{
		Items: []models.CandidatePoolItem{
			{StockCode: "sz000004", Rank: 4, Score: 10, StrategyName: "s4"},
			{StockCode: "sz000001", Rank: 1, Score: 90, StrategyName: "s1"},
			{StockCode: "sz000003", Rank: 3, Score: 30, StrategyName: "s3"},
			{StockCode: "sz000002", Rank: 2, Score: 80, StrategyName: "s2"},
		},
	}
	res, err := FilterPoolForTradePlan(pool, 100_000, 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"sz000001", "sz000002", "sz000003", "sz000004"}
	if len(res.Items) != len(want) {
		t.Fatalf("items=%d want %d", len(res.Items), len(want))
	}
	for i, code := range want {
		if res.Items[i].Candidate.StockCode != code {
			t.Fatalf("items[%d]=%s want %s", i, res.Items[i].Candidate.StockCode, code)
		}
		if res.Items[i].Candidate.Rank != i+1 {
			t.Fatalf("items[%d] rank=%d", i, res.Items[i].Candidate.Rank)
		}
		if res.Items[i].Status != "pending" {
			t.Fatalf("items[%d] status=%s", i, res.Items[i].Status)
		}
	}
	if res.Items[0].Candidate.StrategyName != "s1" || res.Items[1].Candidate.StrategyName != "s2" {
		t.Fatalf("strategy fields must copy from pool item, got %s %s",
			res.Items[0].Candidate.StrategyName, res.Items[1].Candidate.StrategyName)
	}
}

func TestFilterPoolR1_DoesNotPassHoldingsOrCashToSelect(t *testing.T) {
	ctx := r1WideRisk(1_000_000, 2, 3)
	ctx.NameMarketValue = map[string]float64{"sz000001": 10_000} // well under 20%
	stubPlanFilterContext(t, ctx)
	pool := &models.CandidatePool{
		Items: []models.CandidatePoolItem{
			{StockCode: "sz000001", Rank: 1, Score: 90},
			{StockCode: "sz000002", Rank: 2, Score: 80},
		},
	}
	res, err := FilterPoolForTradePlan(pool, 100_000, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.AcceptedCount != 2 {
		t.Fatalf("already_holding must not run in Select; accepted=%d", res.AcceptedCount)
	}
	if res.Items[0].Candidate.StockCode != "sz000001" || res.Items[0].Status != "pending" {
		t.Fatalf("held name should still pending: %+v", res.Items[0])
	}
}

func pendingCodes(res *risk.PlanFilterResult) []string {
	out := make([]string, 0)
	for _, it := range res.Items {
		if it.Allowed && it.Status == "pending" {
			out = append(out, it.Candidate.StockCode)
		}
	}
	return out
}
