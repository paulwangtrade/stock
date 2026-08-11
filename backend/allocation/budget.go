package allocation

import "math"

// Budget computes available capital without mutating req or Snapshot.
//
//	available_capital = max(0, min(cash - reserved_cash - pending_buy, gross_cap - current_exposure))
//
// Planned sell is not an input. Negative source fields are not rewritten; budget is floored at 0.
func Budget(req CapitalAllocationRequest) CapitalAllocationResult {
	pct := req.Policy.MaxGrossExposurePct
	if pct <= 0 || math.IsNaN(pct) || math.IsInf(pct, 0) {
		pct = DefaultMaxGrossExposurePct
	}

	out := CapitalAllocationResult{
		Reason:         ReasonOK,
		PolicyGrossPct: pct,
	}

	snap := req.Snapshot
	if snap == nil || !snap.Found {
		out.Reason = ReasonNoAccount
		out.Binding = ReasonNoAccount
		return out
	}

	if req.Policy.BlockNewEntries {
		out.ExposureLimit = pct * snap.TotalEquity
		out.CashAvailable = snap.Cash - snap.ReservedCash - req.PendingBuy
		out.GrossHeadroom = out.ExposureLimit - snap.TotalExposure
		out.Reason = ReasonBlocked
		out.Binding = ReasonBlocked
		return out
	}

	if hasNegativeInput(snap.Cash, snap.ReservedCash, req.PendingBuy, snap.TotalExposure, snap.TotalEquity) {
		out.ExposureLimit = pct * snap.TotalEquity
		out.CashAvailable = snap.Cash - snap.ReservedCash - req.PendingBuy
		out.GrossHeadroom = out.ExposureLimit - snap.TotalExposure
		out.Reason = ReasonNegativeInput
		out.Binding = ReasonNegativeInput
		return out
	}

	cashAvail := snap.Cash - snap.ReservedCash - req.PendingBuy
	grossCap := pct * snap.TotalEquity
	grossHead := grossCap - snap.TotalExposure

	out.CashAvailable = cashAvail
	out.ExposureLimit = grossCap
	out.GrossHeadroom = grossHead

	avail := cashAvail
	binding := ReasonCash
	if grossHead < avail {
		avail = grossHead
		binding = ReasonGross
	}
	if avail < 0 {
		avail = 0
	}
	out.AvailableCapital = avail
	out.Binding = binding

	if avail == 0 {
		if cashAvail <= 0 {
			out.Reason = ReasonCash
			out.Binding = ReasonCash
			return out
		}
		out.Reason = ReasonGross
		out.Binding = ReasonGross
		return out
	}
	if cashAvail <= 0 {
		out.Reason = ReasonCash
		out.Binding = ReasonCash
		out.AvailableCapital = 0
		return out
	}
	out.Reason = ReasonOK
	return out
}

func hasNegativeInput(cash, reserved, pending, exposure, equity float64) bool {
	return cash < 0 || reserved < 0 || pending < 0 || exposure < 0 || equity < 0
}
