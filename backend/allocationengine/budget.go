package allocationengine

import "math"

const defaultMaxGrossExposurePct = 0.85

// ComputeBudget derives AllocationBudget from PortfolioSnapshot + ResolvedConstraints.
// Steps: effective reserve → available_cash → risk_budget → available_capital = min(cash, gross).
// Does not mutate Snapshot. Does not credit planned sells.
func ComputeBudget(snap *PortfolioSnapshot, resolved ResolvedConstraints) AllocationBudget {
	pct := resolved.MaxGrossExposurePct
	if pct <= 0 || math.IsNaN(pct) || math.IsInf(pct, 0) {
		pct = defaultMaxGrossExposurePct
	}
	out := AllocationBudget{PolicyGrossPct: pct, Binding: BindingOK}

	if snap == nil || !snap.Found {
		out.Binding = BindingNoAccount
		return out
	}

	// Reserve deduction: max(ledger reserved, cash × reserve_ratio).
	reserve := snap.ReservedCash
	if resolved.ReserveCashRatio > 0 {
		want := snap.Cash * resolved.ReserveCashRatio
		if want > reserve {
			reserve = want
		}
	}
	out.ReserveCash = reserve

	cashAvail := snap.Cash - reserve
	grossCap := pct * snap.Equity
	grossHead := grossCap - snap.Exposure
	out.AvailableCash = cashAvail
	out.RiskBudget = grossHead

	if resolved.BlockNewEntries {
		out.AvailableCapital = 0
		out.Binding = BindingBlocked
		return out
	}

	avail := cashAvail
	if grossHead < avail {
		avail = grossHead
	}
	if avail < 0 {
		avail = 0
	}
	out.AvailableCapital = avail

	if avail == 0 {
		if cashAvail <= 0 {
			out.Binding = BindingCash
		} else {
			out.Binding = BindingGross
		}
		return out
	}
	if cashAvail > 0 && grossHead > 0 && math.Abs(cashAvail-grossHead) < 1e-9 {
		out.Binding = BindingOK
	} else if cashAvail <= grossHead {
		out.Binding = BindingCash
	} else {
		out.Binding = BindingGross
	}
	return out
}
