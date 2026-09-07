package riskallocshadow

import (
	"math"

	"go-stock/backend/allocationengine"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/tradingconfig"
)

const (
	SchemaVersion = "risk_alloc_shadow.h0-2+h3"

	DefaultLegacyAmount = 100_000.0
)

// Input drives the Shadow-only Risk → Tighten → Allocation pipeline.
// Does not write TradePlan, RiskView, or Execution.
type Input struct {
	Ledger              *portfolio.Snapshot
	Risk                *tradingconfig.RiskView // read-only对照
	Constraints         portfoliolayer.ConstraintSet
	Selection           allocationengine.SelectionResult
	LegacyAmountPerName float64 // ≤0 → DefaultLegacyAmount
	TradeDate           string
}

// BudgetView is a persist-safe budget snapshot.
type BudgetView struct {
	AvailableCash    float64 `json:"available_cash"`
	ReserveCash      float64 `json:"reserve_cash"`
	RiskBudget       float64 `json:"risk_budget"`
	AvailableCapital float64 `json:"available_capital"`
	Binding          string  `json:"binding,omitempty"`
	PolicyGrossPct   float64 `json:"policy_gross_pct,omitempty"`
}

// ScalarChange records before/after for one constraint or budget scalar.
type ScalarChange struct {
	Before float64 `json:"before"`
	After  float64 `json:"after"`
	Delta  float64 `json:"delta"`
}

// IntChange records before/after for integer constraints (e.g. max_new_names).
type IntChange struct {
	Before int `json:"before"`
	After  int `json:"after"`
	Delta  int `json:"delta"`
}

// SectorConstraintChange is only meaningful when sector.available=true.
type SectorConstraintChange struct {
	Available               bool    `json:"available"`
	Applied                 bool    `json:"applied"`
	MaxSectorWeightBefore   float64 `json:"max_sector_weight_before"`
	MaxSectorWeightAfter    float64 `json:"max_sector_weight_after"`
	MaxNamesPerSectorBefore int     `json:"max_names_per_sector_before"`
	MaxNamesPerSectorAfter  int     `json:"max_names_per_sector_after"`
	Note                    string  `json:"note,omitempty"`
}

// AllocSideSummary is one allocation side (Legacy fixed vs Risk-adjusted Portfolio).
type AllocSideSummary struct {
	OK               bool       `json:"ok"`
	Method           string     `json:"method"`
	SelectedCount    int        `json:"selected_count"`
	WaitlistCount    int        `json:"waitlist_count"`
	UniformOrScalar  float64    `json:"uniform_or_scalar"`
	SumAllocationSet float64    `json:"sum_allocation_set"`
	Budget           BudgetView `json:"budget"`
	Binding          string     `json:"binding,omitempty"`
}

// AmountDiff is Legacy vs Risk-adjusted Portfolio per symbol.
type AmountDiff struct {
	Symbol          string   `json:"symbol"`
	LegacyAmount    *float64 `json:"legacy_amount,omitempty"`
	PortfolioAmount *float64 `json:"portfolio_amount,omitempty"`
	Delta           *float64 `json:"delta,omitempty"` // port − legacy when both present
	Kind            string   `json:"kind"`
}

// RiskAllocationShadowReport is the H0.2+H.3 joint Shadow artifact.
// record_only — never a TradePlan.
type RiskAllocationShadowReport struct {
	SchemaVersion      string `json:"schema_version"`
	RecordOnly         bool   `json:"record_only"`
	NotATradePlan      bool   `json:"not_a_trade_plan"`
	NotRiskViewWrite   bool   `json:"not_risk_view_write"`
	NotFilterWrite     bool   `json:"not_filter_write"`
	NotExecutionWrite  bool   `json:"not_execution_write"`
	TightenOnlyApplied bool   `json:"tighten_only_applied"`

	TradeDate         string `json:"trade_date,omitempty"`
	InputsFingerprint string `json:"inputs_fingerprint,omitempty"`
	RiskFound         bool   `json:"risk_found"`

	OriginalBudget     BudgetView `json:"original_budget"`
	RiskAdjustedBudget BudgetView `json:"risk_adjusted_budget"`

	ReserveChange   ScalarChange           `json:"reserve_change"`
	MaxNamesChange  IntChange              `json:"max_names_change"`
	SingleCapChange ScalarChange           `json:"single_cap_change"`
	SectorChange    SectorConstraintChange `json:"sector_constraint_change"`

	RiskGrossCeilingBefore  float64 `json:"risk_gross_ceiling_before"`
	RiskGrossCeilingAfter   float64 `json:"risk_gross_ceiling_after"`
	RiskSingleCeilingBefore float64 `json:"risk_single_ceiling_before"`
	RiskSingleCeilingAfter  float64 `json:"risk_single_ceiling_after"`
	RiskCeilingPreserved    bool    `json:"risk_ceiling_preserved"`

	Legacy                AllocSideSummary `json:"legacy_allocation"`
	RiskAdjustedPortfolio AllocSideSummary `json:"risk_adjusted_portfolio_allocation"`
	AmountDiffs           []AmountDiff     `json:"amount_diffs"`

	TightenNotes []portfoliorisk.TightenNote `json:"tighten_notes"`
	Notes        []string                    `json:"notes,omitempty"`
}

// Build runs PortfolioRiskSnapshot → SuggestTighten → ApplyTightenOnly → AllocationEngine
// and compares Legacy fixed_amount projection vs Risk-adjusted Portfolio allocation.
// Mutates neither Input.Constraints nor RiskView.
func Build(in Input) *RiskAllocationShadowReport {
	out := &RiskAllocationShadowReport{
		SchemaVersion:      SchemaVersion,
		RecordOnly:         true,
		NotATradePlan:      true,
		NotRiskViewWrite:   true,
		NotFilterWrite:     true,
		NotExecutionWrite:  true,
		TightenOnlyApplied: true,
		TradeDate:          in.TradeDate,
		AmountDiffs:        []AmountDiff{},
		TightenNotes:       []portfoliorisk.TightenNote{},
		Notes:              []string{},
	}

	base := in.Constraints
	consCopy := base
	riskSnap := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:    in.Ledger,
		TradeDate:   in.TradeDate,
		Risk:        in.Risk,
		Constraints: &consCopy,
	})
	if riskSnap != nil {
		out.RiskFound = riskSnap.Found
		out.InputsFingerprint = riskSnap.InputsFingerprint
	}

	baseResolved := base.Resolve()
	engSnap := toEngineSnapshot(in.Ledger)
	basePolicy := toEnginePolicy(baseResolved)
	origBudget := allocationengine.ComputeBudget(engSnap, basePolicy)
	out.OriginalBudget = budgetView(origBudget)

	sug := portfoliorisk.SuggestTighten(riskSnap, base)
	out.TightenNotes = sug.Notes
	if out.TightenNotes == nil {
		out.TightenNotes = []portfoliorisk.TightenNote{}
	}

	effective := portfoliorisk.ApplyTightenOnly(base, sug.ConstraintSet)
	// Hard guarantee: Risk + Execution layers unchanged from base (ApplyTightenOnly already does this).
	effective.Risk = base.Risk
	effective.Execution = base.Execution

	adjResolved := effective.Resolve()
	adjPolicy := toEnginePolicy(adjResolved)
	adjBudget := allocationengine.ComputeBudget(engSnap, adjPolicy)
	out.RiskAdjustedBudget = budgetView(adjBudget)

	out.ReserveChange = ScalarChange{
		Before: origBudget.ReserveCash,
		After:  adjBudget.ReserveCash,
		Delta:  adjBudget.ReserveCash - origBudget.ReserveCash,
	}
	out.MaxNamesChange = IntChange{
		Before: baseResolved.MaxNewNames,
		After:  adjResolved.MaxNewNames,
		Delta:  adjResolved.MaxNewNames - baseResolved.MaxNewNames,
	}
	out.SingleCapChange = ScalarChange{
		Before: baseResolved.MaxSingleWeight,
		After:  adjResolved.MaxSingleWeight,
		Delta:  adjResolved.MaxSingleWeight - baseResolved.MaxSingleWeight,
	}
	out.SectorChange = sectorChange(riskSnap, baseResolved, adjResolved, sug)

	out.RiskGrossCeilingBefore = baseResolved.MaxGrossExposurePct
	out.RiskGrossCeilingAfter = adjResolved.MaxGrossExposurePct
	out.RiskSingleCeilingBefore = baseResolved.MaxSingleWeight
	out.RiskSingleCeilingAfter = adjResolved.MaxSingleWeight
	out.RiskCeilingPreserved =
		adjResolved.MaxGrossExposurePct <= baseResolved.MaxGrossExposurePct+1e-12 &&
			adjResolved.MaxSingleWeight <= baseResolved.MaxSingleWeight+1e-12

	adjAlloc := allocationengine.Allocate(allocationengine.EngineInput{
		Selection: in.Selection,
		Snapshot:  engSnap,
		Budget:    &adjBudget,
		Resolved:  adjPolicy,
		Options:   allocationengine.Options{},
	})
	out.RiskAdjustedPortfolio = summarizeEngine(adjAlloc, "equal_weight")

	legacyAmt := in.LegacyAmountPerName
	if legacyAmt <= 0 || math.IsNaN(legacyAmt) || math.IsInf(legacyAmt, 0) {
		legacyAmt = DefaultLegacyAmount
	}
	out.Legacy = summarizeLegacy(in.Selection, legacyAmt)
	out.AmountDiffs = diffAmounts(out.Legacy, adjAlloc, in.Selection, legacyAmt)

	if !out.RiskCeilingPreserved {
		out.Notes = append(out.Notes, "risk_ceiling_regression_detected")
	}
	return out
}
