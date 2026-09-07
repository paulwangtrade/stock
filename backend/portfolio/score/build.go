package score

import (
	"math"
	"strings"
	"time"
)

// Build projects InvestmentScoreView. Nil components stay nil — never invents neutrals.
func Build(in Inputs) *InvestmentScoreView {
	asOf := in.AsOf
	if asOf.IsZero() {
		asOf = time.Now()
	}
	code := strings.TrimSpace(in.StockCode)
	out := &InvestmentScoreView{
		StockCode:      code,
		TradeDate:      strings.TrimSpace(in.TradeDate),
		AsOf:           asOf,
		MissingFactors: []string{},
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}

	cand := in.Candidate.Present
	hold := in.Holding.Present
	if !cand && !hold {
		out.SubjectType = ""
		out.Quality = QualityNotFound
		out.Found = false
		out.MissingFactors = []string{"strategy_score", "risk_score", "momentum_score", "portfolio_fit_score"}
		return out
	}
	out.Found = true

	switch {
	case cand && hold:
		out.SubjectType = SubjectBoth
	case cand:
		out.SubjectType = SubjectCandidate
	default:
		out.SubjectType = SubjectHolding
	}
	out.StockName = firstName(in.Candidate.StockName, in.Holding.StockName)

	// --- component mapping (real data only) ---
	out.StrategyScore = mapStrategy(in)
	out.MomentumScore = mapMomentum(in)
	out.RiskScore = mapRisk(in)
	out.PortfolioFitScore = mapFit(in)

	if out.StrategyScore == nil {
		out.MissingFactors = append(out.MissingFactors, "strategy_score")
	}
	if out.RiskScore == nil {
		out.MissingFactors = append(out.MissingFactors, "risk_score")
	}
	if out.MomentumScore == nil {
		out.MissingFactors = append(out.MissingFactors, "momentum_score")
	}
	if out.PortfolioFitScore == nil {
		out.MissingFactors = append(out.MissingFactors, "portfolio_fit_score")
	}

	out.TotalScore = composeTotal(out.SubjectType, out.StrategyScore, out.RiskScore, out.MomentumScore, out.PortfolioFitScore)
	if len(out.MissingFactors) == 0 && out.TotalScore != nil {
		out.Quality = QualityOK
	} else if out.TotalScore != nil {
		out.Quality = QualityDegraded
	} else {
		out.Quality = QualityDegraded
		out.MissingFactors = appendUnique(out.MissingFactors, "total_score")
	}
	return out
}

func mapStrategy(in Inputs) *float64 {
	// Prefer candidate composite Score (authoritative opportunity quality).
	if in.Candidate.Present {
		return f64(clamp100(in.Candidate.Score * 100))
	}
	// Holding-only: only when strategy_status is a known observation (not UNKNOWN).
	switch strings.ToUpper(strings.TrimSpace(in.Holding.StrategyStatus)) {
	case "ACTIVE_SIGNAL":
		return f64(75)
	case "SIGNAL_PENDING":
		return f64(55)
	case "NO_SIGNAL":
		return f64(35)
	default:
		return nil
	}
}

func mapMomentum(in Inputs) *float64 {
	if in.Candidate.Present && in.Candidate.HasSignal {
		return f64(clamp100(in.Candidate.SignalScore * 100))
	}
	if in.Holding.Present && in.Holding.UnrealizedReturn != nil {
		return f64(returnToMomentum(*in.Holding.UnrealizedReturn))
	}
	return nil
}

func mapRisk(in Inputs) *float64 {
	// Holding risk_level is a real observation.
	if in.Holding.Present {
		base, ok := riskLevelScore(in.Holding.RiskLevel)
		if !ok {
			// fall through to candidate plan risk if any
		} else {
			if in.Holding.UnrealizedReturn != nil && *in.Holding.UnrealizedReturn < -0.10 {
				base = math.Min(base, 25)
			} else if in.Holding.UnrealizedReturn != nil && *in.Holding.UnrealizedReturn < -0.05 {
				base = math.Min(base, 45)
			}
			return f64(base)
		}
	}
	// Candidate: only when TradePlan item carries a real RiskCode (rejected / flagged).
	if in.Candidate.Present && strings.TrimSpace(in.Candidate.TradePlanRiskCode) != "" {
		return f64(40)
	}
	return nil
}

func mapFit(in Inputs) *float64 {
	if in.Holding.Present {
		w := in.Holding.Weight
		if w < 0 {
			w = 0
		}
		score := 100.0
		// Penalize concentration using observed weight bands (aligned with readiness spirit).
		switch {
		case w >= 0.15:
			score = 35
		case w >= 0.10:
			score = 55
		case w >= 0.05:
			score = 75
		default:
			score = 90
		}
		if strings.EqualFold(in.Holding.PositionStatus, "NEED_REVIEW") {
			score = math.Min(score, 50)
		}
		return f64(score)
	}
	// Candidate-only: no invented fit without portfolio weight / plan notional.
	return nil
}

func composeTotal(subject string, strategy, risk, momentum, fit *float64) *float64 {
	type part struct {
		w float64
		v *float64
	}
	var parts []part
	if subject == SubjectHolding {
		parts = []part{{0.15, strategy}, {0.35, risk}, {0.20, momentum}, {0.30, fit}}
	} else {
		// CANDIDATE and BOTH: opportunity-leaning weights; BOTH still prefers candidate weights
		// while components themselves already prefer holding for risk/fit.
		parts = []part{{0.40, strategy}, {0.20, risk}, {0.20, momentum}, {0.20, fit}}
	}
	var sumW, sum float64
	for _, p := range parts {
		if p.v == nil {
			continue
		}
		sumW += p.w
		sum += p.w * (*p.v)
	}
	if sumW <= 0 {
		return nil
	}
	return f64(clamp100(sum / sumW))
}

func riskLevelScore(level string) (float64, bool) {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "LOW":
		return 80, true
	case "MEDIUM":
		return 55, true
	case "HIGH":
		return 30, true
	default:
		return 0, false
	}
}

// returnToMomentum maps unrealized return to 0–100 (deep loss → low).
func returnToMomentum(ret float64) float64 {
	// -20% → 0, 0% → 50, +20% → 100 (clamped)
	v := 50 + ret*50/0.20
	return clamp100(v)
}

func clamp100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func f64(v float64) *float64 { return &v }

func firstName(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

func appendUnique(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
