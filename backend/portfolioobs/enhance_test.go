package portfolioobs

import (
	"encoding/json"
	"strings"
	"testing"

	"go-stock/backend/holdingdecision"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestClassifyAging(t *testing.T) {
	b, aging := ClassifyAging(3, "2026-08-10")
	require.Equal(t, AgingShort, b)
	require.False(t, aging)

	b, aging = ClassifyAging(5, "2026-08-01")
	require.Equal(t, AgingMedium, b)
	require.False(t, aging)

	b, aging = ClassifyAging(30, "2026-07-01")
	require.Equal(t, AgingMedium, b)
	require.False(t, aging)

	b, aging = ClassifyAging(31, "2026-06-01")
	require.Equal(t, AgingLong, b)
	require.True(t, aging)

	b, aging = ClassifyAging(100, "")
	require.Equal(t, AgingUnknown, b)
	require.False(t, aging, "missing buy date must not become AGING/LONG")
}

func TestScoreHealth_ProfitableNormal(t *testing.T) {
	px, ret := 11.0, 0.10
	h := ScoreHealth(HealthInput{
		CurrentPrice:  &px,
		Return:        &ret,
		DecisionState: holdingdecision.StateHoldNormal,
		RiskState:     papertrading.RiskStateNormal,
		ProfitState:   papertrading.ProfitStateProfit,
		AgingBucket:   AgingShort,
	})
	require.False(t, h.Incomplete)
	require.NotNil(t, h.Score)
	require.InDelta(t, 100.0, *h.Score, 1e-9)
	require.Equal(t, HealthHealthy, h.Level)
}

func TestScoreHealth_LongHoldingAgingOnly(t *testing.T) {
	px, ret := 11.0, 0.08
	h := ScoreHealth(HealthInput{
		CurrentPrice:  &px,
		Return:        &ret,
		DecisionState: holdingdecision.StateHoldNormal,
		RiskState:     papertrading.RiskStateNormal,
		ProfitState:   papertrading.ProfitStateProfit,
		AgingBucket:   AgingLong,
	})
	require.NotNil(t, h.Score)
	require.InDelta(t, 90.0, *h.Score, 1e-9)
	require.Equal(t, HealthHealthy, h.Level)
	require.NotEqual(t, HealthRisk, h.Level)
}

func TestScoreHealth_MissingDataNotUpgraded(t *testing.T) {
	h := ScoreHealth(HealthInput{
		DecisionState: holdingdecision.StateHoldNormal,
		RiskState:     papertrading.RiskStateNormal, // Evaluation default when return nil
		AgingBucket:   AgingUnknown,
	})
	require.True(t, h.Incomplete)
	require.Nil(t, h.Score)
	require.Equal(t, HealthUnknown, h.Level)
}

func TestProjectDecisionHistory_NormalWatchReview(t *testing.T) {
	hist := ProjectDecisionHistory("sh600000", []DecisionHistoryPoint{
		{AsOf: "2026-08-10", State: holdingdecision.StateHoldNormal, Source: "injected"},
		{AsOf: "2026-08-11", State: holdingdecision.StateHoldWatch, Source: "injected"},
		{AsOf: "2026-08-12", State: holdingdecision.StateHoldReview, Source: "injected"},
	})
	require.True(t, hist.Complete)
	require.Equal(t, 2, hist.TransitionCount)
	require.Equal(t, holdingdecision.StateHoldNormal, hist.Points[0].State)
	require.Equal(t, holdingdecision.StateHoldWatch, hist.Points[1].State)
	require.Equal(t, holdingdecision.StateHoldReview, hist.Points[2].State)
	raw, err := json.Marshal(hist)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
}

func TestEnhance_LongHoldingNoSell(t *testing.T) {
	px, ret := 11.0, 0.05
	obs := &Observation{
		AsOf: "2026-08-12T08:00:00+08:00",
		Positions: []PositionRow{
			{
				Symbol: "sz000001", Action: actionNone, HoldingDays: 45, FirstBuyDate: "2026-06-20",
				CurrentPrice: &px, Return: &ret,
				DecisionState: holdingdecision.StateHoldNormal,
				RiskState:     papertrading.RiskStateNormal,
				ProfitState:   papertrading.ProfitStateProfit,
			},
		},
	}
	Enhance(obs)
	require.Equal(t, AgingLong, obs.Positions[0].HoldingPeriodBucket)
	require.True(t, obs.Positions[0].IsAging)
	require.Equal(t, HealthHealthy, obs.Positions[0].HealthLevel)
	require.Equal(t, 1, obs.Health.AgingPositions)
	require.Equal(t, actionNone, obs.Positions[0].Action)
	require.Equal(t, actionNone, obs.Action)
	require.False(t, obs.OpportunityCost.Available)
	require.Equal(t, OpportunityCostUnknown, obs.OpportunityCost.Level)
	require.Len(t, obs.Positions[0].DecisionHistory, 1)
	require.Contains(t, obs.Disclaimer, "不会自动卖出")
	raw, err := json.Marshal(obs)
	require.NoError(t, err)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
}

func TestEnhance_MissingDataPortfolioUnknown(t *testing.T) {
	obs := &Observation{
		AsOf: "2026-08-12",
		Positions: []PositionRow{
			{Symbol: "sz000002", Action: actionNone, DecisionState: holdingdecision.StateHoldNormal, RiskState: papertrading.RiskStateNormal},
		},
	}
	Enhance(obs)
	require.Equal(t, HealthUnknown, obs.Positions[0].HealthLevel)
	require.Equal(t, HealthUnknown, obs.Health.PortfolioHealth)
	require.Equal(t, 1, obs.Health.UnknownHealthCount)
	require.Equal(t, 0, obs.Health.RiskPositions)
}
