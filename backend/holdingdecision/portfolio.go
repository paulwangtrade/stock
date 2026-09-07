package holdingdecision

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
)

const portfolioDataSourceNote = "Portfolio Holding Observation · Snapshot account + Evaluation facts + Decision states; read-only; not a sell or rebalance instruction"

// AccountSlice is the snapshot-backed account block (persisted mark, not overlay).
type AccountSlice struct {
	AccountID     uint    `json:"account_id,omitempty"`
	AccountName   string  `json:"account_name,omitempty"`
	Found         bool    `json:"found"`
	TotalEquity   float64 `json:"total_equity"`
	Cash          float64 `json:"cash"`
	MarketValue   float64 `json:"market_value"`
	Exposure      float64 `json:"exposure"`
	AvailableCash float64 `json:"available_cash"`
}

// DecisionCounts is stock-level decision histogram (not a sell queue).
type DecisionCounts struct {
	HoldNormalCount    int `json:"hold_normal_count"`
	HoldWatchCount     int `json:"hold_watch_count"`
	HoldReviewCount    int `json:"hold_review_count"`
	ExitCandidateCount int `json:"exit_candidate_count"`
}

// RiskDistribution is Evaluation risk_state histogram (facts, not decisions).
type RiskDistribution struct {
	NormalCount  int `json:"normal_count"`
	WatchCount   int `json:"watch_count"`
	DangerCount  int `json:"danger_count"`
	UnknownCount int `json:"unknown_count"`
}

// DecisionMarketValue weights decision states by evaluation (overlay) market value.
type DecisionMarketValue struct {
	Basis            string  `json:"basis"`
	Total            float64 `json:"total"`
	HoldNormal       float64 `json:"hold_normal"`
	HoldWatch        float64 `json:"hold_watch"`
	HoldReview       float64 `json:"hold_review"`
	ExitCandidate    float64 `json:"exit_candidate"`
	HoldWatchWeight  float64 `json:"hold_watch_weight"`
	HoldReviewWeight float64 `json:"hold_review_weight"`
}

// HoldingLink joins one symbol's evaluation facts with its decision (observation only).
type HoldingLink struct {
	Symbol           string   `json:"symbol"`
	DecisionState    string   `json:"decision_state"`
	DecisionReason   string   `json:"decision_reason"`
	RiskState        string   `json:"risk_state,omitempty"`
	ProfitState      string   `json:"profit_state,omitempty"`
	PeriodState      string   `json:"period_state,omitempty"`
	MarketValue      *float64 `json:"market_value,omitempty"`
	UnrealizedReturn *float64 `json:"unrealized_return,omitempty"`
	HoldingDays      int      `json:"holding_days"`
	Action           string   `json:"action"`
}

// PortfolioSummary is the combination-level observation DTO (Phase10-D.9).
type PortfolioSummary struct {
	AsOf                    string              `json:"as_of,omitempty"`
	Account                 AccountSlice        `json:"account"`
	PositionCount           int                 `json:"position_count"`
	EvalHoldingCount        int                 `json:"eval_holding_count"`
	DecisionCounts          DecisionCounts      `json:"decision_counts"`
	RiskDistribution        RiskDistribution    `json:"risk_distribution"`
	DecisionMarketValue     DecisionMarketValue `json:"decision_market_value"`
	PortfolioDecisionState  string              `json:"portfolio_decision_state"`
	PortfolioDecisionReason string              `json:"portfolio_decision_reason"`
	Holdings                []HoldingLink       `json:"holdings"`
	ExitCandidateEnabled    bool                `json:"exit_candidate_enabled"`
	Action                  string              `json:"action"`
	DataSourceNote          string              `json:"data_source_note"`
}

// BuildPortfolioObservation aggregates Snapshot + Evaluation + Decision.
// Pure: does not mutate inputs, does not read/write DB, does not emit sell/rebalance.
func BuildPortfolioObservation(snap *portfolio.Snapshot, eval *papertrading.HoldingEvalObservationView, decision *View) *PortfolioSummary {
	out := &PortfolioSummary{
		Action:                  actionNone,
		DataSourceNote:          portfolioDataSourceNote,
		Holdings:                []HoldingLink{},
		PortfolioDecisionState:  StateHoldNormal,
		PortfolioDecisionReason: ReasonNone,
	}
	if decision == nil {
		decision = EvaluateObservation(eval, DefaultPolicy())
	}
	out.ExitCandidateEnabled = decision.ExitCandidateEnabled

	if snap != nil {
		out.AsOf = snap.AsOf.Format(time.RFC3339)
		out.Account = AccountSlice{
			AccountID:     snap.AccountID,
			AccountName:   snap.AccountName,
			Found:         snap.Found,
			TotalEquity:   snap.TotalEquity,
			Cash:          snap.Cash,
			MarketValue:   snap.MarketValue,
			Exposure:      snap.TotalExposure,
			AvailableCash: snap.AvailableCash,
		}
		out.PositionCount = snap.PositionCount
	}
	if eval != nil && !eval.AsOf.IsZero() && out.AsOf == "" {
		out.AsOf = eval.AsOf.Format(time.RFC3339)
	}

	evalBy := map[string]papertrading.HoldingEvalStockRow{}
	if eval != nil {
		out.EvalHoldingCount = len(eval.Holdings)
		for _, row := range eval.Holdings {
			code := strings.TrimSpace(row.StockCode)
			evalBy[code] = row
			switch strings.ToUpper(strings.TrimSpace(row.RiskState)) {
			case papertrading.RiskStateWatch:
				out.RiskDistribution.WatchCount++
			case papertrading.RiskStateDanger:
				out.RiskDistribution.DangerCount++
			case papertrading.RiskStateNormal:
				out.RiskDistribution.NormalCount++
			default:
				out.RiskDistribution.UnknownCount++
			}
		}
	}

	snapMV := map[string]float64{}
	if snap != nil {
		for _, p := range snap.Positions {
			snapMV[strings.TrimSpace(p.StockCode)] = p.MarketValue
		}
	}

	usedEvalMV := false
	mv := DecisionMarketValue{Basis: "snapshot_mark"}
	for _, h := range decision.Holdings {
		row, ok := evalBy[h.Symbol]
		link := HoldingLink{
			Symbol:         h.Symbol,
			DecisionState:  h.State,
			DecisionReason: h.Reason,
			Action:         actionNone,
		}
		if ok {
			link.RiskState = row.RiskState
			link.ProfitState = row.ProfitState
			link.PeriodState = row.HoldingPeriodState
			link.UnrealizedReturn = row.UnrealizedReturn
			link.HoldingDays = row.HoldingDays
			link.MarketValue = row.MarketValue
		}
		resolved, fromEval := resolveMarketValue(link.MarketValue, snapMV[h.Symbol])
		if fromEval {
			usedEvalMV = true
		}
		if link.MarketValue == nil && resolved > 0 {
			v := resolved
			link.MarketValue = &v
		}
		addDecisionMV(&mv, h.State, resolved)
		out.Holdings = append(out.Holdings, link)
		switch h.State {
		case StateHoldWatch:
			out.DecisionCounts.HoldWatchCount++
		case StateHoldReview:
			out.DecisionCounts.HoldReviewCount++
		case StateExitCandidate:
			out.DecisionCounts.ExitCandidateCount++
		default:
			out.DecisionCounts.HoldNormalCount++
		}
		if rankState(h.State) > rankState(out.PortfolioDecisionState) {
			out.PortfolioDecisionState = h.State
			out.PortfolioDecisionReason = h.Reason
		}
	}
	if usedEvalMV {
		mv.Basis = "evaluation_overlay"
	}
	if mv.Total > 0 {
		mv.HoldWatchWeight = mv.HoldWatch / mv.Total
		mv.HoldReviewWeight = mv.HoldReview / mv.Total
	}
	out.DecisionMarketValue = mv

	sort.Slice(out.Holdings, func(i, j int) bool {
		return out.Holdings[i].Symbol < out.Holdings[j].Symbol
	})
	return out
}

func resolveMarketValue(evalMV *float64, snapMV float64) (float64, bool) {
	if evalMV != nil {
		return *evalMV, true
	}
	return snapMV, false
}

func addDecisionMV(mv *DecisionMarketValue, state string, value float64) {
	mv.Total += value
	switch state {
	case StateHoldWatch:
		mv.HoldWatch += value
	case StateHoldReview:
		mv.HoldReview += value
	case StateExitCandidate:
		mv.ExitCandidate += value
	default:
		mv.HoldNormal += value
	}
}
