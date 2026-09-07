package providershadow

import (
	"math"
	"strings"
	"time"

	"go-stock/backend/decisionprovider"
	"go-stock/backend/selection"
)

const amountEpsilon = 1e-6

// CompareResult is G.5 CompareProviders output.
// ChainEnvelope is always Legacy (write-chain identity). PortfolioEnvelope is observation only.
type CompareResult struct {
	ChainEnvelope     *decisionprovider.DecisionEnvelope
	PortfolioEnvelope *decisionprovider.DecisionEnvelope
	Report            *ProviderComparisonReport
	RiskAdjustment    *RiskAdjustmentTrace
}

// CompareProviders runs Legacy + Portfolio on the same DecisionContext and diffs envelopes.
// Portfolio side applies H0.2 risk tighten then H.3 AllocationEngine (via Portfolio provider).
// It never calls the write-chain risk filter and never swaps chain_envelope to Portfolio.
func CompareProviders(
	ctx decisionprovider.DecisionContext,
	enabled bool,
	legacy, portfolio decisionprovider.DecisionProvider,
) CompareResult {
	return CompareProvidersPrepared(ctx, enabled, legacy, portfolio, PrepareOptions{})
}

// CompareProvidersPrepared is CompareProviders with explicit Portfolio risk prepare options.
func CompareProvidersPrepared(
	ctx decisionprovider.DecisionContext,
	enabled bool,
	legacy, portfolio decisionprovider.DecisionProvider,
	prepOpts PrepareOptions,
) CompareResult {
	legacyCtx := ctx
	legacyCtx.Version.Provider = decisionprovider.ProviderFixedAmount
	legacyEnv := decideSafe(legacy, legacyCtx, decisionprovider.ProviderFixedAmount)

	out := CompareResult{ChainEnvelope: legacyEnv}
	if !enabled {
		return out
	}

	prep := PreparePortfolioSide(ctx, prepOpts)
	out.RiskAdjustment = prep.Trace

	portCtx := ctx
	portCtx.Constraints = prep.EffectiveConstraints
	portCtx.Version.Provider = decisionprovider.ProviderPortfolioAllocation
	portEnv := decideSafe(portfolio, portCtx, decisionprovider.ProviderPortfolioAllocation)
	out.PortfolioEnvelope = portEnv

	ranked := []selection.Candidate{}
	if ctx.Selection != nil {
		ranked = ctx.Selection.RankedCandidates
	}
	report := DiffEnvelopes(legacyEnv, portEnv, ctx.DecisionTime, ranked)
	stampReportFlags(&report)
	report.RiskAdjustment = prep.Trace
	out.Report = &report
	return out
}

func decideSafe(p decisionprovider.DecisionProvider, ctx decisionprovider.DecisionContext, name string) (env *decisionprovider.DecisionEnvelope) {
	defer func() {
		if rec := recover(); rec != nil {
			env = closedFail(ctx, name, ErrRuntimePanic, panicText(rec))
		}
	}()
	if p == nil {
		return closedFail(ctx, name, decisionprovider.ErrCodeContextInvalid, "provider is nil")
	}
	got, err := p.Decide(ctx)
	if got == nil {
		msg := "nil envelope"
		if err != nil {
			msg = err.Error()
		}
		return closedFail(ctx, name, decisionprovider.ErrCodeInvalidAllocation, msg)
	}
	if err != nil && got.OK {
		got.OK = false
		if got.Error == nil {
			got.Error = &decisionprovider.DecisionError{Code: decisionprovider.ErrCodeInvalidAllocation, Message: err.Error()}
		}
	}
	return got
}

func closedFail(ctx decisionprovider.DecisionContext, provider, code, msg string) *decisionprovider.DecisionEnvelope {
	return &decisionprovider.DecisionEnvelope{
		OK:           false,
		Provider:     provider,
		DecisionTime: ctx.DecisionTime,
		Version:      ctx.Version,
		Lines:        []decisionprovider.DecisionLine{},
		Rejected:     []decisionprovider.RejectedLine{},
		Error:        &decisionprovider.DecisionError{Code: code, Message: msg},
	}
}

func panicText(rec any) string {
	if rec == nil {
		return "panic"
	}
	if err, ok := rec.(error); ok {
		return strings.TrimSpace(err.Error())
	}
	if s, ok := rec.(string); ok {
		return strings.TrimSpace(s)
	}
	return "panic"
}

// DiffEnvelopes is a pure envelope comparison (G.5).
// Portfolio fail does not expand fake "buy nothing" diffs.
func DiffEnvelopes(
	legacy, portfolio *decisionprovider.DecisionEnvelope,
	decisionTime time.Time,
	ranked []selection.Candidate,
) ProviderComparisonReport {
	report := ProviderComparisonReport{
		RecordOnly:         true,
		NotATradePlan:      true,
		NotAProviderSwitch: true,
		DecisionTime:       decisionTime,
		ChainProvider:      decisionprovider.ProviderFixedAmount,
		Legacy:             envelopeRef(legacy),
		Portfolio:          envelopeRef(portfolio),
		Symbols:            emptySymbolDiff(),
		Amounts:            []AmountDiff{},
		Roles:              []RoleDiff{},
		Reasons:            []ReasonDiff{},
	}

	legacyOK := legacy != nil && legacy.OK
	portOK := portfolio != nil && portfolio.OK
	switch {
	case legacyOK && portOK:
		report.Comparable = true
	case legacyOK && !portOK:
		report.IncomparableReason = IncomparablePortfolioFailed
		report.Metrics.SkippedBecause = IncomparablePortfolioFailed
		report.Allocation = BuildAllocationCompare(legacy, portfolio, nil)
		return report
	case !legacyOK && portOK:
		report.IncomparableReason = IncomparableLegacyFailed
		report.Metrics.SkippedBecause = IncomparableLegacyFailed
		report.Allocation = BuildAllocationCompare(legacy, portfolio, nil)
		return report
	default:
		report.IncomparableReason = IncomparableBothFailed
		report.Metrics.SkippedBecause = IncomparableBothFailed
		report.Allocation = BuildAllocationCompare(legacy, portfolio, nil)
		return report
	}

	onlyL, onlyP, common := splitLineSets(legacy, portfolio)
	report.Symbols = SymbolDiff{OnlyLegacy: onlyL, OnlyPortfolio: onlyP, Common: common}

	amounts, equalCount := diffAmounts(legacy, portfolio, onlyL, onlyP, common)
	report.Amounts = amounts
	roles := diffRoles(legacy, portfolio, ranked)
	report.Roles = roles
	reasons, decCount, resCount := diffReasons(legacy, portfolio, common)
	report.Reasons = reasons

	report.Metrics = ComparisonMetrics{
		OnlyLegacyCount:         len(onlyL),
		OnlyPortfolioCount:      len(onlyP),
		CommonCount:             len(common),
		AmountDiffCount:         len(amounts),
		AmountEqualCount:        equalCount,
		RoleDiffCount:           len(roles),
		ReasonDecisionDiffCount: decCount,
		ReasonResearchDiffCount: resCount,
	}
	// H3.1: allocation axis (Budget/Amount/Reserve/RiskCut/Waitlist) — observe only.
	report.Allocation = BuildAllocationCompare(legacy, portfolio, amounts)
	return report
}

func normSymbol(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func lineOrder(env *decisionprovider.DecisionEnvelope) []string {
	if env == nil {
		return nil
	}
	out := make([]string, 0, len(env.Lines))
	seen := map[string]bool{}
	for _, ln := range env.Lines {
		sym := normSymbol(ln.Symbol)
		if sym == "" || seen[sym] {
			continue
		}
		seen[sym] = true
		out = append(out, sym)
	}
	return out
}

func splitLineSets(legacy, portfolio *decisionprovider.DecisionEnvelope) (onlyL, onlyP, common []string) {
	onlyL = []string{}
	onlyP = []string{}
	common = []string{}
	inP := map[string]bool{}
	for _, s := range lineOrder(portfolio) {
		inP[s] = true
	}
	inL := map[string]bool{}
	for _, s := range lineOrder(legacy) {
		inL[s] = true
		if inP[s] {
			common = append(common, s)
		} else {
			onlyL = append(onlyL, s)
		}
	}
	for _, s := range lineOrder(portfolio) {
		if !inL[s] {
			onlyP = append(onlyP, s)
		}
	}
	return onlyL, onlyP, common
}

func findLine(env *decisionprovider.DecisionEnvelope, sym string) (decisionprovider.DecisionLine, bool) {
	if env == nil {
		return decisionprovider.DecisionLine{}, false
	}
	for _, ln := range env.Lines {
		if normSymbol(ln.Symbol) == sym {
			return ln, true
		}
	}
	return decisionprovider.DecisionLine{}, false
}

func amountView(env *decisionprovider.DecisionEnvelope, sym string) AmountView {
	ln, ok := findLine(env, sym)
	if !ok {
		return AmountView{Presence: PresenceAbsent}
	}
	v := ln.TargetAmount
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return AmountView{Presence: PresenceAbsent}
	}
	if v == 0 {
		z := 0.0
		return AmountView{Presence: PresenceZero, Value: &z}
	}
	if v > 0 {
		cp := v
		return AmountView{Presence: PresenceActual, Value: &cp}
	}
	return AmountView{Presence: PresenceAbsent}
}

func amountKind(a, b AmountView) string {
	switch {
	case a.Presence == PresenceActual && b.Presence == PresenceActual:
		return KindBothActual
	case (a.Presence == PresenceZero && b.Presence == PresenceActual) || (a.Presence == PresenceActual && b.Presence == PresenceZero):
		return KindZeroVsActual
	case a.Presence == PresenceZero && b.Presence == PresenceZero:
		return KindBothZero
	case (a.Presence == PresenceAbsent && b.Presence == PresenceZero) || (a.Presence == PresenceZero && b.Presence == PresenceAbsent):
		return KindAbsentVsZero
	case (a.Presence == PresenceAbsent && b.Presence == PresenceActual) || (a.Presence == PresenceActual && b.Presence == PresenceAbsent):
		return KindAbsentVsActual
	default:
		return ""
	}
}

func numericValue(v AmountView) float64 {
	if v.Value == nil {
		return 0
	}
	return *v.Value
}

func diffAmounts(legacy, portfolio *decisionprovider.DecisionEnvelope, onlyL, onlyP, common []string) ([]AmountDiff, int) {
	out := []AmountDiff{}
	equal := 0
	add := func(sym string) {
		lv := amountView(legacy, sym)
		pv := amountView(portfolio, sym)
		kind := amountKind(lv, pv)
		if kind == "" || kind == KindBothZero {
			if kind == KindBothZero {
				equal++
			}
			return
		}
		if kind == KindBothActual {
			delta := numericValue(pv) - numericValue(lv)
			if math.Abs(delta) <= amountEpsilon {
				equal++
				return
			}
			d := delta
			out = append(out, AmountDiff{Symbol: sym, Legacy: lv, Portfolio: pv, Delta: &d, Kind: kind})
			return
		}
		var delta *float64
		if (lv.Presence == PresenceZero || lv.Presence == PresenceActual) &&
			(pv.Presence == PresenceZero || pv.Presence == PresenceActual) {
			d := numericValue(pv) - numericValue(lv)
			delta = &d
		}
		out = append(out, AmountDiff{Symbol: sym, Legacy: lv, Portfolio: pv, Delta: delta, Kind: kind})
	}
	for _, s := range onlyL {
		add(s)
	}
	for _, s := range onlyP {
		add(s)
	}
	for _, s := range common {
		add(s)
	}
	return out, equal
}

func roleOf(env *decisionprovider.DecisionEnvelope, sym string) string {
	if env == nil {
		return RoleAbsent
	}
	for _, r := range env.Rejected {
		if normSymbol(r.Symbol) == sym {
			return RoleRejected
		}
	}
	for _, ln := range env.Lines {
		if normSymbol(ln.Symbol) != sym {
			continue
		}
		if ln.Metadata.InAllocationSet {
			return RoleSelected
		}
		return RoleWaitlist
	}
	return RoleAbsent
}

func diffRoles(legacy, portfolio *decisionprovider.DecisionEnvelope, ranked []selection.Candidate) []RoleDiff {
	seen := map[string]bool{}
	order := []string{}
	push := func(sym string) {
		sym = normSymbol(sym)
		if sym == "" || seen[sym] {
			return
		}
		seen[sym] = true
		order = append(order, sym)
	}
	for _, ln := range lineOrder(legacy) {
		push(ln)
	}
	for _, ln := range lineOrder(portfolio) {
		push(ln)
	}
	if portfolio != nil {
		for _, r := range portfolio.Rejected {
			push(r.Symbol)
		}
	}
	if legacy != nil {
		for _, r := range legacy.Rejected {
			push(r.Symbol)
		}
	}
	for _, c := range ranked {
		push(c.StockCode)
	}

	out := []RoleDiff{}
	for _, sym := range order {
		lr := roleOf(legacy, sym)
		pr := roleOf(portfolio, sym)
		if lr == pr {
			continue
		}
		out = append(out, RoleDiff{
			Symbol:        sym,
			LegacyRole:    lr,
			PortfolioRole: pr,
			Pair:          lr + "_to_" + pr,
		})
	}
	return out
}

func decisionReason(ln decisionprovider.DecisionLine) string {
	s := strings.TrimSpace(ln.Metadata.AllocationReason)
	if s == "" {
		s = strings.TrimSpace(ln.Reason)
	}
	return strings.ToLower(s)
}

func researchReason(ln decisionprovider.DecisionLine, fromPortfolio bool) (string, bool) {
	if fromPortfolio {
		s := strings.TrimSpace(ln.Metadata.CandidateReason)
		if s == "" {
			return "", false
		}
		return s, true
	}
	return strings.TrimSpace(ln.Reason), true
}

func diffReasons(legacy, portfolio *decisionprovider.DecisionEnvelope, common []string) ([]ReasonDiff, int, int) {
	out := []ReasonDiff{}
	dec, res := 0, 0
	for _, sym := range common {
		ll, lok := findLine(legacy, sym)
		pl, pok := findLine(portfolio, sym)
		if !lok || !pok {
			continue
		}
		ld := decisionReason(ll)
		pd := decisionReason(pl)
		if ld != pd {
			out = append(out, ReasonDiff{Symbol: sym, Layer: "decision", LegacyText: ld, PortfolioText: pd})
			dec++
		}
		lr, lHas := researchReason(ll, false)
		pr, pHas := researchReason(pl, true)
		if lHas && pHas && lr != pr {
			out = append(out, ReasonDiff{Symbol: sym, Layer: "research", LegacyText: lr, PortfolioText: pr})
			res++
		}
	}
	return out, dec, res
}
