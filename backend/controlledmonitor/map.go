package controlledmonitor

import (
	"strings"

	"go-stock/backend/allocationengine"
)

// AllocationSideFromEngine maps an H.3 AllocationResult into AllocationSide (nil-safe).
// Does not run the engine.
func AllocationSideFromEngine(res *allocationengine.AllocationResult) *AllocationSide {
	if res == nil {
		return nil
	}
	sum := 0.0
	for _, it := range res.Allocated {
		sum += it.TargetAmount
	}
	if sum == 0 {
		for _, it := range res.Items {
			if it.InAllocationSet {
				sum += it.TargetAmount
			}
		}
	}
	return &AllocationSide{
		Present:          true,
		OK:               true,
		Method:           strings.TrimSpace(res.Method),
		SelectedCount:    len(res.Allocated),
		WaitlistCount:    len(res.Waitlist),
		SumNotional:      sum,
		AvailableCapital: res.Budget.AvailableCapital,
		ReserveCash:      res.Budget.ReserveCash,
		Binding:          strings.TrimSpace(res.Budget.Binding),
	}
}

// FilterSideFromCounts builds a FilterSide from accept/reject counts (nil reasons ok).
func FilterSideFromCounts(accepted, rejected int, riskStatus string, reasons map[string]int) *FilterSide {
	return &FilterSide{
		Present:       true,
		AcceptedCount: accepted,
		RejectedCount: rejected,
		RiskStatus:    strings.TrimSpace(riskStatus),
		RejectReasons: copyReasonMap(reasons),
	}
}
