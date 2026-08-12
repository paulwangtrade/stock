package portfolioobs

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio"
	"go-stock/backend/rebalance"
)

// Assemble projects existing Snapshot + Evaluation + Decision + Diff into one observation DTO.
// Does not recompute PnL, decision rules, or Diff math; does not mutate inputs.
func Assemble(
	snap *portfolio.Snapshot,
	eval *papertrading.HoldingEvalObservationView,
	decision *holdingdecision.View,
	diff *rebalance.View,
	target *rebalance.TargetPortfolio,
	warnings []string,
	now time.Time,
) *Observation {
	if now.IsZero() {
		now = time.Now()
	}
	out := &Observation{
		ObservationTime: now.Format(time.RFC3339),
		Disclaimer:      Disclaimer,
		Action:          actionNone,
		Positions:       []PositionRow{},
		Warnings:        append([]string{}, warnings...),
		DataSourceNotes: map[string]string{
			"snapshot":   "Portfolio Snapshot · paper_sim mark_price",
			"evaluation": "Holding Evaluation · overlay quote when available",
			"decision":   holdingdecision.StateHoldNormal + " / WATCH / REVIEW · not a sell recommendation",
			"rebalance":  "Rebalance Diff · observation only; not an order",
		},
		OpportunityCost: OpportunityCostView{
			Available: false,
			Level:     OpportunityCostUnknown,
			Note:      opportunityCostNote,
		},
		Health: PortfolioHealthSummary{PortfolioHealth: HealthUnknown},
	}

	if snap != nil && !snap.AsOf.IsZero() {
		out.AsOf = snap.AsOf.Format(time.RFC3339)
	} else if eval != nil && !eval.AsOf.IsZero() {
		out.AsOf = eval.AsOf.Format(time.RFC3339)
	}

	if snap != nil {
		out.Account = AccountSummary{
			TotalEquity:   snap.TotalEquity,
			Cash:          snap.Cash,
			MarketValue:   snap.MarketValue,
			Exposure:      snap.TotalExposure,
			PositionCount: snap.PositionCount,
			Found:         snap.Found,
			AccountID:     snap.AccountID,
			AccountName:   snap.AccountName,
		}
	}

	// Reuse D.9 projection for decision counts + overlay weights (no new formulas).
	sum := holdingdecision.BuildPortfolioObservation(snap, eval, decision)
	if sum != nil {
		out.Decision = DecisionSummary{
			NormalCount:         sum.DecisionCounts.HoldNormalCount,
			WatchCount:          sum.DecisionCounts.HoldWatchCount,
			ReviewCount:         sum.DecisionCounts.HoldReviewCount,
			ExitCandidateCount:  sum.DecisionCounts.ExitCandidateCount,
			WeightBasis:         sum.DecisionMarketValue.Basis,
		}
		if sum.DecisionMarketValue.Total > 0 {
			t := sum.DecisionMarketValue.Total
			out.Decision.NormalWeight = sum.DecisionMarketValue.HoldNormal / t
			out.Decision.WatchWeight = sum.DecisionMarketValue.HoldWatch / t
			out.Decision.ReviewWeight = sum.DecisionMarketValue.HoldReview / t
			out.Decision.ExitCandidateWeight = sum.DecisionMarketValue.ExitCandidate / t
		}
	}

	if diff != nil {
		out.Rebalance = RebalanceSummary{
			KeepCount:            diff.Counts.Keep,
			AddCount:             diff.Counts.Add,
			IncreaseCount:        diff.Counts.Increase,
			DecreaseCount:        diff.Counts.Decrease,
			RemoveCount:          diff.Counts.Remove,
			BlockedSwitchCount:   diff.BlockedSwitchCount,
			Available:            true,
		}
	}
	if snap != nil {
		out.Rebalance.CurrentPositionCount = snap.PositionCount
	}
	if target != nil {
		out.Rebalance.TargetPositionCount = len(target.Positions)
	}

	out.Positions = joinPositions(snap, eval, decision, diff)
	Enhance(out)
	return out
}

func joinPositions(
	snap *portfolio.Snapshot,
	eval *papertrading.HoldingEvalObservationView,
	decision *holdingdecision.View,
	diff *rebalance.View,
) []PositionRow {
	type acc struct {
		row PositionRow
	}
	by := map[string]*acc{}
	ensure := func(sym string) *acc {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			return nil
		}
		if e, ok := by[sym]; ok {
			return e
		}
		e := &acc{row: PositionRow{Symbol: sym, Action: actionNone}}
		by[sym] = e
		return e
	}

	if snap != nil {
		for _, p := range snap.Positions {
			e := ensure(p.StockCode)
			if e == nil {
				continue
			}
			e.row.StockName = strings.TrimSpace(p.StockName)
			e.row.CurrentWeight = p.Weight
			e.row.CurrentAmount = p.MarketValue
		}
	}
	if eval != nil {
		for _, h := range eval.Holdings {
			e := ensure(h.StockCode)
			if e == nil {
				continue
			}
			if e.row.StockName == "" {
				e.row.StockName = strings.TrimSpace(h.StockName)
			}
			e.row.Cost = h.AvgCost
			e.row.CurrentPrice = h.CurrentPrice
			if e.row.CurrentPrice == nil {
				e.row.CurrentPrice = h.MarketPrice
			}
			e.row.PnL = h.UnrealizedPnL
			e.row.Return = h.UnrealizedReturn
			e.row.HoldingDays = h.HoldingDays
			e.row.FirstBuyDate = strings.TrimSpace(h.FirstBuyDate)
			e.row.RiskState = strings.TrimSpace(h.RiskState)
			e.row.ProfitState = strings.TrimSpace(h.ProfitState)
		}
	}
	if decision != nil {
		for _, h := range decision.Holdings {
			e := ensure(h.Symbol)
			if e == nil {
				continue
			}
			e.row.DecisionState = h.State
			e.row.DecisionReason = h.Reason
		}
	}
	if diff != nil {
		for _, it := range diff.Items {
			e := ensure(it.Symbol)
			if e == nil {
				continue
			}
			e.row.CurrentWeight = it.CurrentWeight
			e.row.CurrentAmount = it.CurrentAmount
			e.row.TargetWeight = it.TargetWeight
			e.row.TargetAmount = it.TargetAmount
			e.row.DeltaWeight = it.DeltaWeight
			e.row.RebalanceAction = it.Action
			e.row.RebalanceReason = it.Reason
		}
	}

	out := make([]PositionRow, 0, len(by))
	for _, e := range by {
		if e.row.DeltaWeight == 0 && (e.row.TargetWeight != 0 || e.row.CurrentWeight != 0) {
			e.row.DeltaWeight = e.row.TargetWeight - e.row.CurrentWeight
		}
		out = append(out, e.row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}
