package sellallocation

import (
	"strings"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/holdingdecision/rules"
)

// ProjectIntentFromRules maps H1.2 HoldingDecision → SellIntent.
// HOLD → ok=false (no sell intent).
func ProjectIntentFromRules(d rules.HoldingDecision, currentWeight float64, opt Options) (SellIntent, bool) {
	action := strings.ToUpper(strings.TrimSpace(d.FinalAction))
	if action == "" {
		action = strings.ToUpper(strings.TrimSpace(d.Action))
	}
	return projectAction(d.Symbol, action, d.Explanation, d.ReasonCodes, currentWeight, nil, nil, opt)
}

// ProjectIntentFromAction maps H1.1 ActionDecision → SellIntent.
func ProjectIntentFromAction(d holdingdecision.ActionDecision, currentWeight float64, opt Options) (SellIntent, bool) {
	var frac *float64
	var targetAfter *float64
	if d.Reduce != nil {
		frac = d.Reduce.Fraction
		targetAfter = d.Reduce.TargetWeightAfter
	}
	return projectAction(d.Symbol, d.Action, d.Summary, d.ReasonCodes, currentWeight, frac, targetAfter, opt)
}

func projectAction(
	symbol, action, reason string,
	codes []string,
	currentWeight float64,
	frac, targetAfter *float64,
	opt Options,
) (SellIntent, bool) {
	action = strings.ToUpper(strings.TrimSpace(action))
	if action != ActionReduce && action != ActionExit {
		return SellIntent{}, false
	}
	if opt.DefaultReduceRatio <= 0 || opt.DefaultReduceRatio >= 1 {
		opt.DefaultReduceRatio = DefaultReduceFraction
	}
	intent := SellIntent{
		Symbol:      strings.TrimSpace(symbol),
		Action:      action,
		Reason:      strings.TrimSpace(reason),
		ReasonCodes: append([]string{}, codes...),
		RecordOnly:  true,
		NotAnOrder:  true,
	}
	if intent.Reason == "" && len(codes) > 0 {
		intent.Reason = codes[0]
	}
	if intent.Reason == "" {
		intent.Reason = ReasonFromDecision
	}
	if action == ActionExit {
		intent.TargetPositionWeight = 0
		intent.TargetReduceRatio = 1
		return intent, true
	}
	ratio := opt.DefaultReduceRatio
	if frac != nil && *frac > 0 && *frac <= 1 {
		ratio = *frac
	}
	intent.TargetReduceRatio = ratio
	if targetAfter != nil && *targetAfter >= 0 {
		intent.TargetPositionWeight = *targetAfter
	} else if currentWeight > 0 {
		intent.TargetPositionWeight = currentWeight * (1 - ratio)
		if intent.TargetPositionWeight < 0 {
			intent.TargetPositionWeight = 0
		}
	}
	return intent, true
}

// LookupTargetWeight returns target weight for symbol from TargetPortfolio (0,false if absent).
func LookupTargetWeight(tgt *TargetPortfolio, symbol string) (float64, bool) {
	if tgt == nil {
		return 0, false
	}
	sym := normSym(symbol)
	for _, n := range tgt.Names {
		if normSym(n.Symbol) == sym {
			return n.TargetWeight, true
		}
	}
	return 0, false
}

func normSym(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
