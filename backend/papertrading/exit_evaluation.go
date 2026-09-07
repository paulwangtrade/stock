// Exit Evaluation — read-only re-assessment projection (Phase10-D.2.1 + D.2.5 Context + D.2.6 Policy).
//
// Answers: "does this holding need re-evaluation?" — NOT "should we sell now".
// Thresholds come from ExitPolicyProvider → ExitPolicy (legacy ExitEvaluationPolicy kept for compat).
// Consumes Holding Evaluation + read-only Item/Plan context; no Broker/Gateway/Execution writes.

package papertrading

import (
	"sort"
	"strings"
	"time"
)

// ExitEvaluationState — observation labels only (no sell actions).
const (
	ExitEvalStateNormal         = "NORMAL"
	ExitEvalStateWatch          = "WATCH"
	ExitEvalStateReviewRequired = "REVIEW_REQUIRED"
)

// ExitEvaluation reason codes (no SELL / EXIT_NOW / FORCE_CLOSE).
const (
	ExitReasonTimeReview = "TIME_REVIEW"
	ExitReasonLossReview = "LOSS_REVIEW"
	ExitReasonPlanReview = "PLAN_REVIEW"
)

const exitEvaluationDataSourceNote = "Exit Evaluation · read-only re-assessment from Holding Evaluation + Entry/Plan context; not a sell recommendation"

// ExitEvaluationPolicy is the legacy threshold struct (D.2.1). Prefer ExitPolicy / Provider.
// Kept so existing callers and tests do not break.
type ExitEvaluationPolicy struct {
	// TimeReviewDays: holding_days > this → TIME_REVIEW (default 20).
	TimeReviewDays int
	// LossReviewReturn: unrealized_return <= this → LOSS_REVIEW (default -0.05).
	LossReviewReturn float64
	// ReviewRequiredReturn: unrealized_return <= this → REVIEW_REQUIRED (default -0.10).
	ReviewRequiredReturn float64
}

// DefaultExitEvaluationPolicy mirrors DefaultExitPolicy (compat alias).
var DefaultExitEvaluationPolicy = ExitEvaluationPolicy{
	TimeReviewDays:       DefaultExitPolicy.MaxHoldingDays,
	LossReviewReturn:     DefaultExitPolicy.LossWatchThreshold,
	ReviewRequiredReturn: DefaultExitPolicy.LossReviewThreshold,
}

func (p ExitEvaluationPolicy) normalized() ExitEvaluationPolicy {
	return ExitPolicyFromEvaluation(p).ToEvaluationPolicy()
}

// ExitEvaluationLabel is state + reason_codes + summary for stock or lot.
type ExitEvaluationLabel struct {
	State       string   `json:"state"`
	ReasonCodes []string `json:"reason_codes"`
	Summary     string   `json:"summary"`
}

// ExitEvaluationView is the account-level exit evaluation response.
type ExitEvaluationView struct {
	Enabled        bool                     `json:"enabled"`
	AccountID      uint                     `json:"account_id,omitempty"`
	AsOf           time.Time                `json:"as_of"`
	Holdings       []ExitEvaluationStockRow `json:"holdings"`
	Summary        ExitEvaluationSummary    `json:"summary"`
	DataSourceNote string                   `json:"data_source_note"`
	PolicyNote     string                   `json:"policy_note,omitempty"`
	Policy         ExitPolicyRef            `json:"policy"`
}

// ExitEvaluationStockRow is one stock with re-assessment label + lots.
type ExitEvaluationStockRow struct {
	StockCode     string                    `json:"stock_code"`
	StockName     string                    `json:"stock_name"`
	Evaluation    ExitEvaluationLabel       `json:"evaluation"`
	Lots          []ExitEvaluationLotRow    `json:"lots"`
	LatestOutcome *ExitReviewOutcomeSummary `json:"latest_outcome,omitempty"`
	// Explanation is Phase17-C2 read-only hold/risk tags (copied from HoldingEval; no BUY/SELL).
	Explanation *PositionEvaluationExplanation `json:"explanation,omitempty"`
	// HealthScore is Phase17-C3 position quality (copied from HoldingEval; does not drive Exit state).
	HealthScore *HoldingHealthScore `json:"health_score,omitempty"`
	// TSuitability is Phase17.1 做 T suitability (copied from HoldingEval; does not drive Exit state).
	TSuitability *HoldingTSuitability `json:"t_suitability,omitempty"`
}

// ExitEvaluationLotRow preserves attribution identity + optional ExitContext.
type ExitEvaluationLotRow struct {
	FillID           uint                `json:"fill_id"`
	PlanID           uint                `json:"plan_id"`
	PlanItemID       uint                `json:"plan_item_id"`
	HoldingDays      int                 `json:"holding_days"`
	UnrealizedReturn *float64            `json:"unrealized_return"` // ratio; null if missing price
	Evaluation       ExitEvaluationLabel `json:"evaluation"`
	Context          ExitContext         `json:"context"`
}

// ExitEvaluationSummary aggregates counts.
type ExitEvaluationSummary struct {
	StockCount        int            `json:"stock_count"`
	ByEvaluationState map[string]int `json:"by_evaluation_state"`
	ReasonCounts      map[string]int `json:"reason_counts"`
}

// ExitEvaluationBuildOptions filters the DB-backed builder.
type ExitEvaluationBuildOptions struct {
	StockCode string
	AsOf      time.Time
	// Policy is the legacy override (ExitEvaluationPolicy). Prefer ExitPolicy / PolicyProvider.
	Policy *ExitEvaluationPolicy
	// ExitPolicy overrides Provider when set (product struct).
	ExitPolicy *ExitPolicy
	// PolicyProvider resolves thresholds when no explicit policy override is set.
	PolicyProvider ExitPolicyProvider
}

// BuildExitEvaluation loads Holding Evaluation, attaches Item/Plan context, projects exit labels.
// Pipeline: HoldingEval → Explanation → HealthScore → TSuitability → ExitEval
// (TSuitability / Health are additive; Exit state unchanged by them).
func BuildExitEvaluation(opts ExitEvaluationBuildOptions) (*ExitEvaluationView, error) {
	holding, err := BuildHoldingEvaluationObservation(HoldingEvaluationBuildOptions{
		StockCode: opts.StockCode,
		AsOf:      opts.AsOf,
	})
	exitPol := ResolveExitPolicy(opts, ExitPolicyResolveContext{StockCode: opts.StockCode})
	if err != nil {
		out := ProjectExitEvaluationWithExitPolicy(nil, exitPol, nil)
		out.Enabled = IsEnabled()
		return out, err
	}
	asOf := opts.AsOf
	if asOf.IsZero() && holding != nil {
		asOf = holding.AsOf
	}
	hints := LoadExplanationSourceHints(holding)
	EnrichHoldingWithExplanations(holding, hints, ExplanationOptions{AsOf: asOf})
	EnrichHoldingWithHealthScores(holding)
	EnrichHoldingWithTSuitability(holding, nil)
	ctxByFill := LoadExitContextByFillID(holding)
	out := ProjectExitEvaluationWithExitPolicy(holding, exitPol, ctxByFill)
	return out, nil
}

// ProjectExitEvaluation maps Holding Evaluation → ExitEvaluationView without DB context load.
// Compatible with D.2.1 tests; PLAN_REVIEW only appears if ctx is supplied via WithContext.
func ProjectExitEvaluation(holding *HoldingEvalObservationView, policy ExitEvaluationPolicy) *ExitEvaluationView {
	return ProjectExitEvaluationWithContext(holding, policy, nil)
}

// ProjectExitEvaluationWithContext projects using legacy ExitEvaluationPolicy (compat).
func ProjectExitEvaluationWithContext(
	holding *HoldingEvalObservationView,
	policy ExitEvaluationPolicy,
	ctxByFillID map[uint]ExitContext,
) *ExitEvaluationView {
	return ProjectExitEvaluationWithExitPolicy(holding, ExitPolicyFromEvaluation(policy), ctxByFillID)
}

// ProjectExitEvaluationWithExitPolicy projects metrics from ExitPolicy + optional per-fill context.
// Pure over inputs: no DB writes. ctxByFillID may be nil.
func ProjectExitEvaluationWithExitPolicy(
	holding *HoldingEvalObservationView,
	policy ExitPolicy,
	ctxByFillID map[uint]ExitContext,
) *ExitEvaluationView {
	pol := policy.Normalized()
	out := &ExitEvaluationView{
		Enabled:        true,
		Holdings:       []ExitEvaluationStockRow{},
		DataSourceNote: exitEvaluationDataSourceNote,
		PolicyNote:     pol.policyNote(),
		Policy:         pol.Ref(),
		Summary: ExitEvaluationSummary{
			ByEvaluationState: map[string]int{
				ExitEvalStateNormal:         0,
				ExitEvalStateWatch:          0,
				ExitEvalStateReviewRequired: 0,
			},
			ReasonCounts: map[string]int{
				ExitReasonTimeReview: 0,
				ExitReasonLossReview: 0,
				ExitReasonPlanReview: 0,
			},
		},
	}
	if holding == nil {
		return out
	}
	out.Enabled = holding.Enabled
	out.AccountID = holding.AccountID
	out.AsOf = holding.AsOf

	for i := range holding.Holdings {
		h := holding.Holdings[i]
		if h.Explanation == nil {
			ex := BuildPositionEvaluationExplanation(h, ExplanationSourceHint{}, ExplanationOptions{AsOf: holding.AsOf})
			h.Explanation = &ex
		}
		if h.HealthScore == nil && h.Explanation != nil {
			hs := BuildHoldingHealthScore(*h.Explanation, &h)
			h.HealthScore = &hs
		}
		if h.TSuitability == nil {
			gate := HoldingTSuitabilityPositionGate{CanSell: true, AvailableQty: h.TotalVolume}
			ts := BuildHoldingTSuitabilityFromStock(&h, gate, VolatilityUnknown, holding.AsOf)
			h.TSuitability = &ts
		}
		row := evaluateExitStock(h, pol, ctxByFillID)
		out.Holdings = append(out.Holdings, row)
		out.Summary.ByEvaluationState[row.Evaluation.State]++
		for _, code := range row.Evaluation.ReasonCodes {
			out.Summary.ReasonCounts[code]++
		}
	}
	sort.SliceStable(out.Holdings, func(i, j int) bool {
		return out.Holdings[i].StockCode < out.Holdings[j].StockCode
	})
	out.Summary.StockCount = len(out.Holdings)
	return out
}

func evaluateExitStock(h HoldingEvalStockRow, pol ExitPolicy, ctxByFillID map[uint]ExitContext) ExitEvaluationStockRow {
	row := ExitEvaluationStockRow{
		StockCode: h.StockCode,
		StockName: h.StockName,
		Lots:      []ExitEvaluationLotRow{},
		Evaluation: ExitEvaluationLabel{
			State:       ExitEvalStateNormal,
			ReasonCodes: []string{},
		},
		Explanation:  h.Explanation,
		HealthScore:  h.HealthScore,
		TSuitability: h.TSuitability,
	}
	reasonSet := map[string]bool{}
	state := ExitEvalStateNormal

	for _, lot := range h.Lots {
		if lot.FillID == 0 {
			continue // never forge
		}
		ctx := ExitContext{Plan: ExitPlanContext{PlanID: lot.PlanID}}
		if ctxByFillID != nil {
			if c, ok := ctxByFillID[lot.FillID]; ok {
				ctx = c
			}
		}
		if ctx.Plan.PlanID == 0 {
			ctx.Plan.PlanID = lot.PlanID
		}
		lotLabel := evaluateExitLot(lot.HoldingDays, lot.ReturnRate, pol, ctx)
		row.Lots = append(row.Lots, ExitEvaluationLotRow{
			FillID:           lot.FillID,
			PlanID:           lot.PlanID,
			PlanItemID:       lot.PlanItemID,
			HoldingDays:      lot.HoldingDays,
			UnrealizedReturn: lot.ReturnRate,
			Evaluation:       lotLabel,
			Context:          ctx,
		})
		state = worseExitState(state, lotLabel.State)
		for _, c := range lotLabel.ReasonCodes {
			reasonSet[c] = true
		}
	}

	// Stock-level rules (same policy) using aggregated holding metrics.
	stockLabel := evaluateExitMetrics(h.HoldingDays, h.UnrealizedReturn, pol)
	state = worseExitState(state, stockLabel.State)
	for _, c := range stockLabel.ReasonCodes {
		reasonSet[c] = true
	}

	row.Evaluation.State = state
	row.Evaluation.ReasonCodes = sortedReasonCodes(reasonSet)
	row.Evaluation.Summary = BuildExitReviewSummary(h.HoldingDays, h.UnrealizedReturn, row.Evaluation.State, row.Evaluation.ReasonCodes)
	return row
}

func evaluateExitLot(holdingDays int, unrealizedReturn *float64, pol ExitPolicy, ctx ExitContext) ExitEvaluationLabel {
	label := evaluateExitMetrics(holdingDays, unrealizedReturn, pol)
	label = applyPlanReview(label, ctx)
	label.Summary = BuildExitReviewSummary(holdingDays, unrealizedReturn, label.State, label.ReasonCodes)
	return label
}

func evaluateExitMetrics(holdingDays int, unrealizedReturn *float64, pol ExitPolicy) ExitEvaluationLabel {
	pol = pol.Normalized()
	label := ExitEvaluationLabel{
		State:       ExitEvalStateNormal,
		ReasonCodes: []string{},
	}
	reasons := map[string]bool{}

	if holdingDays > pol.MaxHoldingDays {
		reasons[ExitReasonTimeReview] = true
		label.State = worseExitState(label.State, ExitEvalStateWatch)
	}

	// Missing price / nil return → do not invent LOSS_REVIEW.
	if unrealizedReturn != nil {
		r := *unrealizedReturn
		if r <= pol.LossReviewThreshold {
			reasons[ExitReasonLossReview] = true
			label.State = worseExitState(label.State, ExitEvalStateReviewRequired)
		} else if r <= pol.LossWatchThreshold {
			reasons[ExitReasonLossReview] = true
			label.State = worseExitState(label.State, ExitEvalStateWatch)
		}
	}

	label.ReasonCodes = sortedReasonCodes(reasons)
	return label
}

// applyPlanReview adds PLAN_REVIEW when plan lifecycle is abnormal (re-check only).
func applyPlanReview(label ExitEvaluationLabel, ctx ExitContext) ExitEvaluationLabel {
	if !IsPlanLifecycleAbnormal(ctx.Plan.PlanStatus) {
		return label
	}
	reasons := map[string]bool{}
	for _, c := range label.ReasonCodes {
		reasons[c] = true
	}
	reasons[ExitReasonPlanReview] = true
	label.State = worseExitState(label.State, ExitEvalStateWatch)
	label.ReasonCodes = sortedReasonCodes(reasons)
	return label
}

func worseExitState(a, b string) string {
	rank := func(s string) int {
		switch s {
		case ExitEvalStateReviewRequired:
			return 3
		case ExitEvalStateWatch:
			return 2
		case ExitEvalStateNormal:
			return 1
		default:
			return 0
		}
	}
	if rank(b) > rank(a) {
		return b
	}
	if a == "" {
		return ExitEvalStateNormal
	}
	return a
}

func sortedReasonCodes(set map[string]bool) []string {
	if len(set) == 0 {
		return []string{}
	}
	order := []string{ExitReasonTimeReview, ExitReasonLossReview, ExitReasonPlanReview}
	out := make([]string, 0, len(set))
	for _, c := range order {
		if set[c] {
			out = append(out, c)
		}
	}
	// any unexpected codes last (stable)
	extra := make([]string, 0)
	for c := range set {
		known := false
		for _, k := range order {
			if c == k {
				known = true
				break
			}
		}
		if !known {
			extra = append(extra, c)
		}
	}
	sort.Strings(extra)
	out = append(out, extra...)
	return out
}

// ReasonCodeLabelZH is a display hint for Observation UI (not an action).
func ReasonCodeLabelZH(code string) string {
	switch strings.TrimSpace(code) {
	case ExitReasonTimeReview:
		return "持有时间关注"
	case ExitReasonLossReview:
		return "亏损关注"
	case ExitReasonPlanReview:
		return "计划变化关注"
	default:
		return code
	}
}
