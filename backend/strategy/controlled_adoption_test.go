package strategy

import (
	"fmt"
	"testing"
	"time"

	"go-stock/backend/controlledadoption"
	"go-stock/backend/data"
	"go-stock/backend/decisionprovider"
	"go-stock/backend/models"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfoliolayer"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func stubControlledFilterBypass(t *testing.T) {
	t.Helper()
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled: false, MarketLevel: 3, AmountPerStock: amount, MaxNames: maxNames,
			Cash: 2_000_000, EquityBase: 2_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })
}

func seedControlledPool(t *testing.T, tradeDate, strategy string, codes ...string) *models.CandidatePool {
	t.Helper()
	items := make([]models.CandidatePoolItem, 0, len(codes))
	for i, code := range codes {
		items = append(items, models.CandidatePoolItem{
			TradeDate:    tradeDate,
			StockCode:    code,
			StockName:    code,
			Rank:         i + 1,
			Score:        float64(len(codes) - i),
			StrategyName: strategy,
		})
	}
	pool := &models.CandidatePool{
		TradeDate:   tradeDate,
		GeneratedAt: time.Now(),
		Source:      models.CandidatePoolSourceFollow,
		Status:      models.CandidatePoolStatusReady,
	}
	require.NoError(t, data.NewCandidatePoolRepo().CreatePoolWithItems(pool, items))
	got, err := data.NewCandidatePoolRepo().GetByID(pool.ID)
	require.NoError(t, err)
	return got
}

func installControlledSnap(t *testing.T, accountID uint) {
	t.Helper()
	prev := loadControlledPortfolioSnapshot
	loadControlledPortfolioSnapshot = func() *portfoliolayer.PortfolioSnapshot {
		return portfoliolayer.FromLedger(&portfolio.Snapshot{
			AsOf:          time.Date(2026, 8, 21, 15, 0, 0, 0, time.Local),
			AccountID:     accountID,
			Found:         true,
			TotalEquity:   2_000_000,
			Cash:          1_500_000,
			AvailableCash: 1_500_000,
			MarketValue:   500_000,
			TotalExposure: 500_000,
		})
	}
	t.Cleanup(func() { loadControlledPortfolioSnapshot = prev })
}

func TestControlledAdoption_DefaultOff_UsesLegacy(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubControlledFilterBypass(t)
	controlledadoption.ResetActive()
	t.Cleanup(controlledadoption.ResetActive)

	pool := seedControlledPool(t, "2026-08-22", "alpha", "sz000002", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeOff, plan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, plan.DecisionProvider)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			require.Equal(t, 100_000.0, it.TargetAmount)
		}
	}
}

func TestControlledAdoption_WhitelistMiss_UsesLegacy_StampsControlled(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubControlledFilterBypass(t)
	installControlledSnap(t, 7)
	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		AccountIDs:    []uint{7},
		StrategyNames: []string{"alpha"},
		TradeDates:    []string{"2099-01-01"}, // date miss
	})
	t.Cleanup(controlledadoption.ResetActive)

	pool := seedControlledPool(t, "2026-08-22", "alpha", "sz000002", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeControlled, plan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, plan.DecisionProvider)
	require.Equal(t, "", plan.AllocationVersion)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
}

func TestControlledAdoption_WhitelistHit_UsesPortfolio(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubControlledFilterBypass(t)
	installControlledSnap(t, 7)
	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		AccountIDs:    []uint{7},
		StrategyNames: []string{"alpha"},
		TradeDates:    []string{"2026-08-22"},
	})
	t.Cleanup(controlledadoption.ResetActive)

	pool := seedControlledPool(t, "2026-08-22", "alpha", "sz000002", "sz000003", "sz000004")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeControlled, plan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderPortfolio, plan.DecisionProvider)
	require.Equal(t, models.TradePlanAllocationVersionF1Equal, plan.AllocationVersion)
	require.Equal(t, models.TradePlanDecisionVersionG21, plan.DecisionVersion)

	var pendingAmt []float64
	for _, it := range plan.Items {
		if it.Status == models.TradePlanItemPending {
			pendingAmt = append(pendingAmt, it.TargetAmount)
			require.NotEqual(t, 100_000.0, it.TargetAmount, "portfolio amounts should not be fixed sizer")
			require.Greater(t, it.TargetAmount, 0.0)
		}
	}
	require.NotEmpty(t, pendingAmt)
}

func TestControlledAdoption_KillSwitch_ForcesLegacy(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubControlledFilterBypass(t)
	installControlledSnap(t, 7)
	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		KillSwitch:    true,
		AccountIDs:    []uint{7},
		StrategyNames: []string{"alpha"},
		TradeDates:    []string{"2026-08-22"},
	})
	t.Cleanup(controlledadoption.ResetActive)

	pool := seedControlledPool(t, "2026-08-22", "alpha", "sz000002", "sz000001")
	plan, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanProviderModeControlled, plan.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderFixed, plan.DecisionProvider)
	require.Equal(t, 100_000.0, plan.AmountPerStock)
}

func TestControlledAdoption_PortfolioFail_BlocksDraft_NoLegacyFallback(t *testing.T) {
	setupDraftPlanTestDB(t)
	stubControlledFilterBypass(t)
	installControlledSnap(t, 7)
	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:      controlledadoption.AdoptionControlled,
		AccountIDs:    []uint{7},
		StrategyNames: []string{"alpha"},
		TradeDates:    []string{"2026-08-22"},
	})
	t.Cleanup(controlledadoption.ResetActive)

	prevProv := portfolioDecisionProviderForControlled
	portfolioDecisionProviderForControlled = &failingPortfolioProvider{}
	t.Cleanup(func() { portfolioDecisionProviderForControlled = prevProv })

	pool := seedControlledPool(t, "2026-08-22", "alpha", "sz000002", "sz000001")
	_, err := BuildDraftTradePlanFromCandidatePool(pool)
	require.Error(t, err)
	require.Contains(t, err.Error(), "controlled adoption blocked draft")
	require.Contains(t, err.Error(), "no legacy fallback")
}

func TestControlledAdoption_FrozenPlan_MetadataImmutable(t *testing.T) {
	setupDraftPlanTestDB(t)
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate:         "2026-08-22",
		GeneratedAt:       now,
		Status:            models.TradePlanStatusReady,
		PlanVersion:       1,
		Side:              "buy",
		AmountPerStock:    55_000,
		EnableExecute:     true,
		FreezeAt:          &now,
		ProviderMode:      models.TradePlanProviderModeControlled,
		DecisionProvider:  models.TradePlanDecisionProviderPortfolio,
		DecisionVersion:   models.TradePlanDecisionVersionG21,
		AllocationVersion: models.TradePlanAllocationVersionF1Equal,
	}
	require.NoError(t, data.NewTradePlanRepo().CreatePlanWithItems(plan, []models.TradePlanItem{
		{TradeDate: "2026-08-22", StockCode: "sz000002", Priority: 1, TargetAmount: 55_000, Status: models.TradePlanItemPending},
	}))

	controlledadoption.SetActive(controlledadoption.ControlledProviderPolicy{
		Adoption:   controlledadoption.AdoptionOff,
		KillSwitch: true,
	})
	t.Cleanup(controlledadoption.ResetActive)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.True(t, got.IsFrozen())
	require.Equal(t, models.TradePlanProviderModeControlled, got.ProviderMode)
	require.Equal(t, models.TradePlanDecisionProviderPortfolio, got.DecisionProvider)
	require.Equal(t, models.TradePlanAllocationVersionF1Equal, got.AllocationVersion)
	require.Equal(t, 55_000.0, got.Items[0].TargetAmount)
}

type failingPortfolioProvider struct{}

func (f *failingPortfolioProvider) Name() string { return decisionprovider.ProviderPortfolioAllocation }

func (f *failingPortfolioProvider) Decide(ctx decisionprovider.DecisionContext) (*decisionprovider.DecisionEnvelope, error) {
	return &decisionprovider.DecisionEnvelope{
		OK:       false,
		Provider: decisionprovider.ProviderPortfolioAllocation,
		Error: &decisionprovider.DecisionError{
			Code:    decisionprovider.ErrCodeInvalidAllocation,
			Message: "injected controlled failure",
		},
		DecisionTime: ctx.DecisionTime,
		Version:      ctx.Version,
	}, fmt.Errorf("injected controlled failure")
}

var _ decisionprovider.DecisionProvider = (*failingPortfolioProvider)(nil)
