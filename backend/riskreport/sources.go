package riskreport

import (
	"go-stock/backend/marketstate"
)

// FetchLiveSources loads read-only inputs for Advanced Risk Report.
//
// Phase13-V.1 Beta closure: tip PaperTrading does not yet ship Holding/Exit/Execution
// observation builders. Portfolio/holding/exit/exec fields stay empty (degraded);
// market session still comes from marketstate. No trading writes.
func FetchLiveSources(tradeDate string) (ReportSources, []SourceRef, string, error) {
	_ = tradeDate
	src := ReportSources{
		HoldingRiskStates: map[string]int{},
		HoldingSymbols:    map[string][]string{},
		ExitReasonCounts:  map[string]int{},
		ExitReviewStates:  map[string]int{},
	}
	refs := []SourceRef{
		{Kind: "observation_bundle", Label: "degraded_beta_tip_no_paper_observation_builders"},
	}
	scope := "degraded"

	ms := marketstate.SnapshotNow()
	src.MarketState = string(ms.State)
	src.MarketTrading = ms.IsTradingDay && (ms.State == marketstate.StateOpen || ms.State == marketstate.StateClose)
	refs = append(refs, SourceRef{Kind: "market_session", Label: string(ms.State)})

	return src, refs, scope, nil
}
