package exitwatch

import (
	"math"
	"strings"
	"time"

	"go-stock/backend/papertrading"
	"go-stock/backend/portfolio/positionstate"
)

// EvaluatePaper projects Exit Evaluation rows plus PositionState.
// It does not re-derive thresholds and does not create a sell draft.
func EvaluatePaper(view *papertrading.ExitEvaluationView, states []positionstate.PositionStateView, opt Options) []Item {
	if view == nil {
		return nil
	}
	opt = opt.normalized()
	pol := policyFromRef(view.Policy)
	asOf := opt.AsOf
	if !view.AsOf.IsZero() && opt.AsOf.IsZero() {
		asOf = view.AsOf
	}
	bar := opt.Bar
	if bar == "" && !asOf.IsZero() {
		bar = asOf.In(time.Local).Format("2006-01-02")
	}
	byCode := map[string]positionstate.PositionStateView{}
	for _, st := range states {
		key := normCode(st.Symbol)
		if key == "" {
			continue
		}
		byCode[key] = st
	}
	out := make([]Item, 0, len(view.Holdings))
	for _, row := range view.Holdings {
		code := normCode(row.StockCode)
		st, hasState := byCode[code]
		priceKnown, costKnown, stale := paperCompleteness(row)
		posID := code
		facts := Facts{
			Source:            SourcePaperSim,
			PositionID:        posID,
			StockCode:         strings.TrimSpace(row.StockCode),
			StockName:         row.StockName,
			CostKnown:         costKnown,
			PriceKnown:        priceKnown,
			PriceStale:        stale,
			UseExitEvaluation: true,
			ExitState:         row.Evaluation.State,
			ExitReasonCodes:   append([]string(nil), row.Evaluation.ReasonCodes...),
			SignalReview:      signalExpired(row),
			T1Locked:          hasState && positionT1Locked(st),
			ManualDraftRef:    draftRef(opt.ManualDraftRefs, code),
			Bar:               bar,
			AsOf:              asOf,
			Policy:            pol,
		}
		if code == "" {
			facts.PositionID = ""
			facts.StockCode = ""
		}
		out = append(out, Project(facts))
	}
	return sortItems(out)
}

func policyFromRef(ref papertrading.ExitPolicyRef) Policy {
	return Policy{
		ID:                  ref.PolicyID,
		Version:             ref.Version,
		MaxHoldingDays:      ref.MaxHoldingDays,
		LossWatchThreshold:  ref.LossWatchThreshold,
		LossReviewThreshold: ref.LossReviewThreshold,
	}.normalized()
}

func paperCompleteness(row papertrading.ExitEvaluationStockRow) (priceKnown, costKnown, stale bool) {
	if row.Explanation != nil {
		if positiveFinite(row.Explanation.CurrentPrice) {
			priceKnown = true
		}
		if positiveFinite(row.Explanation.CostPrice) {
			costKnown = true
		}
		if row.Explanation.Freshness.PriceStatus == papertrading.EvalFreshnessStale {
			stale = true
		}
	}
	if row.HealthScore != nil {
		for _, tag := range row.HealthScore.RiskFactors {
			if tag == papertrading.ExplainTagPriceStale {
				stale = true
			}
		}
	}
	for _, lot := range row.Lots {
		if finiteReturn(lot.UnrealizedReturn) != nil {
			priceKnown = true
			costKnown = true
		}
	}
	switch row.Observation.Class {
	case papertrading.ExitObserveClassHold, papertrading.ExitObserveClassReduce, papertrading.ExitObserveClassFlatten:
		priceKnown = true
		costKnown = true
	}
	return priceKnown, costKnown, stale
}

func signalExpired(row papertrading.ExitEvaluationStockRow) bool {
	if row.Explanation == nil {
		return false
	}
	for _, tag := range row.Explanation.RiskHints {
		if tag == papertrading.ExplainTagSignalExpired {
			return true
		}
	}
	return false
}

func positionT1Locked(st positionstate.PositionStateView) bool {
	switch st.State {
	case positionstate.S1NewLocked:
		return true
	}
	switch st.RiskTag {
	case positionstate.RiskTagNewLocked, positionstate.RiskTagPartialLock:
		return true
	}
	return st.LockedQty > 0
}

func positiveFinite(v *float64) bool {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return false
	}
	return *v > 0
}

func draftRef(refs map[string]string, code string) string {
	if len(refs) == 0 {
		return ""
	}
	if s := strings.TrimSpace(refs[code]); s != "" {
		return s
	}
	return strings.TrimSpace(refs[normCode(code)])
}

func normCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
