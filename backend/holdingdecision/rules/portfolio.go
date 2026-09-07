package rules

// Portfolio tighten observation: post-SuggestTighten constraints trigger observation
// when the name would violate the tightened single-name (or related) cap.
// Does not call SuggestTighten; does not write ConstraintSet / TradePlan.

type portfolioTightenRule struct{}

func (portfolioTightenRule) ID() string     { return RulePortfolioTighten }
func (portfolioTightenRule) Family() string { return FamilyPortfolio }

func (portfolioTightenRule) Eval(ctx RuleContext) []RuleHit {
	pol := ctx.Policy
	if !pol.EnablePortfolioTighten {
		return nil
	}
	t := ctx.Tightened
	if t == nil || !t.Available {
		return nil
	}
	if t.MaxSingleWeight == nil || *t.MaxSingleWeight <= 0 {
		return nil
	}
	if !weightAboveCap(ctx.Weight, *t.MaxSingleWeight) {
		return nil
	}
	return []RuleHit{hit(RulePortfolioTighten, FamilyPortfolio, ActionReduce, SeverityWatch, ReasonPortfolioTighten, map[string]any{
		"weight":            ctx.Weight,
		"max_single_weight": *t.MaxSingleWeight,
		"source":            t.Source,
		"note":              "tightened portfolio constraint → observe REDUCE",
	})}
}
