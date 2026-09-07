package positionsizing

import (
	"math"

	"go-stock/backend/allocation"
	"go-stock/backend/tradingconfig"
)

const (
	capCash         = "cash"
	capGross        = "gross"
	capSingleWeight = "single_weight"
	capMinOrder     = "min_order"
	capN            = "n"
	capFixed        = "fixed"
)

// PortfolioAwareSizer equal-splits remaining buy budget, then applies P2 caps.
//
//	usable_cash = cash × (1 − reserveCashRatio)
//	budget      = min(usable_cash, equity×maxExposure − exposure)
//	raw         = floor(budget / N)
//	final       = min(raw, equity×maxSinglePositionWeight)
//	if final < minOrderAmount → 0 (Builder: no_budget, skip FilterPool)
//
// It never queries paper_sim / DB. Callers must inject Request.Portfolio.
type PortfolioAwareSizer struct{}

// Propose returns the per-name integer yuan amount. PlannedVolume stays 0 (Draft).
func (s *PortfolioAwareSizer) Propose(req Request) SizingProposal {
	reason := &AmountReason{
		Method:         string(MethodPortfolioAware),
		CandidateCount: req.CandidateCount,
	}
	out := SizingProposal{
		PlannedAmount: 0,
		PlannedVolume: 0,
		Method:        MethodPortfolioAware,
		Source:        tradingconfig.SourceLegacyPaperMVP,
		AmountReason:  reason,
	}

	n := req.CandidateCount
	if n <= 0 {
		reason.CappedBy = []string{capN}
		return out
	}

	snap := req.Portfolio
	if snap != nil {
		reason.Equity = snap.TotalEquity
		reason.Cash = snap.Cash
		reason.CurrentExposure = snap.TotalExposure
	}

	maxExp := req.MaxGrossExposurePct
	singleW := req.MaxSinglePositionWeight
	if singleW <= 0 {
		singleW = DefaultMaxSinglePositionWeight
	}
	reserve := req.ReserveCashRatio
	if reserve < 0 || reserve >= 1 || math.IsNaN(reserve) {
		reserve = DefaultReserveCashRatio
	}
	minOrder := req.MinOrderAmount
	if minOrder <= 0 {
		minOrder = DefaultMinOrderAmount
	}

	budgetRes := allocation.Budget(allocation.CapitalAllocationRequest{
		Snapshot: snap,
		Policy: allocation.AllocationPolicy{
			MaxGrossExposurePct: maxExp,
		},
	})
	available := budgetRes.AvailableCapital
	capped := make([]string, 0, 3)
	switch budgetRes.Binding {
	case allocation.ReasonCash:
		capped = append(capped, capCash)
	case allocation.ReasonGross:
		capped = append(capped, capGross)
	}

	if snap != nil {
		usable := snap.Cash * (1 - reserve)
		if usable < 0 || math.IsNaN(usable) {
			usable = 0
		}
		if usable < available {
			available = usable
			capped = appendUnique(capped, capCash)
		}
	}
	if available < 0 || math.IsNaN(available) || math.IsInf(available, 0) {
		available = 0
	}
	reason.Budget = available

	if available <= 0 {
		reason.CappedBy = capped
		if len(reason.CappedBy) == 0 {
			reason.CappedBy = []string{capGross}
		}
		return out
	}

	raw := math.Floor(available / float64(n))
	if raw < 0 || math.IsNaN(raw) || math.IsInf(raw, 0) {
		raw = 0
	}
	reason.RawAmount = raw

	equity := 0.0
	if snap != nil {
		equity = snap.TotalEquity
	}
	maxAmount := math.Floor(equity * singleW)
	if maxAmount < 0 {
		maxAmount = 0
	}

	final := raw
	if maxAmount < final {
		final = maxAmount
		capped = appendUnique(capped, capSingleWeight)
	}

	if final < minOrder {
		reason.FinalAmount = 0
		reason.CappedBy = appendUnique(capped, capMinOrder)
		return out
	}

	reason.FinalAmount = final
	reason.CappedBy = capped
	out.PlannedAmount = final
	return out
}

func appendUnique(dst []string, v string) []string {
	for _, x := range dst {
		if x == v {
			return dst
		}
	}
	return append(dst, v)
}
