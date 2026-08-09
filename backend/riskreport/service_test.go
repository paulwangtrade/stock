package riskreport_test

import (
	"context"
	"testing"
	"time"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
	"go-stock/backend/riskreport"

	"github.com/stretchr/testify/require"
)

func proUser(t *testing.T) (string, string) {
	t.Helper()
	u := &featuregate.User{ID: "risk-pro", Tier: featuregate.TierPro}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(u))
	require.True(t, featuregate.Allow(u, featuregate.FeatureAdvancedRisk))
	return u.ID, string(u.Tier)
}

func TestBuild_ReportAggregation(t *testing.T) {
	uid, tier := proUser(t)
	conc := 0.55
	posRatio := 0.9
	cash := 1000.0
	src := &riskreport.ReportSources{
		PortfolioQuality:  "OK",
		Cash:              &cash,
		PositionRatio:     &posRatio,
		Concentration:     &conc,
		PositionSymbols:   []string{"sz000001"},
		HoldingRiskStates: map[string]int{"DANGER": 1, "WATCH": 2, "NORMAL": 3},
		HoldingSymbols:    map[string][]string{"DANGER": {"sz000001"}, "WATCH": {"sz000002", "sz000003"}},
		ExitReasonCounts:  map[string]int{"TIME_REVIEW": 1, "LOSS_REVIEW": 1},
		ExitReviewStates:  map[string]int{"REVIEW_REQUIRED": 1, "NORMAL": 4},
		ExecEnabled:       true,
		ExecTotalOrders:   10,
		ExecFilled:        6,
		ExecFailed:        4,
		ExecFillRate:      0.6,
		ExecDataNote:      "Execution Summary · read-only via ExecutionReadService",
		ExecDataSource:    "paper_sim",
		MarketLevel:       2,
		MarketState:       "OPEN",
		MarketTrading:     true,
	}

	out, err := riskreport.NewService().Build(context.Background(), riskreport.BuildRequest{
		UserID: uid, Tier: tier, TradeDate: "2026-08-09", Sources: src,
		Now: time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.True(t, out.Status == riskreport.StatusOK || out.Status == riskreport.StatusDegraded)
	require.Equal(t, riskreport.SchemaVersion, out.SchemaVersion)
	require.Greater(t, out.Score.Overall, 30)
	require.Contains(t, []string{riskreport.BandMedium, riskreport.BandHigh}, out.Score.Band)
	require.NotEmpty(t, out.Factors)
	require.True(t, hasFactor(out, "PORTFOLIO_CONCENTRATION"))
	require.True(t, hasFactor(out, "POSITION_DANGER"))
	require.True(t, hasFactor(out, "EXEC_REJECT_RATE"))
	require.True(t, hasFactor(out, "MARKET_LEVEL_ELEVATED"))
	require.NotEmpty(t, out.Warnings)
	require.NotEmpty(t, out.Suggestions)
	for _, s := range out.Suggestions {
		require.NotEqual(t, "execute", s.Kind)
		require.NotContains(t, s.Message, "市价卖出")
	}
	require.True(t, out.Dimensions.Portfolio.Available)
	require.True(t, out.Dimensions.Position.Available)
	require.NotEmpty(t, out.Disclaimers)
}

func TestBuild_EmptyData(t *testing.T) {
	uid, tier := proUser(t)
	src := &riskreport.ReportSources{
		PortfolioQuality: "UNKNOWN",
		// no holdings / exec → mostly unavailable
	}
	out, err := riskreport.NewService().Build(context.Background(), riskreport.BuildRequest{
		UserID: uid, Tier: tier, Sources: src,
	})
	require.NoError(t, err)
	require.Equal(t, riskreport.StatusDegraded, out.Status)
	require.True(t, out.Score.Overall >= 0)
	require.False(t, out.Dimensions.Portfolio.Available)
	require.False(t, out.Dimensions.Position.Available)
	require.True(t, hasWarning(out, "PORTFOLIO_DATA_UNKNOWN"))
}

func TestBuild_Gated(t *testing.T) {
	free := &featuregate.User{ID: "risk-free", Tier: featuregate.TierFree}
	require.NoError(t, entitlement.Default().EnsureTierDefaults(free))
	out, err := riskreport.NewService().Build(context.Background(), riskreport.BuildRequest{
		UserID: free.ID, Tier: string(free.Tier),
		Sources: &riskreport.ReportSources{PortfolioQuality: "OK"},
	})
	require.NoError(t, err)
	require.Equal(t, riskreport.StatusGated, out.Status)
	require.Empty(t, out.Factors)
}

func TestAggregate_RiskScoresIncreaseWithStress(t *testing.T) {
	uid, tier := proUser(t)
	calm := &riskreport.ReportSources{
		PortfolioQuality:  "OK",
		HoldingRiskStates: map[string]int{"NORMAL": 5},
		ExitReasonCounts:  map[string]int{},
		ExitReviewStates:  map[string]int{"NORMAL": 5},
		ExecEnabled:       true,
		ExecTotalOrders:   5,
		ExecFilled:        5,
		MarketState:       "OPEN",
		MarketTrading:     true,
	}
	stressed := &riskreport.ReportSources{
		PortfolioQuality:  "OK",
		Concentration:     floatPtr(0.6),
		PositionRatio:     floatPtr(0.95),
		Cash:              floatPtr(0),
		HoldingRiskStates: map[string]int{"DANGER": 3},
		ExitReasonCounts:  map[string]int{"LOSS_REVIEW": 3},
		ExitReviewStates:  map[string]int{"REVIEW_REQUIRED": 3},
		ExecEnabled:       true,
		ExecTotalOrders:   10,
		ExecFailed:        5,
		ExecFilled:        5,
		MarketLevel:       3,
		MarketState:       "OPEN",
		MarketTrading:     true,
	}
	svc := riskreport.NewService()
	a, err := svc.Build(context.Background(), riskreport.BuildRequest{UserID: uid, Tier: tier, Sources: calm})
	require.NoError(t, err)
	b, err := svc.Build(context.Background(), riskreport.BuildRequest{UserID: uid, Tier: tier, Sources: stressed})
	require.NoError(t, err)
	require.Greater(t, b.Score.Overall, a.Score.Overall)
	require.Greater(t, len(b.Factors), len(a.Factors))
}

func hasFactor(r *riskreport.RiskReport, code string) bool {
	for _, f := range r.Factors {
		if f.Code == code {
			return true
		}
	}
	return false
}

func hasWarning(r *riskreport.RiskReport, code string) bool {
	for _, w := range r.Warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

func floatPtr(v float64) *float64 { return &v }
