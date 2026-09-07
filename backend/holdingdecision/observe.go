package holdingdecision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go-stock/backend/papertrading"
)

// Observe produces H.1/H1.2 HoldingDecision actions from holdings + position state + evaluation
// (+ optional score / risk / trend). Pure observation: never calls Execution or creates SellTradePlan.
// When Policy.UseRuleEngine is true, uses H1.2 Fact→Rules→Merger; otherwise H1.1 switch (default HOLD).
func Observe(in ObservationInput) *ActionView {
	pol := in.Policy
	// Observation slice never persists sell plans regardless of input flag.
	pol.PersistSellPlans = false
	if pol.DefaultReduceFraction <= 0 || pol.DefaultReduceFraction >= 1 {
		pol.DefaultReduceFraction = 0.5
	}

	schema := ActionSchemaVersion
	note := actionObserveNote
	if pol.UseRuleEngine {
		schema = ActionSchemaVersionH12
		note = actionObserveNoteH12
	}

	out := &ActionView{
		SchemaVersion:    schema,
		TradeDate:        strings.TrimSpace(in.TradeDate),
		PersistSellPlans: false,
		ReduceEnabled:    pol.ReduceEnabled,
		ExitEnabled:      pol.ExitEnabled,
		RecordOnly:       true,
		NotAnOrder:       true,
		NotExecution:     true,
		NotSellTradePlan: true,
		NotBuyChain:      true,
		SuggestOnly:      true,
		RuleEngineUsed:   pol.UseRuleEngine,
		ByAction: map[string]int{
			ActionHold:   0,
			ActionReduce: 0,
			ActionExit:   0,
		},
		Decisions:      []ActionDecision{},
		DataSourceNote: note,
	}
	if !in.AsOf.IsZero() {
		out.AsOf = in.AsOf.UTC().Format(time.RFC3339)
	}

	symbols := collectSymbols(in)
	sort.Strings(symbols)
	for _, sym := range symbols {
		var d ActionDecision
		if pol.UseRuleEngine {
			d = decideActionWithRules(sym, in, pol)
		} else {
			d = decideAction(sym, in, pol)
			d.SuggestOnly = true
		}
		out.Decisions = append(out.Decisions, d)
		out.ByAction[d.Action]++
	}
	out.InputsFingerprint = actionFingerprint(in, pol, out)
	return out
}

func collectSymbols(in ObservationInput) []string {
	seen := map[string]struct{}{}
	push := func(s string) {
		s = normSym(s)
		if s == "" {
			return
		}
		seen[s] = struct{}{}
	}
	for _, h := range in.Holdings {
		push(h.Symbol)
	}
	for k := range in.Evaluations {
		push(k)
	}
	for k := range in.PositionStates {
		push(k)
	}
	for k := range in.TrendFacts {
		push(k)
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	return out
}

func decideAction(sym string, in ObservationInput, pol ActionPolicy) ActionDecision {
	hold := lookupHolding(in.Holdings, sym)
	pos := lookupPos(in.PositionStates, sym)
	evFact := lookupEval(in.Evaluations, sym)
	score := lookupScore(in.StrategyScores, sym)

	ev := Evidence{
		CurrentPrice: evFact.CurrentPrice,
		ReturnRate:   evFact.ReturnRate,
		HoldingDays:  evFact.HoldingDays,
		RiskState:    evFact.RiskState,
		ProfitState:  evFact.ProfitState,
		PeriodState:  evFact.PeriodState,
		QuoteSource:  evFact.QuoteSource,
	}
	if ev.HoldingDays == 0 && pos.HoldingDays > 0 {
		ev.HoldingDays = pos.HoldingDays
	}
	if ev.ProfitState == "" {
		ev.ProfitState = papertrading.ClassifyProfitState(ev.ReturnRate)
	}
	if ev.RiskState == "" && ev.ReturnRate != nil {
		ev.RiskState = papertrading.ClassifyRiskState(ev.ReturnRate, papertrading.DefaultRiskStateThresholds)
	}

	// Reuse D.8 state projection (observation grade), with ExitCandidate only when that flag is on.
	base := decideFacts(sym, 0, 0, ev, Policy{ExitCandidateEnabled: pol.ExitCandidateEnabled})

	d := ActionDecision{
		Symbol:           sym,
		State:            base.State,
		Action:           ActionHold,
		Reason:           base.Reason,
		ReasonCodes:      append([]string{}, base.ReasonCodes...),
		Summary:          base.Summary,
		Evidence:         ev,
		Weight:           hold.Weight,
		StrategyScore:    score.Score,
		ExecutableHint:   false,
		RecordOnly:       true,
		NotAnOrder:       true,
		PersistSellPlans: false,
	}

	missing := ev.CurrentPrice == nil && ev.ReturnRate == nil
	if missing && hold.TotalQty <= 0 && pos.TotalQty <= 0 {
		d.Reason = ReasonDataMissing
		d.ReasonCodes = []string{ReasonDataMissing}
		d.Summary = "事实不足，维持 HOLD 观察"
		d.ExecutableHint = false
		return d
	}

	risk := strings.ToUpper(strings.TrimSpace(ev.RiskState))
	period := strings.ToUpper(strings.TrimSpace(ev.PeriodState))
	cap := concentrationCap(pol, in.RiskSnapshot)

	// Conservative action selection: EXIT > REDUCE > HOLD. Flags default off → HOLD.
	switch {
	case pol.ExitEnabled && !missing && risk == papertrading.RiskStateDanger && period == papertrading.HoldingPeriodLong:
		d.Action = ActionExit
		d.Reason = ReasonExitPolicy
		d.ReasonCodes = uniqCodes(append(d.ReasonCodes, ReasonExitPolicy))
		d.Exit = &ExitHint{Intent: ExitIntentFlattenSellable}
		d.Summary = "观察动作 EXIT（可卖部分清仓意图）；非下单、不 persist"
		// Score may reinforce HOLD only — never sole EXIT. Low score does not add EXIT alone.
		if score.Score != nil && *score.Score >= 0 {
			// keep EXIT; score is evidence only
			_ = score
		}
	case pol.ReduceEnabled && !missing && (weightAboveCap(hold.Weight, cap) || risk == papertrading.RiskStateDanger):
		d.Action = ActionReduce
		d.Reason = ReasonReducePolicy
		codes := append(d.ReasonCodes, ReasonReducePolicy)
		if weightAboveCap(hold.Weight, cap) {
			codes = append(codes, ReasonConcentration)
		}
		d.ReasonCodes = uniqCodes(codes)
		frac := pol.DefaultReduceFraction
		var targetAfter *float64
		if hold.Weight > 0 && frac > 0 && frac < 1 {
			v := hold.Weight * (1 - frac)
			targetAfter = &v
		}
		d.Reduce = &ReduceHint{Fraction: &frac, TargetWeightAfter: targetAfter}
		d.Summary = "观察动作 REDUCE；非下单、不 persist"
	default:
		d.Action = ActionHold
		if d.Summary == "" {
			d.Summary = "观察动作 HOLD"
		} else if !strings.Contains(d.Summary, "HOLD") && d.Action == ActionHold {
			d.Summary = d.Summary + "；动作 HOLD"
		}
	}

	d.ExecutableHint = executableHint(pos, hold)
	if !d.ExecutableHint && (d.Action == ActionReduce || d.Action == ActionExit) {
		d.ReasonCodes = uniqCodes(append(d.ReasonCodes, ReasonT1Locked))
		d.Summary = d.Summary + "；T+1/不可卖 → executable_hint=false"
	}
	return d
}

func executableHint(pos PositionStateFact, hold HoldingFact) bool {
	qty := pos.AvailableQty
	if qty <= 0 {
		qty = 0
	}
	total := pos.TotalQty
	if total <= 0 {
		total = hold.TotalQty
	}
	if total <= 0 {
		return false
	}
	if !pos.CanSell {
		return false
	}
	return qty > 0
}

func concentrationCap(pol ActionPolicy, risk *RiskSnapshotFact) float64 {
	if pol.ConcentrationCap > 0 {
		return pol.ConcentrationCap
	}
	if risk != nil && risk.MaxSingleNamePct != nil && *risk.MaxSingleNamePct > 0 {
		return *risk.MaxSingleNamePct
	}
	return 0
}

func weightAboveCap(weight, cap float64) bool {
	if cap <= 0 || math.IsNaN(weight) || weight <= 0 {
		return false
	}
	return weight > cap+1e-12
}

func lookupHolding(rows []HoldingFact, sym string) HoldingFact {
	for _, h := range rows {
		if normSym(h.Symbol) == sym {
			return h
		}
	}
	return HoldingFact{Symbol: sym}
}

func lookupPos(m map[string]PositionStateFact, sym string) PositionStateFact {
	if m == nil {
		return PositionStateFact{Symbol: sym}
	}
	if v, ok := m[sym]; ok {
		return v
	}
	if v, ok := m[strings.ToUpper(sym)]; ok {
		return v
	}
	return PositionStateFact{Symbol: sym}
}

func lookupEval(m map[string]EvalFact, sym string) EvalFact {
	if m == nil {
		return EvalFact{Symbol: sym}
	}
	if v, ok := m[sym]; ok {
		return v
	}
	return EvalFact{Symbol: sym}
}

func lookupScore(m map[string]StrategyScoreFact, sym string) StrategyScoreFact {
	if m == nil {
		return StrategyScoreFact{Symbol: sym}
	}
	if v, ok := m[sym]; ok {
		return v
	}
	return StrategyScoreFact{Symbol: sym}
}

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func uniqCodes(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	if len(out) == 0 {
		return []string{ReasonNone}
	}
	return out
}

func actionFingerprint(in ObservationInput, pol ActionPolicy, out *ActionView) string {
	type row struct {
		Sym   string   `json:"s"`
		W     float64  `json:"w"`
		Can   bool     `json:"can"`
		Avail int64    `json:"aq"`
		Risk  string   `json:"r"`
		Per   string   `json:"p"`
		Ret   *float64 `json:"ret"`
		Score *float64 `json:"sc"`
	}
	payload := struct {
		Schema  string   `json:"schema"`
		Date    string   `json:"date"`
		Reduce  bool     `json:"reduce"`
		Exit    bool     `json:"exit"`
		Persist bool     `json:"persist"`
		Cap     float64  `json:"cap"`
		Rows    []row    `json:"rows"`
		Actions []string `json:"actions"`
	}{
		Schema:  ActionSchemaVersion,
		Date:    out.TradeDate,
		Reduce:  pol.ReduceEnabled,
		Exit:    pol.ExitEnabled,
		Persist: false,
		Cap:     concentrationCap(pol, in.RiskSnapshot),
	}
	for _, d := range out.Decisions {
		payload.Actions = append(payload.Actions, d.Symbol+":"+d.Action+":"+fmt.Sprintf("%v", d.ExecutableHint))
		var ret *float64
		if d.Evidence.ReturnRate != nil {
			v := *d.Evidence.ReturnRate
			ret = &v
		}
		pos := lookupPos(in.PositionStates, d.Symbol)
		payload.Rows = append(payload.Rows, row{
			Sym: d.Symbol, W: d.Weight, Can: pos.CanSell, Avail: pos.AvailableQty,
			Risk: d.Evidence.RiskState, Per: d.Evidence.PeriodState, Ret: ret, Score: d.StrategyScore,
		})
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
