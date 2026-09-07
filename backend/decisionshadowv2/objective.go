package decisionshadowv2

import (
	"fmt"

	"go-stock/backend/portfoliolayer"
)

// CompileAndResolve maps Objective onto Preference layers and intersects with
// RiskCeilingTemplate (Objective cannot raise Risk ceilings).
func CompileAndResolve(obj PortfolioObjective, ceiling portfoliolayer.ConstraintSet) (portfoliolayer.ConstraintSet, portfoliolayer.ResolvedConstraints, string) {
	cs := ceiling
	// Objective preferences land on Portfolio / User preference layers only.
	if obj.MaxGrossExposurePct != nil {
		v := *obj.MaxGrossExposurePct
		cs.Portfolio.MaxGrossExposurePct = &v
		cs.User.MaxGrossExposurePct = &v
	}
	if obj.MaxSingleWeight != nil {
		v := *obj.MaxSingleWeight
		cs.Portfolio.MaxSingleWeight = &v
		cs.User.MaxSingleWeight = &v
	}
	if obj.ReserveCashRatio != nil {
		v := *obj.ReserveCashRatio
		cs.Portfolio.ReserveCashRatio = &v
		cs.User.ReserveCashRatio = &v
	}
	if obj.MaxNewNames != nil {
		v := *obj.MaxNewNames
		cs.Portfolio.MaxNewNames = &v
		cs.User.MaxNewNames = &v
	}
	if obj.BlockNewEntries != nil && *obj.BlockNewEntries {
		// Tighten-only: Objective may raise block_new, never clear a Risk ceiling block.
		cs.Risk.BlockNewEntries = true
	}
	if obj.SkipAlreadyHolding != nil {
		v := *obj.SkipAlreadyHolding
		cs.Portfolio.SkipAlreadyHolding = &v
		cs.User.SkipAlreadyHolding = &v
	}

	resolved := cs.Resolve()
	summary := fmt.Sprintf(
		"objective→resolved max_new=%d reserve=%.4f gross=%.4f single=%.4f block_new=%v",
		resolved.MaxNewNames, resolved.ReserveCashRatio, resolved.MaxGrossExposurePct,
		resolved.MaxSingleWeight, resolved.RiskBlockNewEntries,
	)
	return cs, resolved, summary
}
