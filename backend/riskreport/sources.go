package riskreport

import (
	"strings"

	"go-stock/backend/marketstate"
	"go-stock/backend/papertrading"
)

// FetchLiveSources loads read-only Observation views (no trading writes).
func FetchLiveSources(tradeDate string) (ReportSources, []SourceRef, string, error) {
	src := ReportSources{
		HoldingRiskStates: map[string]int{},
		HoldingSymbols:    map[string][]string{},
		ExitReasonCounts:  map[string]int{},
		ExitReviewStates:  map[string]int{},
	}
	refs := []SourceRef{}
	scope := "unknown"

	port, err := papertrading.BuildRiskObservation()
	if err != nil {
		return src, refs, scope, err
	}
	if port != nil {
		src.PortfolioEnabled = port.Enabled
		src.PortfolioQuality = port.Quality
		src.Cash = port.Cash
		src.PositionRatio = port.PositionRatio
		src.Concentration = port.Concentration
		for _, p := range port.Positions {
			src.PositionSymbols = append(src.PositionSymbols, p.StockCode)
		}
		refs = append(refs, SourceRef{Kind: "risk_observation", Label: port.DataSourceNote})
	}

	hold, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	if err != nil {
		// continue with partial
	}
	if hold != nil {
		for _, h := range hold.Holdings {
			st := strings.ToUpper(strings.TrimSpace(h.RiskState))
			if st == "" {
				st = "NORMAL"
			}
			src.HoldingRiskStates[st]++
			src.HoldingSymbols[st] = append(src.HoldingSymbols[st], h.StockCode)
		}
		refs = append(refs, SourceRef{Kind: "holding_evaluation", Label: hold.DataSourceNote})
	}

	exit, err := papertrading.BuildExitEvaluation(papertrading.ExitEvaluationBuildOptions{})
	if err != nil {
		// partial
	}
	if exit != nil {
		for k, v := range exit.Summary.ReasonCounts {
			src.ExitReasonCounts[k] = v
		}
		for k, v := range exit.Summary.ByEvaluationState {
			src.ExitReviewStates[k] = v
		}
		refs = append(refs, SourceRef{Kind: "exit_evaluation", Label: exit.DataSourceNote})
	}

	exec, err := papertrading.BuildExecutionSummary(tradeDate)
	if err != nil {
		// partial
	}
	if exec != nil {
		src.ExecEnabled = exec.Enabled
		src.ExecTotalOrders = exec.TotalOrders
		src.ExecFilled = exec.FilledOrders
		src.ExecFailed = exec.FailedOrders
		src.ExecFillRate = exec.FillRate
		src.ExecDataNote = exec.DataSourceNote
		if strings.Contains(strings.ToLower(exec.DataSourceNote), "legacy") {
			src.ExecDataSource = papertrading.ExecutionReadSourceLegacyFallback
			scope = "paper_legacy"
		} else if exec.Enabled || exec.TotalOrders > 0 {
			src.ExecDataSource = papertrading.ExecutionReadSourcePaperSim
			scope = "paper_sim"
		}
		refs = append(refs, SourceRef{Kind: "execution_summary", Label: exec.DataSourceNote})
	}

	ms := marketstate.SnapshotNow()
	src.MarketState = string(ms.State)
	src.MarketTrading = ms.IsTradingDay && (ms.State == marketstate.StateOpen || ms.State == marketstate.StateClose)
	refs = append(refs, SourceRef{Kind: "market_session", Label: string(ms.State)})

	return src, refs, scope, nil
}
