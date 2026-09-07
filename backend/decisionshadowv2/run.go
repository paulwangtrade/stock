package decisionshadowv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfoliorisk"
	"go-stock/backend/selection"
)

// Runtime is the H3.2 Portfolio Decision Shadow V2 entry. Default Enabled=false.
type Runtime struct {
	Enabled bool
}

// NewRuntime returns a V2 runtime. Pass enabled=true only for explicit observation.
func NewRuntime(enabled bool) *Runtime {
	return &Runtime{Enabled: enabled}
}

// Observe runs the full V2 simulation when Enabled (or in.Enabled). Never writes Draft/TP/Exec.
func (r *Runtime) Observe(in Input) *Report {
	enabled := DefaultEnabled
	if r != nil && r.Enabled {
		enabled = true
	}
	if in.Enabled {
		enabled = true
	}
	return Run(in, enabled)
}

// Run executes Portfolio Decision Shadow V2. enabled=false → skipped report with empty projections.
func Run(in Input, enabled bool) *Report {
	rep := baseReport(in, enabled)
	if !enabled {
		rep.Skipped = true
		rep.SkipReason = "enabled=false (default OFF; not a provider switch)"
		rep.LegacyPlanProjection = emptyProjection("fixed_amount")
		rep.PortfolioPlanProjection = emptyProjection("portfolio_allocation")
		rep.Difference = DecisionShadowDifference{
			SelectionDiff: SelectionDiff{
				OnlyLegacy: []string{}, OnlyPortfolio: []string{}, Both: []string{},
			},
			AmountDiff: []AmountDiffRow{},
			WeightDiff: []WeightDiffRow{},
			SectorDiff: SectorDiff{Available: false, UnavailableReason: "skipped"},
		}
		return rep
	}

	applyDefaultOptions(&in)

	ranked := rankedCandidates(in.CandidatePool)
	rep.InputsFingerprint = fingerprint(in, ranked)

	// --- Legacy side (no Objective / Tighten) ---
	legacy := projectLegacy(in, ranked)
	rep.LegacyPlanProjection = legacy

	// --- Portfolio side: Objective Resolve → Risk Tighten → Alloc ---
	ceiling := in.RiskCeilingTemplate
	baseCS, baseResolved, objSummary := CompileAndResolve(in.Objective, ceiling)
	rep.ChainTrace.ObjectiveCompileSummary = objSummary
	rep.ChainTrace.ResolvedBaseSummary = fmt.Sprintf(
		"max_new=%d reserve=%.4f gross=%.4f single=%.4f",
		baseResolved.MaxNewNames, baseResolved.ReserveCashRatio,
		baseResolved.MaxGrossExposurePct, baseResolved.MaxSingleWeight,
	)

	riskSnap := in.PrebuiltRisk
	if riskSnap == nil {
		ledger := in.Ledger
		if ledger == nil && in.Snapshot != nil {
			ledger = in.Snapshot.Ledger()
		}
		riskSnap = portfoliorisk.Build(portfoliorisk.BuildInput{
			Snapshot:    ledger,
			TradeDate:   in.TradeDate,
			Risk:        in.RiskView,
			Constraints: &baseCS,
		})
	}
	rep.ChainTrace.RiskSnapshotAvailable = riskSnap != nil
	if riskSnap != nil {
		rep.ChainTrace.RiskFound = riskSnap.Found
	}

	sug := portfoliorisk.SuggestTighten(riskSnap, baseCS)
	effectiveCS := portfoliorisk.ApplyTightenOnly(baseCS, sug.ConstraintSet)
	effectiveResolved := effectiveCS.Resolve()
	rep.ChainTrace.TightenApplied = sug.HasPatches
	for _, n := range sug.Notes {
		rep.ChainTrace.TightenNotes = append(rep.ChainTrace.TightenNotes,
			fmt.Sprintf("%s:%s", n.Field, n.Reason))
	}
	rep.ChainTrace.ResolvedEffectiveSummary = fmt.Sprintf(
		"max_new=%d reserve=%.4f gross=%.4f single=%.4f block_new=%v",
		effectiveResolved.MaxNewNames, effectiveResolved.ReserveCashRatio,
		effectiveResolved.MaxGrossExposurePct, effectiveResolved.MaxSingleWeight,
		effectiveResolved.RiskBlockNewEntries,
	)

	// Risk impact: allocate once on base (pre-tighten) and once on effective.
	preProj, _, _, _ := projectPortfolio(in, ranked, baseCS, baseResolved, in.AllocationOptions)
	portProj, sel, eng, budgetSummary := projectPortfolio(in, ranked, effectiveCS, effectiveResolved, in.AllocationOptions)
	rep.PortfolioPlanProjection = portProj
	rep.ChainTrace.AllocationBudgetSummary = budgetSummary
	rep.ChainTrace.AllocationMethod = allocMethod(eng)
	if sel != nil {
		rep.ChainTrace.SelectionSummary = fmt.Sprintf(
			"selected=%d waitlist=%d rejected=%d limit=%d",
			len(sel.Selected), len(sel.Waitlist), len(sel.Rejected), portfolioLimit(in, effectiveResolved),
		)
	}

	allocReasons := uniqueReasons(portProj)
	rep.ChainTrace.AllocationChangeReasons = allocReasons

	riskImpact := RiskImpactDiff{
		TightenApplied:        sug.HasPatches,
		NotionalBeforeTighten: preProj.Totals.BuyNotionalSum,
		NotionalAfterTighten:  portProj.Totals.BuyNotionalSum,
		NotionalDelta:         portProj.Totals.BuyNotionalSum - preProj.Totals.BuyNotionalSum,
		NameCountBefore:       preProj.Totals.NameCount,
		NameCountAfter:        portProj.Totals.NameCount,
		Explain:               explainRiskImpact(sug, preProj, portProj, allocReasons),
	}
	for _, n := range sug.Notes {
		riskImpact.Notes = append(riskImpact.Notes, fmt.Sprintf("%s:%s %s", n.Field, n.Reason, n.Detail))
	}

	if !portProj.OK {
		rep.Failures = append(rep.Failures, Failure{
			Side: "portfolio", Code: "PORTFOLIO_FAIL", Message: portProj.Failure,
		})
	}

	secOK := sectorAvailable(in, legacy, portProj)
	rep.Difference = buildDifference(legacy, portProj, riskImpact, secOK)
	rep.Notes = append(rep.Notes,
		"record_only simulation; not Draft/TradePlan/Execution",
		"legacy path: fixed_amount projection only",
		"portfolio path: Objective Resolve → SuggestTighten → AllocationEngine",
	)
	return rep
}

func baseReport(in Input, enabled bool) *Report {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	return &Report{
		SchemaVersion:     SchemaVersion,
		Enabled:           enabled,
		AsOf:              asOf,
		TradeDate:         in.TradeDate,
		AccountID:         in.AccountID,
		RecordOnly:        true,
		NotADraft:         true,
		NotATradePlan:     true,
		NotExecution:      true,
		NotProviderSwitch: true,
		NotBuyChainWrite:  true,
		Failures:          []Failure{},
		Notes:             []string{},
	}
}

func emptyProjection(id string) PlanProjection {
	return PlanProjection{
		ProviderIdentity: id,
		Selected:         []SelectedName{},
		Lines:            []PlanLine{},
		OK:               false,
		Failure:          "skipped",
	}
}

func uniqueReasons(p PlanProjection) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, ln := range p.Lines {
		r := strings.TrimSpace(ln.AllocationReason)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func explainRiskImpact(sug portfoliorisk.TightenSuggestion, before, after PlanProjection, reasons []string) []string {
	out := []string{}
	if !sug.HasPatches {
		out = append(out, "no_tighten_patches; risk_impact_delta_from_constraints_only=false")
	} else {
		out = append(out, fmt.Sprintf(
			"tighten_changed_notional: before=%.0f after=%.0f delta=%.0f",
			before.Totals.BuyNotionalSum, after.Totals.BuyNotionalSum,
			after.Totals.BuyNotionalSum-before.Totals.BuyNotionalSum,
		))
	}
	if before.Totals.BudgetBinding != after.Totals.BudgetBinding {
		out = append(out, fmt.Sprintf(
			"budget_binding: %s → %s", before.Totals.BudgetBinding, after.Totals.BudgetBinding,
		))
	}
	if len(reasons) > 0 {
		out = append(out, "allocation_reasons="+strings.Join(reasons, ","))
	}
	return out
}

func fingerprint(in Input, ranked []selection.Candidate) string {
	type fp struct {
		TradeDate string
		AccountID string
		Codes     []string
		LegacyAmt float64
		LegLimit  int
		PortLimit int
		Obj       PortfolioObjective
		CeilingR  *float64
		CeilingS  *float64
	}
	codes := make([]string, 0, len(ranked))
	for _, c := range ranked {
		codes = append(codes, normalizeSym(c.StockCode))
	}
	payload, _ := json.Marshal(fp{
		TradeDate: in.TradeDate,
		AccountID: in.AccountID,
		Codes:     codes,
		LegacyAmt: legacyAmount(in),
		LegLimit:  legacyLimit(in),
		PortLimit: in.SelectionLimitPortfolio,
		Obj:       in.Objective,
		CeilingR:  in.RiskCeilingTemplate.Risk.MaxGrossExposurePct,
		CeilingS:  in.RiskCeilingTemplate.Risk.MaxSingleNamePct,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:8])
}

// applyDefaultOptions: zero Options → Phase=shadow_only + ProjectPostBuyBook=true.
// Explicit ProjectPostBuyBook=false with Phase set is honored.
func applyDefaultOptions(in *Input) {
	if in.Options.Phase == "" && !in.Options.ProjectPostBuyBook && !in.Options.IncludeWaitlistInDiff {
		in.Options.Phase = "shadow_only"
		in.Options.ProjectPostBuyBook = true
		return
	}
	if in.Options.Phase == "" {
		in.Options.Phase = "shadow_only"
	}
}
