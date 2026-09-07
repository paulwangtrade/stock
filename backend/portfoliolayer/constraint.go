package portfoliolayer

import (
	"math"
	"strconv"

	"go-stock/backend/allocation"
	"go-stock/backend/selection"
)

// Constraint classes (F.2). Source category is not the same as the consuming layer.
const (
	ClassUserPreference = "user_preference"
	ClassStrategy       = "strategy"
	ClassPortfolio      = "portfolio"
	ClassRisk           = "risk"
	ClassExecution      = "execution"
)

const (
	SourceUser          = "user"
	SourceStrategy      = "strategy"
	SourcePortfolio     = "portfolio"
	SourceRisk          = "risk"
	SourceSystemDefault = "system_default"
	SourceCeiling       = "risk_ceiling"
)

const (
	ConsumerPortfolioSelection = "portfolio_selection"
	ConsumerAllocation         = "allocation"
	ConsumerPlanFilter         = "plan_filter"
	ConsumerExecution          = "execution"
)

const (
	DefaultMaxNewNames         = selection.DefaultMaxSelectedNames
	DefaultMaxGrossExposurePct = allocation.DefaultMaxGrossExposurePct
	DefaultMaxSingleNamePct    = 0.20
)

// PreferenceLayer is a tighten-only preference / policy slot (User, Strategy, or Portfolio).
// Nil pointers mean unset. Attempted loosening of Risk ceilings is ignored at Resolve.
type PreferenceLayer struct {
	MaxNewNames         *int
	SkipAlreadyHolding  *bool
	AllowAddToHolding   *bool
	MaxSectorWeight     *float64
	MaxNamesPerSector   *int
	ReserveCashRatio    *float64
	MinOrderAmount      *float64
	MaxSingleWeight     *float64
	MaxGrossExposurePct *float64
}

// RiskLayer is the hard ceiling. User/Strategy cannot widen these values.
type RiskLayer struct {
	MaxGrossExposurePct *float64
	MaxSingleNamePct    *float64
	MarketLevel         int
	BlockNewEntries     bool
	MaxDailyLossPct     float64
	Enabled             bool
}

// ExecutionLayer is stored for completeness. Generate-time Select/Allocate must not consume it.
type ExecutionLayer struct {
	MinLot int64
}

// ConstraintSet is the F.2 source bundle before resolve.
type ConstraintSet struct {
	User      PreferenceLayer
	Strategy  PreferenceLayer
	Portfolio PreferenceLayer
	Risk      RiskLayer
	Execution ExecutionLayer
}

// ConstraintTrace records who supplied the effective (tightest) value.
type ConstraintTrace struct {
	Field  string
	Value  string
	Source string
	Note   string
}

// ResolvedConstraints is the F.2 effective set consumed by Select/Allocate.
type ResolvedConstraints struct {
	MaxNewNames         int
	SkipAlreadyHolding  bool
	AllowAddToHolding   bool
	MaxSectorWeight     float64
	MaxNamesPerSector   int
	ReserveCashRatio    float64
	MinOrderAmount      float64
	MaxSingleWeight     float64
	MaxGrossExposurePct float64
	RiskMarketLevel     int
	RiskBlockNewEntries bool
	RiskMaxDailyLossPct float64
	Trace               []ConstraintTrace
}

// AsPortfolioConstraints projects the F.1 PortfolioConstraints subset.
func (r ResolvedConstraints) AsPortfolioConstraints() PortfolioConstraints {
	return PortfolioConstraints{
		MaxNewNames:        r.MaxNewNames,
		SkipAlreadyHolding: r.SkipAlreadyHolding,
		MaxSectorWeight:    r.MaxSectorWeight,
		MaxNamesPerSector:  r.MaxNamesPerSector,
		AllowAddToHolding:  r.AllowAddToHolding,
	}
}

// Resolve merges layers: preferences tighten; Risk is a ceiling that cannot be widened.
func (s ConstraintSet) Resolve() ResolvedConstraints {
	out := ResolvedConstraints{
		RiskMarketLevel:     s.Risk.MarketLevel,
		RiskBlockNewEntries: s.Risk.BlockNewEntries,
		RiskMaxDailyLossPct: s.Risk.MaxDailyLossPct,
	}

	out.MaxNewNames, out.Trace = minPositiveIntField(out.Trace, "max_new_names",
		intOffer{s.User.MaxNewNames, SourceUser},
		intOffer{s.Strategy.MaxNewNames, SourceStrategy},
		intOffer{s.Portfolio.MaxNewNames, SourcePortfolio},
	)
	if out.MaxNewNames == 0 {
		out.MaxNewNames = DefaultMaxNewNames
		out.Trace = append(out.Trace, ConstraintTrace{
			Field: "max_new_names", Value: strconv.Itoa(out.MaxNewNames),
			Source: SourceSystemDefault, Note: "default",
		})
	}

	out.SkipAlreadyHolding, out.Trace = orTrueField(out.Trace, "skip_already_holding",
		boolOffer{s.User.SkipAlreadyHolding, SourceUser},
		boolOffer{s.Strategy.SkipAlreadyHolding, SourceStrategy},
		boolOffer{s.Portfolio.SkipAlreadyHolding, SourcePortfolio},
	)
	out.AllowAddToHolding, out.Trace = orTrueField(out.Trace, "allow_add_to_holding",
		boolOffer{s.User.AllowAddToHolding, SourceUser},
		boolOffer{s.Strategy.AllowAddToHolding, SourceStrategy},
		boolOffer{s.Portfolio.AllowAddToHolding, SourcePortfolio},
	)
	if out.SkipAlreadyHolding {
		out.AllowAddToHolding = false
		out.Trace = append(out.Trace, ConstraintTrace{
			Field: "allow_add_to_holding", Value: "false",
			Source: SourceUser, Note: "forced_false_when_skip_already_holding",
		})
	}

	out.MaxSectorWeight, out.Trace = minPositiveFloatField(out.Trace, "max_sector_weight",
		floatOffer{s.User.MaxSectorWeight, SourceUser},
		floatOffer{s.Strategy.MaxSectorWeight, SourceStrategy},
		floatOffer{s.Portfolio.MaxSectorWeight, SourcePortfolio},
	)
	out.MaxNamesPerSector, out.Trace = minPositiveIntField(out.Trace, "max_names_per_sector",
		intOffer{s.User.MaxNamesPerSector, SourceUser},
		intOffer{s.Strategy.MaxNamesPerSector, SourceStrategy},
		intOffer{s.Portfolio.MaxNamesPerSector, SourcePortfolio},
	)
	out.ReserveCashRatio, out.Trace = maxPositiveFloatField(out.Trace, "reserve_cash_ratio",
		floatOffer{s.User.ReserveCashRatio, SourceUser},
		floatOffer{s.Strategy.ReserveCashRatio, SourceStrategy},
		floatOffer{s.Portfolio.ReserveCashRatio, SourcePortfolio},
	)
	out.MinOrderAmount, out.Trace = maxPositiveFloatField(out.Trace, "min_order_amount",
		floatOffer{s.User.MinOrderAmount, SourceUser},
		floatOffer{s.Strategy.MinOrderAmount, SourceStrategy},
		floatOffer{s.Portfolio.MinOrderAmount, SourcePortfolio},
	)

	riskGross := positiveOr(s.Risk.MaxGrossExposurePct, DefaultMaxGrossExposurePct)
	prefGross, prefGrossSrc := minPositiveFloat(
		floatOffer{s.User.MaxGrossExposurePct, SourceUser},
		floatOffer{s.Strategy.MaxGrossExposurePct, SourceStrategy},
		floatOffer{s.Portfolio.MaxGrossExposurePct, SourcePortfolio},
	)
	out.MaxGrossExposurePct = riskGross
	out.Trace = append(out.Trace, ConstraintTrace{
		Field: "max_gross_exposure_pct", Value: fmtFloat(riskGross),
		Source: SourceRisk, Note: "ceiling",
	})
	if prefGross > 0 && prefGross < riskGross {
		out.MaxGrossExposurePct = prefGross
		out.Trace = append(out.Trace, ConstraintTrace{
			Field: "max_gross_exposure_pct", Value: fmtFloat(prefGross),
			Source: prefGrossSrc, Note: "tightened_within_ceiling",
		})
	}

	riskSingle := positiveOr(s.Risk.MaxSingleNamePct, DefaultMaxSingleNamePct)
	prefSingle, prefSingleSrc := minPositiveFloat(
		floatOffer{s.User.MaxSingleWeight, SourceUser},
		floatOffer{s.Strategy.MaxSingleWeight, SourceStrategy},
		floatOffer{s.Portfolio.MaxSingleWeight, SourcePortfolio},
	)
	out.MaxSingleWeight = riskSingle
	out.Trace = append(out.Trace, ConstraintTrace{
		Field: "max_single_weight", Value: fmtFloat(riskSingle),
		Source: SourceRisk, Note: "ceiling",
	})
	if prefSingle > 0 && prefSingle < riskSingle {
		out.MaxSingleWeight = prefSingle
		out.Trace = append(out.Trace, ConstraintTrace{
			Field: "max_single_weight", Value: fmtFloat(prefSingle),
			Source: prefSingleSrc, Note: "tightened_within_ceiling",
		})
	}

	return out
}

type intOffer struct {
	v   *int
	src string
}

type boolOffer struct {
	v   *bool
	src string
}

type floatOffer struct {
	v   *float64
	src string
}

func minPositiveIntField(trace []ConstraintTrace, field string, offers ...intOffer) (int, []ConstraintTrace) {
	best := 0
	src := ""
	for _, o := range offers {
		if o.v == nil || *o.v <= 0 {
			continue
		}
		if best == 0 || *o.v < best {
			best = *o.v
			src = o.src
		}
	}
	if best > 0 {
		trace = append(trace, ConstraintTrace{Field: field, Value: strconv.Itoa(best), Source: src, Note: "tightest"})
	}
	return best, trace
}

func orTrueField(trace []ConstraintTrace, field string, offers ...boolOffer) (bool, []ConstraintTrace) {
	for _, o := range offers {
		if o.v != nil && *o.v {
			trace = append(trace, ConstraintTrace{Field: field, Value: "true", Source: o.src, Note: "stricter_true"})
			return true, trace
		}
	}
	return false, trace
}

func minPositiveFloatField(trace []ConstraintTrace, field string, offers ...floatOffer) (float64, []ConstraintTrace) {
	best, src := minPositiveFloat(offers...)
	if best > 0 {
		trace = append(trace, ConstraintTrace{Field: field, Value: fmtFloat(best), Source: src, Note: "tightest"})
	}
	return best, trace
}

func maxPositiveFloatField(trace []ConstraintTrace, field string, offers ...floatOffer) (float64, []ConstraintTrace) {
	best := 0.0
	src := ""
	for _, o := range offers {
		if o.v == nil || *o.v <= 0 || math.IsNaN(*o.v) {
			continue
		}
		if *o.v > best {
			best = *o.v
			src = o.src
		}
	}
	if best > 0 {
		trace = append(trace, ConstraintTrace{Field: field, Value: fmtFloat(best), Source: src, Note: "tightest"})
	}
	return best, trace
}

func minPositiveFloat(offers ...floatOffer) (float64, string) {
	best := 0.0
	src := ""
	for _, o := range offers {
		if o.v == nil || *o.v <= 0 || math.IsNaN(*o.v) {
			continue
		}
		if best == 0 || *o.v < best {
			best = *o.v
			src = o.src
		}
	}
	return best, src
}

func positiveOr(v *float64, fallback float64) float64 {
	if v == nil || *v <= 0 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return fallback
	}
	return *v
}

func fmtFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
