package tests

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"testing"

	"go-stock/backend/opportunity/outcome"
	"go-stock/backend/opportunity/projection"

	"github.com/stretchr/testify/require"
)

// Closed-loop smoke: SignalSnapshot → OpportunityProjection → TradePlan → Fill →
// Position → PortfolioProvenance → Outcome (read-only GET, memory DB fixtures).

// CL-v04-OPEN — Beta v0.4 minimum Gate.
func TestBetaV04ClosedLoop_OPEN(t *testing.T) {
	fx := seedBetaV04OpenFixtures(t, 41, 500)
	mux := registerBetaV04ClosedLoopMux(t)

	// SignalSnapshot + OpportunityProjection (+ Decision, Portfolio partial)
	projRow := betaV04FetchProjections(t, mux)
	betaV04AssertSignalOpportunity(t, projRow, fx)
	betaV04AssertDecisionOpen(t, projRow, fx)

	// TradePlan + origin
	plan, originItem := betaV04FetchPlanAndOrigin(t, mux, fx.PlanID)
	require.EqualValuesf(t, fx.PoolID, plan["pool_id"], "plan.pool_id == pool_id")

	// Position (snapshot) + PortfolioProvenance
	snapQty := betaV04FetchSnapshotPositionQty(t, mux)
	prov := betaV04FetchProvenance(t, mux)

	// Fill + Outcome
	outBody := betaV04FetchOutcomes(t, mux, outcome.OutcomeStatusOpen)
	outRow := betaV04FirstOutcomeItem(t, "Outcome OPEN", outBody)
	betaV04AssertOutcomeOpen(t, outRow, fx)

	betaV04RunCrossChecks(t, fx, projRow, originItem, snapQty, prov, outRow)
}

// CL-v04-CLOSED — buy + sell FIFO round-trip with realized return.
func TestBetaV04ClosedLoop_CLOSED(t *testing.T) {
	fx, buyPrice, sellPrice := seedBetaV04ClosedFixtures(t)
	mux := registerBetaV04ClosedLoopMux(t)

	projRow := betaV04FetchProjections(t, mux)
	betaV04AssertSignalOpportunity(t, projRow, fx)
	betaV04AssertDecisionOpen(t, projRow, fx)

	_, originItem := betaV04FetchPlanAndOrigin(t, mux, fx.PlanID)
	snapQty := betaV04FetchSnapshotPositionQty(t, mux)
	prov := betaV04FetchProvenance(t, mux)

	outBody := betaV04FetchOutcomes(t, mux, outcome.OutcomeStatusClosed)
	outRow := betaV04FirstOutcomeItem(t, "Outcome CLOSED", outBody)
	betaV04AssertOutcomeClosed(t, outRow, buyPrice, sellPrice, fx.BuyVolume)

	betaV04RunCrossChecks(t, fx, projRow, originItem, snapQty, prov, outRow)
}

// CL-v04-NOTRADE — Signal/Pool present, no fills or plan.
func TestBetaV04ClosedLoop_NOTRADE(t *testing.T) {
	seedBetaV04NoTradeFixtures(t)
	mux := registerBetaV04ClosedLoopMux(t)

	projRow := betaV04FetchProjections(t, mux)
	signal := projRow["signal"].(map[string]any)
	require.Truef(t, signal["present"].(bool), "signal.present")
	require.Greaterf(t, signal["snapshot_id"], float64(0), "snapshot_id > 0")
	require.Equalf(t, betaV04StockCode, projRow["stock_code"], "stock_code")

	opp := projRow["opportunity"].(map[string]any)
	require.Truef(t, opp["present"].(bool), "opportunity.present")
	require.EqualValuesf(t, betaV04PoolRank, opp["rank"], "opportunity.rank")

	decision := projRow["decision"].(map[string]any)
	require.Equalf(t, projection.DecisionStatusWatch, decision["decision_status"], "decision WATCH")

	outBody := betaV04FetchOutcomes(t, mux, outcome.OutcomeStatusNoTrade)
	outRow := betaV04FirstOutcomeItem(t, "Outcome NO_TRADE", outBody)
	require.Equalf(t, outcome.OutcomeStatusNoTrade, outRow["outcome_status"], "outcome_status NO_TRADE")

	entry := outRow["entry"].(map[string]any)
	require.Falsef(t, entry["present"].(bool), "entry.present false")

	outSignal := outRow["signal"].(map[string]any)
	require.Equalf(t, betaV04SignalTag, outSignal["signal_tag"], "outcome signal_tag anchor")
	outOpp := outRow["opportunity"].(map[string]any)
	require.EqualValuesf(t, betaV04PoolRank, outOpp["rank"], "outcome opportunity.rank")
}

// Guard: six HTTP hops succeed without 500 (X-01).
func TestBetaV04ClosedLoop_HTTPChain_OK(t *testing.T) {
	fx := seedBetaV04OpenFixtures(t, 41, 500)
	mux := registerBetaV04ClosedLoopMux(t)

	paths := []struct {
		step string
		url  string
	}{
		{"projections", fmt.Sprintf("/api/opportunities/projections?stock_code=%s&trade_date=%s", betaV04StockCode, betaV04TradeDate)},
		{"plan", "/api/tradeplans/plan?plan_id=" + strconv.FormatUint(uint64(fx.PlanID), 10)},
		{"origin", fmt.Sprintf("/api/tradeplans/%d/origin", fx.PlanID)},
		{"snapshot", "/api/portfolio/snapshot"},
		{"provenance", fmt.Sprintf("/api/portfolio/positions/%s/provenance", betaV04StockCode)},
		{"outcomes", fmt.Sprintf("/api/opportunities/outcomes?stock_code=%s&status=OPEN", betaV04StockCode)},
	}
	for _, p := range paths {
		body := betaV04GET(t, mux, p.step, p.url, http.StatusOK)
		betaV04RequireOK(t, p.step, body)
	}
	require.False(t, math.IsNaN(float64(fx.BuyFillID)))
}
