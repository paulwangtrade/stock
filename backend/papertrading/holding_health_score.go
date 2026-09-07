// Holding Health Score — Phase17 C3 read-only position quality score.
//
// Consumes PositionEvaluationExplanation (+ HoldingEval profit metrics). Does NOT
// recompute signal/trend/freshness tags. Does NOT emit BUY/SELL or change ExitEval state.

package papertrading

import (
	"strings"
	"time"
)

// Health grades (observation labels — not sell recommendations).
const (
	HealthGradeA = "A" // 健康持有
	HealthGradeB = "B" // 正常观察
	HealthGradeC = "C" // 重点关注
	HealthGradeD = "D" // 风险较高
)

// Extra profit factors beyond Explanation hold tags (from HoldingEval metrics only).
const (
	HealthFactorProfit          = "PROFIT"
	HealthFactorProfitExpanding = "PROFIT_EXPANDING"
)

const (
	healthScoreBase              = 50
	healthProfitBonus            = 10
	healthProfitExpandingBonus   = 10
	healthSignalActiveBonus      = 15
	healthTrendSupportBonus      = 15
	healthProfitProtectionBonus  = 10
	healthSignalExpiredPenalty   = 15
	healthLossControlPenalty     = 15
	healthPriceStalePenalty      = 10
	healthNoSourceTracePenalty   = 10
	healthProfitExpandingMinRet  = 0.05 // unrealized_return >= 5% → 盈利扩大
	holdingHealthScoreNote       = "Holding Health Score · position quality only; not a sell signal"
)

// HoldingHealthScore is a 0–100 quality score derived from Explanation tags + PnL state.
type HoldingHealthScore struct {
	StockCode          string                         `json:"stock_code"`
	EvaluationTime     time.Time                      `json:"evaluation_time"`
	Score              int                            `json:"score"`
	Grade              string                         `json:"grade"`
	GradeLabel         string                         `json:"grade_label"`
	SupportingFactors  []string                       `json:"supporting_factors"`
	RiskFactors        []string                       `json:"risk_factors"`
	Explanation        *PositionEvaluationExplanation `json:"explanation,omitempty"`
	DataSourceNote     string                         `json:"data_source_note"`
}

// BuildHoldingHealthScore scores from an existing Explanation (required) and optional stock metrics.
// Pure: no DB, no trade actions. Does not mutate ExitEval thresholds.
func BuildHoldingHealthScore(
	ex PositionEvaluationExplanation,
	stock *HoldingEvalStockRow,
) HoldingHealthScore {
	asOf := ex.EvaluationTime
	if asOf.IsZero() {
		asOf = time.Now()
	}
	code := strings.TrimSpace(ex.StockCode)
	if stock != nil && code == "" {
		code = strings.TrimSpace(stock.StockCode)
	}

	factors := make([]string, 0, 8)
	risks := make([]string, 0, 8)
	score := healthScoreBase

	// --- Profit from HoldingEval (do not re-derive Explanation tags) ---
	profitState := ""
	var ret *float64
	if stock != nil {
		profitState = stock.ProfitState
		ret = stock.UnrealizedReturn
		if profitState == "" {
			profitState = ClassifyProfitState(ret)
		}
	} else if ex.PnL != nil {
		if *ex.PnL > 0 {
			profitState = ProfitStateProfit
		} else if *ex.PnL < 0 {
			profitState = ProfitStateLoss
		}
	}
	if profitState == ProfitStateProfit {
		score += healthProfitBonus
		factors = append(factors, HealthFactorProfit)
		if ret != nil && *ret >= healthProfitExpandingMinRet {
			score += healthProfitExpandingBonus
			factors = append(factors, HealthFactorProfitExpanding)
		}
	}

	// --- Hold tags from Explanation ---
	for _, tag := range ex.HoldReasons {
		switch tag {
		case ExplainTagSignalActive:
			score += healthSignalActiveBonus
			factors = append(factors, ExplainTagSignalActive)
		case ExplainTagTrendSupport:
			score += healthTrendSupportBonus
			factors = append(factors, ExplainTagTrendSupport)
		case ExplainTagProfitProtection:
			score += healthProfitProtectionBonus
			factors = append(factors, ExplainTagProfitProtection)
		}
	}

	// --- Risk tags from Explanation ---
	for _, tag := range ex.RiskHints {
		switch tag {
		case ExplainTagSignalExpired:
			score -= healthSignalExpiredPenalty
			risks = append(risks, ExplainTagSignalExpired)
		case ExplainTagLossControl:
			score -= healthLossControlPenalty
			risks = append(risks, ExplainTagLossControl)
		case ExplainTagPriceStale:
			score -= healthPriceStalePenalty
			risks = append(risks, ExplainTagPriceStale)
		case ExplainTagNoSourceTrace:
			score -= healthNoSourceTracePenalty
			risks = append(risks, ExplainTagNoSourceTrace)
		}
	}

	score = clampHealthScore(score)
	grade, label := MapHoldingHealthGrade(score)
	exCopy := ex
	return HoldingHealthScore{
		StockCode:         code,
		EvaluationTime:    asOf,
		Score:             score,
		Grade:             grade,
		GradeLabel:        label,
		SupportingFactors: factors,
		RiskFactors:       risks,
		Explanation:       &exCopy,
		DataSourceNote:    holdingHealthScoreNote,
	}
}

func clampHealthScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// MapHoldingHealthGrade maps score → A/B/C/D (not a sell recommendation).
func MapHoldingHealthGrade(score int) (grade, label string) {
	score = clampHealthScore(score)
	switch {
	case score >= 80:
		return HealthGradeA, "健康持有"
	case score >= 60:
		return HealthGradeB, "正常观察"
	case score >= 40:
		return HealthGradeC, "重点关注"
	default:
		return HealthGradeD, "风险较高"
	}
}

// EnrichHoldingWithHealthScores attaches HoldingHealthScore after Explanation is present.
func EnrichHoldingWithHealthScores(holding *HoldingEvalObservationView) {
	if holding == nil {
		return
	}
	for i := range holding.Holdings {
		ex := holding.Holdings[i].Explanation
		if ex == nil {
			built := BuildPositionEvaluationExplanation(holding.Holdings[i], ExplanationSourceHint{}, ExplanationOptions{AsOf: holding.AsOf})
			ex = &built
			holding.Holdings[i].Explanation = ex
		}
		hs := BuildHoldingHealthScore(*ex, &holding.Holdings[i])
		holding.Holdings[i].HealthScore = &hs
	}
}
