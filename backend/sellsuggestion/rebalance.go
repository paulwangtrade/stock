package sellsuggestion

import (
	"strings"

	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

type rebAnnot struct {
	note string
	risk string
}

func runRebalanceContrast(in Input, opt Options) *rebalance.RebalanceSuggestion {
	if in.RebalanceCurrent == nil || in.RebalanceTarget == nil {
		return nil
	}
	cur := *in.RebalanceCurrent
	if !cur.Found {
		return nil
	}
	return rebalance.Calculate(rebalance.RebalanceInput{
		AsOf:          firstTime(in.AsOf, in.Observation.AsOf),
		TradeDate:     strings.TrimSpace(in.TradeDate),
		Current:       cur,
		Target:        *in.RebalanceTarget,
		PortfolioRisk: in.PortfolioRisk,
		Options:       opt.RebalanceOptions,
	})
}

func indexRebalance(reb *rebalance.RebalanceSuggestion) map[string]rebAnnot {
	out := map[string]rebAnnot{}
	if reb == nil {
		return out
	}
	for _, r := range reb.Reduces {
		sym := normSym(r.Symbol)
		out[sym] = rebAnnot{
			note: "rebalance:REDUCE:" + r.Reason,
			risk: joinCodesRisk(r.ReasonCodes, r.Reason),
		}
	}
	for _, e := range reb.Exits {
		sym := normSym(e.Symbol)
		out[sym] = rebAnnot{
			note: "rebalance:EXIT:" + e.Reason,
			risk: joinCodesRisk(e.ReasonCodes, e.Reason),
		}
	}
	return out
}

func joinCodesRisk(codes []string, reason string) string {
	parts := make([]string, 0, len(codes)+1)
	seen := map[string]struct{}{}
	push := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		parts = append(parts, s)
	}
	for _, c := range codes {
		push(c)
	}
	if isRiskish(reason) {
		push(reason)
	}
	return strings.Join(parts, ";")
}

// TargetFromRebalance projects H4 TargetPortfolio → sellallocation.TargetPortfolio.
func TargetFromRebalance(tgt *rebalance.TargetPortfolio) *sellallocation.TargetPortfolio {
	if tgt == nil {
		return nil
	}
	out := &sellallocation.TargetPortfolio{
		EquityRef: tgt.EquityRef,
		Names:     make([]sellallocation.TargetName, 0, len(tgt.Positions)),
	}
	for _, p := range tgt.Positions {
		sym := normSym(p.Symbol)
		if sym == "" {
			continue
		}
		out.Names = append(out.Names, sellallocation.TargetName{
			Symbol:       sym,
			TargetWeight: p.TargetWeight,
		})
	}
	return out
}
