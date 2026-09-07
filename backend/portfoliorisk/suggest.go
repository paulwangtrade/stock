package portfoliorisk

import (
	"math"
	"sort"

	"go-stock/backend/portfoliolayer"
)

const (
	headroomExhaustedEps   = 1e-6
	cashRatioLowDefault    = 0.20
	reserveFloorOnLowCash  = 0.15
	reserveFloorOnHeadroom = 0.10
)

// TightenNote explains one preference patch. Observational; not a RiskCode.
type TightenNote struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
	Detail string `json:"detail,omitempty"`
}

// TightenSuggestion is a read-only Preference patch derived from PortfolioRiskSnapshot.
// Risk / Execution layers are always empty — never mutated from measurement.
type TightenSuggestion struct {
	RecordOnly        bool                         `json:"record_only"`
	NotRiskViewWrite  bool                         `json:"not_risk_view_write"`
	NotPlanFilter     bool                         `json:"not_plan_filter"`
	NotExecutionWrite bool                         `json:"not_execution_write"`
	ConstraintSet     portfoliolayer.ConstraintSet `json:"constraint_set"`
	Notes             []TightenNote                `json:"notes"`
	HasPatches        bool                         `json:"has_patches"`
}

// SuggestTighten maps PortfolioRiskSnapshot → Preference-only ConstraintSet patch (H0.2).
// available=false blocks produce no patch for that dimension. Theme/correlation never patch.
// Does not modify RiskView, PlanFilter, or Execution.
func SuggestTighten(snap *PortfolioRiskSnapshot, _ portfoliolayer.ConstraintSet) TightenSuggestion {
	out := TightenSuggestion{
		RecordOnly:        true,
		NotRiskViewWrite:  true,
		NotPlanFilter:     true,
		NotExecutionWrite: true,
		Notes:             []TightenNote{},
	}
	if snap == nil || !snap.Found {
		return out
	}

	pref := portfoliolayer.PreferenceLayer{}

	if snap.Exposure.Available {
		if snap.Exposure.HeadroomVsCap != nil && *snap.Exposure.HeadroomVsCap <= headroomExhaustedEps {
			zero := 0
			pref.MaxNewNames = &zero
			out.Notes = append(out.Notes, TightenNote{
				Field: "max_new_names", Reason: "gross_headroom_exhausted",
				Detail: "headroom_vs_cap<=0 → stop expanding allocation set",
			})
			floor := reserveFloorOnHeadroom
			pref.ReserveCashRatio = &floor
			out.Notes = append(out.Notes, TightenNote{
				Field: "reserve_cash_ratio", Reason: "gross_headroom_exhausted",
				Detail: "raise reserve preference (tighten-only)",
			})
			if snap.Exposure.CapGross != nil && *snap.Exposure.CapGross > 0 {
				cap := *snap.Exposure.CapGross
				pref.MaxGrossExposurePct = &cap
				out.Notes = append(out.Notes, TightenNote{
					Field: "max_gross_exposure_pct", Reason: "gross_headroom_exhausted",
					Detail: "portfolio preference pinned at cap (cannot widen risk ceiling)",
				})
			}
		}
		if snap.Exposure.CashRatio != nil && *snap.Exposure.CashRatio < cashRatioLowDefault {
			floor := reserveFloorOnLowCash
			if pref.ReserveCashRatio == nil || *pref.ReserveCashRatio < floor {
				pref.ReserveCashRatio = &floor
			}
			out.Notes = append(out.Notes, TightenNote{
				Field: "reserve_cash_ratio", Reason: "cash_ratio_low",
				Detail: "cash_ratio below floor → raise reserve preference",
			})
		}
	}

	if snap.Concentration.Available {
		if snap.Concentration.Top1Weight != nil && snap.Concentration.CapSingle != nil &&
			*snap.Concentration.CapSingle > 0 &&
			*snap.Concentration.Top1Weight > *snap.Concentration.CapSingle+1e-12 {
			skip := true
			allow := false
			pref.SkipAlreadyHolding = &skip
			pref.AllowAddToHolding = &allow
			cap := *snap.Concentration.CapSingle
			pref.MaxSingleWeight = &cap
			out.Notes = append(out.Notes, TightenNote{
				Field: "skip_already_holding", Reason: "top1_over_single_cap",
				Detail: "concentration hot → prefer skip add-to-holding",
			})
			out.Notes = append(out.Notes, TightenNote{
				Field: "max_single_weight", Reason: "top1_over_single_cap",
				Detail: "pin single preference at risk single cap",
			})
		}
	}

	if snap.Sector.Available && snap.Sector.MaxSectorWeight != nil && *snap.Sector.MaxSectorWeight > 0 {
		maxW := *snap.Sector.MaxSectorWeight
		over := false
		for _, s := range snap.Sector.SectorExposure {
			if s.Weight > maxW+1e-12 {
				over = true
				break
			}
		}
		if over {
			w := maxW
			pref.MaxSectorWeight = &w
			one := 1
			pref.MaxNamesPerSector = &one
			out.Notes = append(out.Notes, TightenNote{
				Field: "max_sector_weight", Reason: "sector_over_cap",
				Detail: "sector available and over max_sector_weight",
			})
			out.Notes = append(out.Notes, TightenNote{
				Field: "max_names_per_sector", Reason: "sector_over_cap",
				Detail: "tighten names-per-sector preference",
			})
		}
	}
	// theme / correlation: unavailable → no patch (explicit)

	if snap.Market.Available && snap.Market.BlockNewEntries {
		zero := 0
		pref.MaxNewNames = &zero
		out.Notes = append(out.Notes, TightenNote{
			Field: "max_new_names", Reason: "market_block_new_entries",
			Detail: "preference stop-new-names; does not write RiskLayer or RiskView",
		})
	}

	sort.Slice(out.Notes, func(i, j int) bool {
		if out.Notes[i].Reason != out.Notes[j].Reason {
			return out.Notes[i].Reason < out.Notes[j].Reason
		}
		return out.Notes[i].Field < out.Notes[j].Field
	})

	out.ConstraintSet = portfoliolayer.ConstraintSet{
		Portfolio: pref,
		// Risk and Execution intentionally zero — ApplyTightenOnly ignores them anyway.
	}
	out.HasPatches = preferenceHasAny(pref)
	return out
}

// ApplyTightenOnly merges a SuggestTighten patch into base using tighten-only rules.
// Never widens Risk ceilings, never copies suggestion.Risk / suggestion.Execution,
// never writes RiskView. Returns a new ConstraintSet value (base is not mutated in place
// beyond Go struct copy semantics for the returned value).
func ApplyTightenOnly(base portfoliolayer.ConstraintSet, suggestion portfoliolayer.ConstraintSet) portfoliolayer.ConstraintSet {
	out := base
	out.Portfolio = mergePreferenceTighten(out.Portfolio, suggestion.Portfolio)
	out.User = mergePreferenceTighten(out.User, suggestion.User)
	out.Strategy = mergePreferenceTighten(out.Strategy, suggestion.Strategy)
	// Explicitly preserve Risk + Execution from base only.
	out.Risk = base.Risk
	out.Execution = base.Execution
	return out
}

func preferenceHasAny(p portfoliolayer.PreferenceLayer) bool {
	return p.MaxNewNames != nil || p.SkipAlreadyHolding != nil || p.AllowAddToHolding != nil ||
		p.MaxSectorWeight != nil || p.MaxNamesPerSector != nil || p.ReserveCashRatio != nil ||
		p.MinOrderAmount != nil || p.MaxSingleWeight != nil || p.MaxGrossExposurePct != nil
}

func mergePreferenceTighten(base, patch portfoliolayer.PreferenceLayer) portfoliolayer.PreferenceLayer {
	out := base
	out.MaxNewNames = minIntPtr(out.MaxNewNames, patch.MaxNewNames)
	out.MaxNamesPerSector = minIntPtr(out.MaxNamesPerSector, patch.MaxNamesPerSector)
	out.MaxSectorWeight = minFloatPtr(out.MaxSectorWeight, patch.MaxSectorWeight)
	out.MaxSingleWeight = minFloatPtr(out.MaxSingleWeight, patch.MaxSingleWeight)
	out.MaxGrossExposurePct = minFloatPtr(out.MaxGrossExposurePct, patch.MaxGrossExposurePct)
	out.MinOrderAmount = maxFloatPtr(out.MinOrderAmount, patch.MinOrderAmount) // higher min = tighter
	out.ReserveCashRatio = maxFloatPtr(out.ReserveCashRatio, patch.ReserveCashRatio)
	out.SkipAlreadyHolding = orTruePtr(out.SkipAlreadyHolding, patch.SkipAlreadyHolding)
	// AllowAddToHolding: tighter is false when skip is true; otherwise prefer false if patch says false.
	if patch.AllowAddToHolding != nil {
		if !*patch.AllowAddToHolding {
			f := false
			out.AllowAddToHolding = &f
		} else if out.AllowAddToHolding == nil {
			t := true
			out.AllowAddToHolding = &t
		}
	}
	if out.SkipAlreadyHolding != nil && *out.SkipAlreadyHolding {
		f := false
		out.AllowAddToHolding = &f
	}
	return out
}

func minIntPtr(a, b *int) *int {
	if b == nil {
		return cloneInt(a)
	}
	if a == nil {
		return cloneInt(b)
	}
	v := *a
	if *b < v {
		v = *b
	}
	return &v
}

func minFloatPtr(a, b *float64) *float64 {
	if b == nil {
		return cloneFloat(a)
	}
	if a == nil {
		return cloneFloat(b)
	}
	v := math.Min(*a, *b)
	return &v
}

func maxFloatPtr(a, b *float64) *float64 {
	if b == nil {
		return cloneFloat(a)
	}
	if a == nil {
		return cloneFloat(b)
	}
	v := math.Max(*a, *b)
	return &v
}

func orTruePtr(a, b *bool) *bool {
	if (a != nil && *a) || (b != nil && *b) {
		t := true
		return &t
	}
	if a != nil {
		return cloneBool(a)
	}
	return cloneBool(b)
}

func cloneInt(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneBool(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
