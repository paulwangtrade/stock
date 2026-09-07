package providershadow

import (
	"go-stock/backend/allocationengine"
	"go-stock/backend/decisionprovider"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/tradingconfig"
)

// RiskAdjustmentTrace records H0.2 risk→constraint tightening on the Portfolio
// shadow path only. It is not a TradePlan and never mutates the Legacy write chain.
type RiskAdjustmentTrace struct {
	RecordOnly          bool   `json:"record_only"`
	NotATradePlan       bool   `json:"not_a_trade_plan"`
	NotLegacyWriteChain bool   `json:"not_legacy_write_chain"`
	NotRiskViewWrite    bool   `json:"not_risk_view_write"`
	NotFilterWrite      bool   `json:"not_filter_write"`
	NotExecutionWrite   bool   `json:"not_execution_write"`

	Applied             bool   `json:"applied"`
	RiskFound           bool   `json:"risk_found"`
	InputsFingerprint   string `json:"inputs_fingerprint,omitempty"`
	SuggestHasPatches   bool   `json:"suggest_has_patches"`
	Notes               []portfoliorisk.TightenNote `json:"notes"`

	BaseReserveCashRatio      *float64 `json:"base_reserve_cash_ratio,omitempty"`
	EffectiveReserveCashRatio *float64 `json:"effective_reserve_cash_ratio,omitempty"`
	BaseMaxNewNames           *int     `json:"base_max_new_names,omitempty"`
	EffectiveMaxNewNames      *int     `json:"effective_max_new_names,omitempty"`
	BaseSkipAlreadyHolding    *bool    `json:"base_skip_already_holding,omitempty"`
	EffectiveSkipAlreadyHolding *bool  `json:"effective_skip_already_holding,omitempty"`

	// AllocationEngine is the H.3 engine observation on the Portfolio side (after tighten).
	AllocationEngine *AllocationEngineTrace `json:"allocation_engine,omitempty"`
}

// AllocationEngineTrace is a compact engine outcome for the Portfolio shadow path.
type AllocationEngineTrace struct {
	Method           string  `json:"method"`
	Binding          string  `json:"binding,omitempty"`
	UniformAmount    float64 `json:"uniform_amount"`
	AvailableCapital float64 `json:"available_capital"`
	ReserveCash      float64 `json:"reserve_cash"`
	SelectedCount    int     `json:"selected_count"`
	WaitlistCount    int     `json:"waitlist_count"`
	SchemaVersion    string  `json:"schema_version,omitempty"`
}

// PortfolioPrepare holds Portfolio-only effective constraints and the risk trace.
type PortfolioPrepare struct {
	EffectiveConstraints portfoliolayer.ConstraintSet
	Trace                *RiskAdjustmentTrace
	RiskSnapshot         *portfoliorisk.PortfolioRiskSnapshot
}

// PrepareOptions configures Portfolio-side risk integration (H0.2) for shadow only.
type PrepareOptions struct {
	Risk   *tradingconfig.RiskView
	Ledger *portfolio.Snapshot // optional; falls back to ctx.Snapshot.Ledger()
}

// PreparePortfolioSide builds PortfolioRiskSnapshot → SuggestTighten → ApplyTightenOnly.
// Legacy context/constraints are never mutated. RiskView is read-only.
func PreparePortfolioSide(ctx decisionprovider.DecisionContext, opts PrepareOptions) PortfolioPrepare {
	trace := &RiskAdjustmentTrace{
		RecordOnly:          true,
		NotATradePlan:       true,
		NotLegacyWriteChain: true,
		NotRiskViewWrite:    true,
		NotFilterWrite:      true,
		NotExecutionWrite:   true,
		Notes:               []portfoliorisk.TightenNote{},
	}
	out := PortfolioPrepare{
		EffectiveConstraints: ctx.Constraints,
		Trace:                trace,
	}

	ledger := opts.Ledger
	if ledger == nil && ctx.Snapshot != nil {
		ledger = ctx.Snapshot.Ledger()
	}
	cons := ctx.Constraints
	riskSnap := portfoliorisk.Build(portfoliorisk.BuildInput{
		Snapshot:    ledger,
		TradeDate:   "",
		Risk:        opts.Risk,
		Constraints: &cons,
	})
	out.RiskSnapshot = riskSnap
	if riskSnap != nil {
		trace.RiskFound = riskSnap.Found
		trace.InputsFingerprint = riskSnap.InputsFingerprint
	}

	sug := portfoliorisk.SuggestTighten(riskSnap, ctx.Constraints)
	trace.SuggestHasPatches = sug.HasPatches
	trace.Notes = sug.Notes
	if sug.Notes == nil {
		trace.Notes = []portfoliorisk.TightenNote{}
	}

	effective := portfoliorisk.ApplyTightenOnly(ctx.Constraints, sug.ConstraintSet)
	out.EffectiveConstraints = effective
	trace.Applied = sug.HasPatches

	trace.BaseReserveCashRatio = cloneFloat(ctx.Constraints.Portfolio.ReserveCashRatio)
	trace.EffectiveReserveCashRatio = cloneFloat(effective.Portfolio.ReserveCashRatio)
	trace.BaseMaxNewNames = cloneInt(ctx.Constraints.Portfolio.MaxNewNames)
	trace.EffectiveMaxNewNames = cloneInt(effective.Portfolio.MaxNewNames)
	trace.BaseSkipAlreadyHolding = cloneBool(ctx.Constraints.Portfolio.SkipAlreadyHolding)
	trace.EffectiveSkipAlreadyHolding = cloneBool(effective.Portfolio.SkipAlreadyHolding)

	trace.AllocationEngine = observeAllocationEngine(ctx, effective)
	return out
}

// observeAllocationEngine runs H.3 AllocationEngine on the tightened Portfolio path
// for the Risk Adjustment Trace. Does not write TradePlan or touch Legacy.
func observeAllocationEngine(ctx decisionprovider.DecisionContext, effective portfoliolayer.ConstraintSet) *AllocationEngineTrace {
	if ctx.Selection == nil {
		return nil
	}
	resolved := effective.Resolve()
	if ctx.Selection.SelectionLimit > 0 &&
		effective.User.MaxNewNames == nil &&
		effective.Strategy.MaxNewNames == nil &&
		effective.Portfolio.MaxNewNames == nil {
		resolved.MaxNewNames = ctx.Selection.SelectionLimit
	}
	sel := portfoliolayer.SelectPortfolio(portfoliolayer.PortfolioSelectionInput{
		RankedCandidates: ctx.Selection.RankedCandidates,
		Snapshot:         ctx.Snapshot,
		Constraints:      resolved.AsPortfolioConstraints(),
	})
	engineSel := allocationengine.Selection{}
	if sel != nil {
		for _, p := range sel.Selected {
			engineSel.Selected = append(engineSel.Selected, p.Candidate.StockCode)
		}
		for _, p := range sel.Waitlist {
			engineSel.Waitlist = append(engineSel.Waitlist, p.Candidate.StockCode)
		}
	}
	engSnap := &allocationengine.Snapshot{Found: false}
	if ctx.Snapshot != nil {
		engSnap = &allocationengine.Snapshot{
			Found:        ctx.Snapshot.Found,
			Equity:       ctx.Snapshot.Equity,
			Cash:         ctx.Snapshot.Cash,
			ReservedCash: ctx.Snapshot.ReservedCash,
			Exposure:     ctx.Snapshot.Exposure,
		}
	}
	policy := allocationengine.Policy{
		MaxGrossExposurePct: resolved.MaxGrossExposurePct,
		MaxSingleWeight:     resolved.MaxSingleWeight,
		ReserveCashRatio:    resolved.ReserveCashRatio,
		MinOrderAmount:      resolved.MinOrderAmount,
		BlockNewEntries:     resolved.RiskBlockNewEntries,
	}
	eng := allocationengine.RunWithComputedBudget(engineSel, engSnap, policy, allocationengine.Options{})
	if eng == nil {
		return nil
	}
	return &AllocationEngineTrace{
		Method:           eng.Method,
		Binding:          eng.Budget.Binding,
		UniformAmount:    eng.UniformAmount,
		AvailableCapital: eng.Budget.AvailableCapital,
		ReserveCash:      eng.Budget.ReserveCash,
		SelectedCount:    len(eng.Allocated),
		WaitlistCount:    len(eng.Waitlist),
		SchemaVersion:    allocationengine.SchemaVersion,
	}
}

func cloneFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func cloneInt(p *int) *int {
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
