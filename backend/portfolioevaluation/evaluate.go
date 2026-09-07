package portfolioevaluation

import (
	"fmt"
	"time"

	"go-stock/backend/portfolioreplay"
	"go-stock/backend/portfoliovalidation"
)

// Evaluate runs the Portfolio Historical Evaluation Framework over ReplayCases / DayCases.
// Default Options.Enabled=false → skipped report (read-only / OFF).
// Never creates TradePlan, never calls Execution, never computes PnL/return/Sharpe.
func Evaluate(in Input) *PortfolioHistoricalEvaluationReport {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	opt := in.Options
	rep := &PortfolioHistoricalEvaluationReport{
		SchemaVersion:      SchemaVersion,
		AsOf:               asOf,
		Enabled:            opt.Enabled,
		RecordOnly:         true,
		ReadOnly:           true,
		NotABacktest:       true,
		NotPnL:             true,
		NotReturn:          true,
		NotSharpe:          true,
		NotAutoTune:        true,
		NotATradePlan:      true,
		NotExecution:       true,
		NotProductionWrite: true,
		Days:               []DayEvaluation{},
		Errors:             []EvalError{},
		Notes: []string{
			"portfolio historical evaluation · decision behavior only",
			"not a backtest · no pnl / return / sharpe / auto-tune",
			"composed from portfolioreplay cases + portfoliosim (+ optional decisionshadowv2 / portfolioinsight)",
		},
	}

	if !opt.Enabled {
		rep.Skipped = true
		rep.SkipReason = "enabled=false (default OFF; read-only framework)"
		return rep
	}

	days := make([]portfoliovalidation.DayCase, 0, len(in.Cases)+len(in.Days))
	for _, c := range in.Cases {
		days = append(days, portfoliovalidation.FromReplayCase(c))
	}
	days = append(days, in.Days...)
	rep.CaseCount = len(days)
	if len(days) == 0 {
		rep.Notes = append(rep.Notes, "no_cases")
		return rep
	}

	evaluated := make([]DayEvaluation, 0, len(days))
	for _, day := range days {
		dv := evaluateDay(day, opt)
		if !dv.OK {
			rep.ErrorCount++
			rep.Errors = append(rep.Errors, EvalError{CaseID: dv.CaseID, Reason: dv.Error})
			if opt.IncludeDayRows {
				evaluated = append(evaluated, dv)
			}
			continue
		}
		rep.SuccessCount++
		evaluated = append(evaluated, dv)
	}

	stab, risk, cons, expl := aggregate(filterOK(evaluated))
	rep.DecisionStability = stab
	rep.RiskBehavior = risk
	rep.ConstraintBehavior = cons
	rep.Explainability = expl

	if opt.IncludeDayRows {
		rep.Days = evaluated
	}
	return rep
}

// EvaluateReplayCases is a convenience wrapper.
func EvaluateReplayCases(cases []portfolioreplay.ReplayCase, opt Options) *PortfolioHistoricalEvaluationReport {
	return Evaluate(Input{Cases: cases, Options: opt})
}

// EvaluateDir loads ReplayCase fixtures from dir then evaluates.
func EvaluateDir(dir string, opt Options) (*PortfolioHistoricalEvaluationReport, error) {
	cases, err := portfolioreplay.LoadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("load replay cases: %w", err)
	}
	return EvaluateReplayCases(cases, opt), nil
}

func filterOK(days []DayEvaluation) []DayEvaluation {
	out := make([]DayEvaluation, 0, len(days))
	for _, d := range days {
		if d.OK {
			out = append(out, d)
		}
	}
	return out
}
