package portfoliotarget

import (
	"strings"

	"go-stock/backend/rebalance"
	"go-stock/backend/sellallocation"
)

// ToRebalanceTarget projects this TargetPortfolio into H4 rebalance.TargetPortfolio.
func (t *TargetPortfolio) ToRebalanceTarget() rebalance.TargetPortfolio {
	out := rebalance.TargetPortfolio{
		EquityRef:        0,
		CashBufferWeight: 0,
		Positions:        []rebalance.TargetPosition{},
		ConstructionNote: "from portfoliotarget.CompileTargetPortfolio",
	}
	if t == nil {
		return out
	}
	out.AsOf = t.AsOfIntent
	out.CashBufferWeight = t.TargetCashRatio
	out.ConstructionNote = "portfoliotarget:" + t.Source + " fp=" + t.Fingerprint
	// EquityRef: derive from any positive TargetAmount/weight if present
	for _, p := range t.TargetPositions {
		if p.TargetWeight > 0 && p.TargetAmount > 0 {
			out.EquityRef = p.TargetAmount / p.TargetWeight
			break
		}
	}
	for _, p := range t.TargetPositions {
		sym := strings.ToLower(strings.TrimSpace(p.Symbol))
		if sym == "" {
			continue
		}
		out.Positions = append(out.Positions, rebalance.TargetPosition{
			Symbol:       sym,
			TargetWeight: p.TargetWeight,
			TargetAmount: p.TargetAmount,
			Source:       p.Source,
		})
	}
	return out
}

// ToSellAllocationTarget projects optional weight book for H1.3 SellAllocation.
func (t *TargetPortfolio) ToSellAllocationTarget() sellallocation.TargetPortfolio {
	out := sellallocation.TargetPortfolio{Names: []sellallocation.TargetName{}}
	if t == nil {
		return out
	}
	for _, p := range t.TargetPositions {
		sym := strings.ToLower(strings.TrimSpace(p.Symbol))
		if sym == "" {
			continue
		}
		out.Names = append(out.Names, sellallocation.TargetName{
			Symbol:       sym,
			TargetWeight: p.TargetWeight,
		})
		if out.EquityRef == 0 && p.TargetWeight > 0 && p.TargetAmount > 0 {
			out.EquityRef = p.TargetAmount / p.TargetWeight
		}
	}
	return out
}

// WeightBySymbol returns target weights keyed by normalized symbol.
func (t *TargetPortfolio) WeightBySymbol() map[string]float64 {
	out := map[string]float64{}
	if t == nil {
		return out
	}
	for _, p := range t.TargetPositions {
		sym := strings.ToLower(strings.TrimSpace(p.Symbol))
		if sym == "" {
			continue
		}
		out[sym] = p.TargetWeight
	}
	return out
}
