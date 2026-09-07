package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/opportunity/projection"

	"github.com/stretchr/testify/require"
)

func registerBetaV04ClosedLoopMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterOpportunitiesRoutes(mux)
	api.RegisterPortfolioDashboardRoutes(mux)
	api.RegisterTradePlansRoutes(mux)
	return mux
}

func betaV04GET(t *testing.T, mux *http.ServeMux, step, path string, wantStatus int) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equalf(t, wantStatus, rec.Code, "%s: unexpected HTTP %d — %s", step, rec.Code, rec.Body.String())
	var body map[string]any
	require.NoErrorf(t, json.Unmarshal(rec.Body.Bytes(), &body), "%s: invalid JSON — %s", step, rec.Body.String())
	return body
}

func betaV04RequireOK(t *testing.T, step string, body map[string]any) {
	t.Helper()
	ok, exists := body["ok"].(bool)
	require.Truef(t, exists && ok, "%s: ok=false body=%v", step, body)
}

func betaV04FetchProjections(t *testing.T, mux *http.ServeMux) map[string]any {
	t.Helper()
	step := "SignalSnapshot→OpportunityProjection GET /api/opportunities/projections"
	body := betaV04GET(t, mux, step,
		fmt.Sprintf("/api/opportunities/projections?stock_code=%s&trade_date=%s", betaV04StockCode, betaV04TradeDate),
		http.StatusOK)
	betaV04RequireOK(t, step, body)
	items, ok := body["items"].([]any)
	require.Truef(t, ok && len(items) > 0, "%s: items empty", step)
	row := items[0].(map[string]any)
	require.Equalf(t, betaV04StockCode, row["stock_code"], "stock_code must be internal lowercase sz301125")
	return row
}

func betaV04AssertSignalOpportunity(t *testing.T, row map[string]any, fx betaV04Fixtures) {
	t.Helper()
	signal, ok := row["signal"].(map[string]any)
	require.Truef(t, ok, "signal block missing")
	require.Truef(t, signal["present"].(bool), "signal.present=false")
	require.EqualValuesf(t, fx.SnapID, signal["snapshot_id"], "snapshot_id")
	require.Equalf(t, betaV04SignalTag, signal["signal_tag"], "signal_tag")
	require.InDeltaf(t, 42.15, signal["signal_price"].(float64), 1e-6, "signal_price")

	opp, ok := row["opportunity"].(map[string]any)
	require.Truef(t, ok, "opportunity block missing")
	require.Truef(t, opp["present"].(bool), "opportunity.present=false")
	require.EqualValuesf(t, fx.PoolID, opp["pool_id"], "pool_id")
	require.EqualValuesf(t, betaV04PoolRank, opp["rank"], "opportunity.rank")
	require.InDeltaf(t, betaV04PoolScore, opp["score"].(float64), 1e-6, "opportunity.score")
}

func betaV04FetchPlanAndOrigin(t *testing.T, mux *http.ServeMux, planID uint) (map[string]any, map[string]any) {
	t.Helper()
	planStep := "TradePlan GET /api/tradeplans/plan"
	planBody := betaV04GET(t, mux, planStep,
		fmt.Sprintf("/api/tradeplans/plan?plan_id=%d", planID), http.StatusOK)
	betaV04RequireOK(t, planStep, planBody)
	plan, ok := planBody["plan"].(map[string]any)
	require.Truef(t, ok, "%s: plan payload missing", planStep)

	originStep := "TradePlan GET /api/tradeplans/{id}/origin"
	originBody := betaV04GET(t, mux, originStep,
		fmt.Sprintf("/api/tradeplans/%d/origin", planID), http.StatusOK)
	betaV04RequireOK(t, originStep, originBody)
	items, ok := originBody["items"].([]any)
	require.Truef(t, ok && len(items) > 0, "%s: items empty", originStep)
	return plan, items[0].(map[string]any)
}

func betaV04FetchSnapshotPositionQty(t *testing.T, mux *http.ServeMux) float64 {
	t.Helper()
	step := "Position GET /api/portfolio/snapshot"
	body := betaV04GET(t, mux, step, "/api/portfolio/snapshot", http.StatusOK)
	betaV04RequireOK(t, step, body)
	snap, ok := body["snapshot"].(map[string]any)
	require.Truef(t, ok, "%s: snapshot missing", step)
	positions, ok := snap["positions"].([]any)
	require.Truef(t, ok, "%s: positions[] missing", step)
	for _, p := range positions {
		pos := p.(map[string]any)
		if pos["stock_code"] == betaV04StockCode {
			qty, ok := pos["total_qty"].(float64)
			require.Truef(t, ok && qty > 0, "%s: position total_qty", step)
			return qty
		}
	}
	require.Failf(t, step, "snapshot missing stock_code=%s", betaV04StockCode)
	return 0
}

func betaV04FetchProvenance(t *testing.T, mux *http.ServeMux) map[string]any {
	t.Helper()
	step := "PortfolioProvenance GET /api/portfolio/positions/{code}/provenance"
	body := betaV04GET(t, mux, step,
		fmt.Sprintf("/api/portfolio/positions/%s/provenance", betaV04StockCode), http.StatusOK)
	betaV04RequireOK(t, step, body)
	prov, ok := body["provenance"].(map[string]any)
	require.Truef(t, ok, "%s: provenance payload missing", step)
	require.Equalf(t, betaV04StockCode, prov["stock_code"], "provenance.stock_code")
	return prov
}

func betaV04FetchOutcomes(t *testing.T, mux *http.ServeMux, status string) map[string]any {
	t.Helper()
	step := fmt.Sprintf("Outcome GET /api/opportunities/outcomes status=%s", status)
	q := fmt.Sprintf("stock_code=%s&status=%s", betaV04StockCode, status)
	if status == outcome.OutcomeStatusNoTrade {
		q = fmt.Sprintf("stock_code=%s&trade_date=%s&status=%s", betaV04StockCode, betaV04TradeDate, status)
	}
	body := betaV04GET(t, mux, step, "/api/opportunities/outcomes?"+q, http.StatusOK)
	betaV04RequireOK(t, step, body)
	return body
}

func betaV04FirstOutcomeItem(t *testing.T, step string, body map[string]any) map[string]any {
	t.Helper()
	items, ok := body["items"].([]any)
	require.Truef(t, ok && len(items) > 0, "%s: items empty", step)
	row := items[0].(map[string]any)
	require.Equalf(t, betaV04StockCode, row["stock_code"], "%s: stock_code", step)
	return row
}

func betaV04RunCrossChecks(
	t *testing.T,
	fx betaV04Fixtures,
	projRow map[string]any,
	originItem map[string]any,
	snapQty float64,
	prov map[string]any,
	outcomeRow map[string]any,
) {
	t.Helper()

	signal := projRow["signal"].(map[string]any)
	opp := projRow["opportunity"].(map[string]any)

	origins := prov["origins"].([]any)
	require.NotEmpty(t, origins, "provenance origins empty")
	origin0 := origins[0].(map[string]any)
	provSignal := origin0["signal"].(map[string]any)
	require.EqualValuesf(t, fx.SnapID, provSignal["snapshot_id"],
		"snapshot_id: projection == provenance")

	outOpp := outcomeRow["opportunity"].(map[string]any)
	require.EqualValuesf(t, opp["rank"], outOpp["rank"], "opportunity.rank: projection == outcome")

	require.Equalf(t, signal["signal_tag"], originItem["signal_tag"], "signal_tag: projection == origin")
	require.Equalf(t, signal["signal_tag"], provSignal["tag"], "signal_tag: projection == provenance")

	trades := prov["trades"].([]any)
	require.NotEmpty(t, trades, "provenance trades empty")
	trade0 := trades[0].(map[string]any)
	entry := outcomeRow["entry"].(map[string]any)

	require.EqualValuesf(t, fx.PlanID, trade0["plan_id"], "plan_id: provenance trades")
	require.EqualValuesf(t, fx.PlanID, entry["buy_plan_id"], "plan_id: outcome entry")
	require.EqualValuesf(t, fx.BuyFillID, trade0["fill_id"], "fill_id: provenance trades")
	require.EqualValuesf(t, fx.BuyFillID, entry["buy_fill_id"], "fill_id: outcome entry")

	provPos := prov["position"].(map[string]any)
	require.InDeltaf(t, snapQty, provPos["quantity"].(float64), 1e-6,
		"quantity: snapshot == provenance position")
	require.InDeltaf(t, float64(fx.BuyVolume), snapQty, 1e-6,
		"quantity: fixture buy volume == snapshot")

	reconcile := prov["reconcile"].(map[string]any)
	require.Equalf(t, "matched", reconcile["status"], "provenance reconcile.status")

	decision := projRow["decision"].(map[string]any)
	if fx.PlanID > 0 {
		require.EqualValuesf(t, fx.PlanID, decision["plan_id"], "plan_id: projection decision")
	}
}

func betaV04AssertDecisionOpen(t *testing.T, projRow map[string]any, fx betaV04Fixtures) {
	t.Helper()
	decision := projRow["decision"].(map[string]any)
	require.Equalf(t, projection.DecisionStatusBuyCandidate, decision["decision_status"], "decision_status BUY_CANDIDATE")
	require.EqualValuesf(t, fx.PlanID, decision["plan_id"], "decision.plan_id")

	tp := projRow["trade_plan"].(map[string]any)
	require.Truef(t, tp["present"].(bool), "trade_plan.present")
	require.EqualValuesf(t, fx.PlanID, tp["plan_id"], "trade_plan.plan_id")

	portfolio := projRow["portfolio"].(map[string]any)
	require.Equalf(t, projection.HoldingStatusHeld, portfolio["holding_status"], "portfolio.HELD")
	require.InDeltaf(t, float64(fx.BuyVolume), portfolio["position_qty"].(float64), 1e-6, "portfolio.position_qty")
}

func betaV04AssertOutcomeOpen(t *testing.T, outRow map[string]any, fx betaV04Fixtures) {
	t.Helper()
	require.Equalf(t, outcome.OutcomeStatusOpen, outRow["outcome_status"], "outcome_status OPEN")
	entry := outRow["entry"].(map[string]any)
	require.Truef(t, entry["present"].(bool), "entry.present")
	require.EqualValuesf(t, fx.BuyFillID, entry["buy_fill_id"], "entry.buy_fill_id")
	require.EqualValuesf(t, fx.PlanID, entry["buy_plan_id"], "entry.buy_plan_id")
	require.InDeltaf(t, fx.BuyPrice, entry["entry_price"].(float64), 1e-6, "entry_price")
	exit := outRow["exit"].(map[string]any)
	require.Falsef(t, exit["present"].(bool), "exit.present false for OPEN")
}

func betaV04AssertOutcomeClosed(t *testing.T, outRow map[string]any, buyPrice, sellPrice float64, buyVolume int64) {
	t.Helper()
	require.Equalf(t, outcome.OutcomeStatusClosed, outRow["outcome_status"], "outcome_status CLOSED")
	exit := outRow["exit"].(map[string]any)
	require.Truef(t, exit["present"].(bool), "exit.present")
	require.InDeltaf(t, sellPrice, exit["exit_price"].(float64), 1e-6, "exit_price")

	perf := outRow["performance"].(map[string]any)
	pct, ok := perf["realized_return_pct"].(float64)
	require.Truef(t, ok, "performance.realized_return_pct")
	expectedPct := (sellPrice*float64(buyVolume) - buyPrice*float64(buyVolume)) / (buyPrice * float64(buyVolume)) * 100
	require.InDeltaf(t, expectedPct, pct, 0.01, "realized_return_pct")

	meta := outRow["metadata"].(map[string]any)
	require.Equalf(t, outcome.FIFOPolicyAccountV1, meta["fifo_policy"], "fifo_policy")
}
