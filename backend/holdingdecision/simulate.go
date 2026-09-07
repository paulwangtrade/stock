package holdingdecision

import (
	"strings"
	"time"

	"go-stock/backend/portfoliorisk"
)

const (
	SimSchemaVersion = "holding_decision.sell_sim.h1"
	sellSimNote      = "Sell Decision Simulation · HOLD|REDUCE|EXIT observation only; no SellTradePlan; no Execution; no real sell"
)

// SellDecisionSimInput is the H.1 sell-decision simulation bundle (read-only).
// Required facts: Holdings, PositionState, Evaluation, RiskSnapshot (or PortfolioRisk).
type SellDecisionSimInput struct {
	AsOf           time.Time
	TradeDate      string
	Holdings       []HoldingFact
	PositionStates map[string]PositionStateFact
	Evaluations    map[string]EvalFact
	// RiskSnapshot is the lightweight risk对照. If nil and PortfolioRisk is set, it is derived.
	RiskSnapshot *RiskSnapshotFact
	// PortfolioRisk is optional H.0 PortfolioRiskSnapshot (shared BUY/SELL risk view).
	PortfolioRisk  *portfoliorisk.PortfolioRiskSnapshot
	StrategyScores map[string]StrategyScoreFact
	Policy         ActionPolicy
}

// SimulateSellDecision runs a pure sell-side decision simulation.
// Output actions are HOLD|REDUCE|EXIT only. Never creates SellTradePlan, never calls Execution,
// never submits a real sell. persist_sell_plans is forced false.
func SimulateSellDecision(in SellDecisionSimInput) *ActionView {
	pol := in.Policy
	pol.PersistSellPlans = false

	risk := in.RiskSnapshot
	if risk == nil && in.PortfolioRisk != nil {
		risk = RiskFactFromPortfolioRisk(in.PortfolioRisk)
	}

	view := Observe(ObservationInput{
		AsOf:           in.AsOf,
		TradeDate:      in.TradeDate,
		Holdings:       in.Holdings,
		PositionStates: in.PositionStates,
		Evaluations:    in.Evaluations,
		StrategyScores: in.StrategyScores,
		RiskSnapshot:   risk,
		Policy:         pol,
	})
	if view == nil {
		return &ActionView{
			SchemaVersion:    SimSchemaVersion,
			PersistSellPlans: false,
			RecordOnly:       true,
			NotAnOrder:       true,
			NotExecution:     true,
			NotSellTradePlan: true,
			NotBuyChain:      true,
			ByAction: map[string]int{
				ActionHold: 0, ActionReduce: 0, ActionExit: 0,
			},
			Decisions:      []ActionDecision{},
			DataSourceNote: sellSimNote,
		}
	}
	view.SchemaVersion = SimSchemaVersion
	view.PersistSellPlans = false
	view.RecordOnly = true
	view.NotAnOrder = true
	view.NotExecution = true
	view.NotSellTradePlan = true
	view.NotBuyChain = true
	view.DataSourceNote = sellSimNote
	for i := range view.Decisions {
		view.Decisions[i].PersistSellPlans = false
		view.Decisions[i].RecordOnly = true
		view.Decisions[i].NotAnOrder = true
	}
	return view
}

// RiskFactFromPortfolioRisk projects H.0 PortfolioRiskSnapshot into simulation risk facts.
// available=false blocks do not invent zeros for caps.
func RiskFactFromPortfolioRisk(s *portfoliorisk.PortfolioRiskSnapshot) *RiskSnapshotFact {
	if s == nil {
		return &RiskSnapshotFact{Found: false}
	}
	out := &RiskSnapshotFact{
		Found:        s.Found,
		MarketRegime: strings.TrimSpace(s.Market.MarketRegime),
	}
	if !s.Found {
		return out
	}
	if s.Exposure.Available {
		out.GrossExposure = cloneFloat64(s.Exposure.GrossExposure)
	}
	if s.Concentration.Available {
		out.Top1Weight = cloneFloat64(s.Concentration.Top1Weight)
		out.MaxSingleNamePct = cloneFloat64(s.Concentration.CapSingle)
	}
	return out
}

func cloneFloat64(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
