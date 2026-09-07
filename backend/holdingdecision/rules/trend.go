package rules

// Trend rules consume only explicit TrendFact.
// Missing TrendFact or Available=false → no judgment.

type trendBreakRule struct{}

func (trendBreakRule) ID() string     { return RuleTrendBreak }
func (trendBreakRule) Family() string { return FamilyTrend }

func (trendBreakRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnableTrend {
		return nil
	}
	if ctx.Trend == nil || !ctx.Trend.Available {
		return nil
	}
	broken := ctx.Trend.ThesisBroken || ctx.Trend.BelowMA
	if !broken {
		return nil
	}
	return []RuleHit{hit(RuleTrendBreak, FamilyTrend, ActionReduce, SeverityHigh, ReasonTrendBreak, map[string]any{
		"thesis_broken": ctx.Trend.ThesisBroken,
		"below_ma":      ctx.Trend.BelowMA,
		"source":        ctx.Trend.Source,
		"note":          "explicit TrendFact break → observe REDUCE",
	})}
}
