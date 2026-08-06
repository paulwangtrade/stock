package data

import (
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func TestGetTradePlanVisibilityByID(t *testing.T) {
	setupPaperTradingTestDB(t)
	require.NoError(t, EnsureTradePlanTables())
	repo := NewTradePlanRepo()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: "2026-07-28", GeneratedAt: now, PoolID: 1,
		Status: models.TradePlanStatusDraft, PlanVersion: 1,
		SourceSession: models.TradePlanSourceAfterClose,
		RiskStatus:    risk.PlanRiskStatusPassed,
	}
	require.NoError(t, repo.CreatePlanWithItems(plan, []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", Priority: 1, TargetAmount: 5000, Status: models.TradePlanItemPending},
	}))

	view, err := repo.GetTradePlanVisibilityByID(plan.ID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, plan.ID, view.ID)
	require.Equal(t, "2026-07-28", view.TradeDate)
	require.Len(t, view.Items, 1)
}
