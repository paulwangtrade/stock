package portfolioobs

import (
	"strings"

	"go-stock/backend/holdingdecision"
)

const opportunityCostNote = "机会成本观察未启用：需要 CandidatePool 与 Target 对比；E.3 不计算复杂模型"

// Enhance attaches Aging / Health / History / opportunity stub onto an assembled observation.
// Pure projection; does not recompute Evaluation/Decision/Diff engines or write DB.
func Enhance(obs *Observation) {
	if obs == nil {
		return
	}
	if obs.Disclaimer == "" {
		obs.Disclaimer = Disclaimer
	}
	if obs.Action == "" {
		obs.Action = actionNone
	}
	if obs.DataSourceNotes == nil {
		obs.DataSourceNotes = map[string]string{}
	}
	obs.DataSourceNotes["aging"] = "E.3 holding_days → SHORT/MEDIUM/LONG；LONG 只观察，不卖出"
	obs.DataSourceNotes["health"] = "E.3 observation health_score；不是 Strategy Score，不是交易评分"
	obs.DataSourceNotes["history"] = "Decision history · runtime current point only；not a trade signal"
	obs.DataSourceNotes["opportunity_cost"] = opportunityCostNote
	obs.OpportunityCost = OpportunityCostView{
		Available: false,
		Level:     OpportunityCostUnknown,
		Note:      opportunityCostNote,
	}

	asOf := strings.TrimSpace(obs.AsOf)
	if len(asOf) >= 10 {
		asOf = asOf[:10]
	} else if t := strings.TrimSpace(obs.ObservationTime); len(t) >= 10 {
		asOf = t[:10]
	}

	for i := range obs.Positions {
		row := &obs.Positions[i]
		row.HoldingPeriodBucket, row.IsAging = ClassifyAging(row.HoldingDays, row.FirstBuyDate)
		h := ScoreHealth(HealthInput{
			CurrentPrice:  row.CurrentPrice,
			Return:        row.Return,
			DecisionState: row.DecisionState,
			RiskState:     row.RiskState,
			ProfitState:   row.ProfitState,
			AgingBucket:   row.HoldingPeriodBucket,
		})
		row.HealthScore = h.Score
		row.HealthLevel = h.Level
		hist := HistoryFromCurrent(row.Symbol, row.DecisionState, row.DecisionReason, asOf)
		row.DecisionHistory = hist.Points
		row.DecisionHistoryNote = hist.Note
		row.OpportunityCostLevel = OpportunityCostUnknown
		if row.Action == "" {
			row.Action = actionNone
		}
	}
	obs.Health = summarizeHealth(obs.Positions)
}

func summarizeHealth(rows []PositionRow) PortfolioHealthSummary {
	sum := PortfolioHealthSummary{PortfolioHealth: HealthUnknown}
	worst := 0
	for _, r := range rows {
		switch strings.ToUpper(strings.TrimSpace(r.HealthLevel)) {
		case HealthHealthy:
			sum.HealthyPositions++
		case HealthRisk:
			sum.RiskPositions++
		case HealthUnknown, "":
			sum.UnknownHealthCount++
		}
		if strings.EqualFold(r.DecisionState, holdingdecision.StateHoldWatch) {
			sum.WatchPositions++
		}
		if strings.EqualFold(r.DecisionState, holdingdecision.StateHoldReview) ||
			strings.EqualFold(r.DecisionState, holdingdecision.StateExitCandidate) {
			sum.ReviewPositions++
		}
		if r.IsAging {
			sum.AgingPositions++
		}
		if rk := healthRank(r.HealthLevel); rk > worst {
			worst = rk
		}
	}
	switch worst {
	case 4:
		sum.PortfolioHealth = HealthRisk
	case 3:
		sum.PortfolioHealth = HealthWatch
	case 2:
		sum.PortfolioHealth = HealthNormal
	case 1:
		sum.PortfolioHealth = HealthHealthy
	default:
		sum.PortfolioHealth = HealthUnknown
	}
	return sum
}
