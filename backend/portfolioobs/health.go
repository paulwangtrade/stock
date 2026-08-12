package portfolioobs

import (
	"math"
	"strings"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"
)

// HealthInput is observation facts for ScoreHealth (no DB).
type HealthInput struct {
	CurrentPrice  *float64
	Return        *float64
	DecisionState string
	RiskState     string
	ProfitState   string
	AgingBucket   string
}

// HealthResult is a read-only observation score (not Strategy Score, not a trade).
type HealthResult struct {
	Score      *float64
	Level      string
	Incomplete bool
	Note       string
}

func ScoreHealth(in HealthInput) HealthResult {
	if in.CurrentPrice == nil && in.Return == nil {
		return HealthResult{
			Level:      HealthUnknown,
			Incomplete: true,
			Note:       "missing price/return; health not upgraded",
		}
	}

	score := 100.0
	dec := strings.ToUpper(strings.TrimSpace(in.DecisionState))
	switch dec {
	case holdingdecision.StateHoldWatch:
		score -= 20
	case holdingdecision.StateHoldReview:
		score -= 45
	case holdingdecision.StateExitCandidate:
		score -= 55
	}

	switch strings.ToUpper(strings.TrimSpace(in.RiskState)) {
	case papertrading.RiskStateWatch:
		score -= 15
	case papertrading.RiskStateDanger:
		score -= 30
	}

	switch strings.ToUpper(strings.TrimSpace(in.ProfitState)) {
	case papertrading.ProfitStateBreakeven:
		score -= 5
	case papertrading.ProfitStateLoss:
		score -= 15
	}

	switch strings.ToUpper(strings.TrimSpace(in.AgingBucket)) {
	case AgingMedium:
		score -= 5
	case AgingLong:
		score -= 10
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return HealthResult{Level: HealthUnknown, Incomplete: true, Note: "invalid score; not upgraded"}
	}

	level := levelFromScore(score)
	if (dec == holdingdecision.StateHoldReview || dec == holdingdecision.StateExitCandidate) &&
		healthRank(level) < healthRank(HealthWatch) {
		level = HealthWatch
	}
	s := score
	return HealthResult{Score: &s, Level: level}
}

func levelFromScore(score float64) string {
	if score >= 80 {
		return HealthHealthy
	}
	if score >= 50 {
		return HealthNormal
	}
	if score >= 30 {
		return HealthWatch
	}
	return HealthRisk
}

func healthRank(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case HealthRisk:
		return 4
	case HealthWatch:
		return 3
	case HealthNormal:
		return 2
	case HealthHealthy:
		return 1
	default:
		return 0
	}
}
