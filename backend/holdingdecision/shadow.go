package holdingdecision

import (
	"sort"
	"strings"
	"sync"
	"time"

	"go-stock/backend/portfoliorisk"
)

const (
	ShadowSchemaVersion = "holding_decision.shadow.h1-3"
	shadowDataNote      = "HoldingDecision Shadow H1.3 · Observe-only report; default off; no SellTradePlan; no Execution; no Position/TradePlan mutation"
)

// ShadowRuntime is the H1.3 Holding Decision Shadow sidecar.
// Production Default() has Enabled=false. Enabling only builds HoldingDecisionShadowReport.
type ShadowRuntime struct {
	mu      sync.Mutex
	enabled bool
	policy  ActionPolicy
	store   []HoldingDecisionShadowReport
	maxKeep int
}

// DefaultShadow returns a process-safe runtime with shadow disabled.
func DefaultShadow() *ShadowRuntime {
	return &ShadowRuntime{
		enabled: false,
		policy:  DefaultActionPolicy(),
		maxKeep: 32,
	}
}

var defaultShadow = DefaultShadow()

// DefaultHoldingShadow is the process-wide Holding Decision Shadow (default off).
func DefaultHoldingShadow() *ShadowRuntime { return defaultShadow }

// ResetDefaultHoldingShadow restores a fresh disabled runtime (tests).
func ResetDefaultHoldingShadow() {
	defaultShadow = DefaultShadow()
}

// Enabled reports whether shadow observation may run.
func (r *ShadowRuntime) Enabled() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled
}

// SetEnabled toggles shadow (tests / small-scope ops). Does not persist sell plans.
func (r *ShadowRuntime) SetEnabled(on bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = on
}

// SetPolicy replaces the observation policy used when shadow runs (still persist=false).
func (r *ShadowRuntime) SetPolicy(p ActionPolicy) {
	if r == nil {
		return
	}
	p.PersistSellPlans = false
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policy = p
}

// ShadowInput is the read-only fact bundle for one shadow pass.
type ShadowInput struct {
	AsOf           time.Time
	TradeDate      string
	Holdings       []HoldingFact
	PositionStates map[string]PositionStateFact
	Evaluations    map[string]EvalFact
	StrategyScores map[string]StrategyScoreFact
	RiskSnapshot   *RiskSnapshotFact
	PortfolioRisk  *portfoliorisk.PortfolioRiskSnapshot // optional; projected if RiskSnapshot nil
	// Policy overrides runtime policy when non-zero flags needed; PersistSellPlans ignored.
	Policy *ActionPolicy
}

// ReasonCount is one reason_code frequency in the shadow report.
type ReasonCount struct {
	Code  string `json:"code"`
	Count int    `json:"count"`
}

// RiskSourceStats summarizes evaluation / portfolio risk signals (observation only).
type RiskSourceStats struct {
	DangerCount        int      `json:"danger_count"`
	WatchCount         int      `json:"watch_count"`
	ConcentrationHits  int      `json:"concentration_hits"`
	T1LockedActionHits int      `json:"t1_locked_action_hits"`
	RiskSnapshotFound  bool     `json:"risk_snapshot_found"`
	MaxSingleNamePct   *float64 `json:"max_single_name_pct,omitempty"`
	GrossExposure      *float64 `json:"gross_exposure,omitempty"`
	Top1Weight         *float64 `json:"top1_weight,omitempty"`
	MarketRegime       string   `json:"market_regime,omitempty"`
	Notes              []string `json:"notes,omitempty"`
}

// PortfolioImpactStats is observational book impact of REDUCE/EXIT intents (not orders).
type PortfolioImpactStats struct {
	HoldWeightSum              float64 `json:"hold_weight_sum"`
	ReduceWeightSum            float64 `json:"reduce_weight_sum"`
	ExitWeightSum              float64 `json:"exit_weight_sum"`
	ReduceNotionalHint         float64 `json:"reduce_notional_hint"` // weight*MV proxy via MarketValue when present
	ExitNotionalHint           float64 `json:"exit_notional_hint"`
	ExecutableReduceOrExit     int     `json:"executable_reduce_or_exit"`
	NonExecutableReduceOrExit  int     `json:"non_executable_reduce_or_exit"`
	NamesOverConcentrationCap  int     `json:"names_over_concentration_cap"`
}

// HoldingDecisionShadowReport is the H1.3 shadow output. Never a SellTradePlan.
type HoldingDecisionShadowReport struct {
	SchemaVersion      string              `json:"schema_version"`
	Enabled            bool                `json:"enabled"`
	Skipped            bool                `json:"skipped,omitempty"` // true when runtime disabled
	SkipReason         string              `json:"skip_reason,omitempty"`
	AsOf               string              `json:"as_of,omitempty"`
	TradeDate          string              `json:"trade_date,omitempty"`
	RecordOnly         bool                `json:"record_only"`
	NotSellTradePlan   bool                `json:"not_sell_trade_plan"`
	NotExecution       bool                `json:"not_execution"`
	NotPositionWrite   bool                `json:"not_position_write"`
	NotTradePlanWrite  bool                `json:"not_trade_plan_write"`
	NotBuyChain        bool                `json:"not_buy_chain"`
	PersistSellPlans   bool                `json:"persist_sell_plans"` // always false
	HoldCount          int                 `json:"hold_count"`
	ReduceCount        int                 `json:"reduce_count"`
	ExitCount          int                 `json:"exit_count"`
	ReasonStats        []ReasonCount       `json:"reason_stats"`
	RiskSources        RiskSourceStats     `json:"risk_sources"`
	PortfolioImpact    PortfolioImpactStats `json:"portfolio_impact"`
	ReduceEnabled      bool                `json:"reduce_enabled"`
	ExitEnabled        bool                `json:"exit_enabled"`
	InputsFingerprint  string              `json:"inputs_fingerprint,omitempty"`
	DataSourceNote     string              `json:"data_source_note"`
	DecisionCount      int                 `json:"decision_count"`
}

// Run executes Observe when enabled and builds HoldingDecisionShadowReport.
// When disabled, returns a skipped report (no Observe). Never mutates positions or plans.
func (r *ShadowRuntime) Run(in ShadowInput) *HoldingDecisionShadowReport {
	out := emptyShadowReport()
	if r == nil || !r.Enabled() {
		out.Skipped = true
		out.SkipReason = "holding_decision_shadow_disabled"
		out.Enabled = false
		return out
	}

	pol := r.copyPolicy()
	if in.Policy != nil {
		pol = *in.Policy
	}
	pol.PersistSellPlans = false

	risk := in.RiskSnapshot
	if risk == nil && in.PortfolioRisk != nil {
		risk = RiskFactFromPortfolioRisk(in.PortfolioRisk)
	}

	view := Observe(ObservationInput{
		AsOf:           in.AsOf,
		TradeDate:      in.TradeDate,
		Holdings:       in.Holdings,
		PositionStates: in.PositionStates,
		Evaluations:    in.Evaluations,
		StrategyScores: in.StrategyScores,
		RiskSnapshot:   risk,
		Policy:         pol,
	})
	rep := BuildShadowReport(view, risk, in.Holdings)
	rep.Enabled = true
	rep.Skipped = false
	rep.ReduceEnabled = pol.ReduceEnabled
	rep.ExitEnabled = pol.ExitEnabled
	if !in.AsOf.IsZero() {
		rep.AsOf = in.AsOf.UTC().Format(time.RFC3339)
	}
	rep.TradeDate = strings.TrimSpace(in.TradeDate)

	r.mu.Lock()
	r.store = append(r.store, *rep)
	if r.maxKeep > 0 && len(r.store) > r.maxKeep {
		r.store = r.store[len(r.store)-r.maxKeep:]
	}
	r.mu.Unlock()
	return rep
}

// Reports returns a copy of retained shadow reports (newest last).
func (r *ShadowRuntime) Reports() []HoldingDecisionShadowReport {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]HoldingDecisionShadowReport, len(r.store))
	copy(out, r.store)
	return out
}

func (r *ShadowRuntime) copyPolicy() ActionPolicy {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.policy
	p.PersistSellPlans = false
	return p
}

func emptyShadowReport() *HoldingDecisionShadowReport {
	return &HoldingDecisionShadowReport{
		SchemaVersion:     ShadowSchemaVersion,
		Enabled:           false,
		RecordOnly:        true,
		NotSellTradePlan:  true,
		NotExecution:      true,
		NotPositionWrite:  true,
		NotTradePlanWrite: true,
		NotBuyChain:       true,
		PersistSellPlans:  false,
		ReasonStats:       []ReasonCount{},
		DataSourceNote:    shadowDataNote,
	}
}

// BuildShadowReport aggregates an Observe ActionView into HoldingDecisionShadowReport.
func BuildShadowReport(view *ActionView, risk *RiskSnapshotFact, holdings []HoldingFact) *HoldingDecisionShadowReport {
	out := emptyShadowReport()
	out.Enabled = true
	if view == nil {
		return out
	}
	out.AsOf = view.AsOf
	out.TradeDate = view.TradeDate
	out.InputsFingerprint = view.InputsFingerprint
	out.DecisionCount = len(view.Decisions)
	out.HoldCount = view.ByAction[ActionHold]
	out.ReduceCount = view.ByAction[ActionReduce]
	out.ExitCount = view.ByAction[ActionExit]
	out.ReduceEnabled = view.ReduceEnabled
	out.ExitEnabled = view.ExitEnabled

	mvBy := map[string]float64{}
	for _, h := range holdings {
		mvBy[normSym(h.Symbol)] = h.MarketValue
	}

	reasonTallies := map[string]int{}
	rs := RiskSourceStats{}
	if risk != nil {
		rs.RiskSnapshotFound = risk.Found
		rs.MaxSingleNamePct = cloneFloat64(risk.MaxSingleNamePct)
		rs.GrossExposure = cloneFloat64(risk.GrossExposure)
		rs.Top1Weight = cloneFloat64(risk.Top1Weight)
		rs.MarketRegime = strings.TrimSpace(risk.MarketRegime)
	}
	pi := PortfolioImpactStats{}
	cap := 0.0
	if risk != nil && risk.MaxSingleNamePct != nil {
		cap = *risk.MaxSingleNamePct
	}

	for _, d := range view.Decisions {
		for _, c := range d.ReasonCodes {
			c = strings.TrimSpace(c)
			if c == "" || c == ReasonNone {
				continue
			}
			reasonTallies[c]++
		}
		evRisk := strings.ToUpper(strings.TrimSpace(d.Evidence.RiskState))
		switch evRisk {
		case "DANGER":
			rs.DangerCount++
		case "WATCH":
			rs.WatchCount++
		}
		for _, c := range d.ReasonCodes {
			if c == ReasonConcentration {
				rs.ConcentrationHits++
			}
			if c == ReasonT1Locked {
				rs.T1LockedActionHits++
			}
		}
		if weightAboveCap(d.Weight, cap) {
			pi.NamesOverConcentrationCap++
		}
		switch d.Action {
		case ActionHold:
			pi.HoldWeightSum += d.Weight
		case ActionReduce:
			pi.ReduceWeightSum += d.Weight
			pi.ReduceNotionalHint += mvBy[normSym(d.Symbol)]
			if d.ExecutableHint {
				pi.ExecutableReduceOrExit++
			} else {
				pi.NonExecutableReduceOrExit++
			}
		case ActionExit:
			pi.ExitWeightSum += d.Weight
			pi.ExitNotionalHint += mvBy[normSym(d.Symbol)]
			if d.ExecutableHint {
				pi.ExecutableReduceOrExit++
			} else {
				pi.NonExecutableReduceOrExit++
			}
		}
	}
	if rs.DangerCount > 0 {
		rs.Notes = append(rs.Notes, "eval_danger_present")
	}
	if rs.ConcentrationHits > 0 {
		rs.Notes = append(rs.Notes, "concentration_reason_present")
	}
	if !rs.RiskSnapshotFound && risk == nil {
		rs.Notes = append(rs.Notes, "risk_snapshot_absent")
	}

	out.ReasonStats = sortReasonCounts(reasonTallies)
	out.RiskSources = rs
	out.PortfolioImpact = pi
	return out
}

func sortReasonCounts(m map[string]int) []ReasonCount {
	out := make([]ReasonCount, 0, len(m))
	for c, n := range m {
		out = append(out, ReasonCount{Code: c, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Code < out[j].Code
	})
	return out
}
