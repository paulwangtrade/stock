package strategy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/risk"
	"go-stock/backend/tradingcalendar"

	"github.com/stretchr/testify/require"
)

func TestRunAfterClosePlanWorkflow_NormalFlow(t *testing.T) {
	wf := &AfterClosePlanWorkflow{
		Calendar: tradingcalendar.Calendar{},
		buildPool: func(tradeDate string, options ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			require.Equal(t, "2026-07-27", tradeDate) // Mon after Fri
			cfg := map[string]any{}
			for _, opt := range options {
				opt(cfg)
			}
			require.Equal(t, "after_close", cfg["session"])
			require.Equal(t, "2026-07-24", cfg["source_date"])
			return &models.CandidatePool{
				ID:        11,
				TradeDate: tradeDate,
				Status:    models.CandidatePoolStatusReady,
				ItemCount: 1,
				Items:     []models.CandidatePoolItem{{StockCode: "sz000001", Rank: 1}},
			}, nil
		},
		buildDraft: func(pool *models.CandidatePool) (*models.TradePlan, error) {
			require.Equal(t, uint(11), pool.ID)
			return &models.TradePlan{
				ID:          21,
				TradeDate:   pool.TradeDate,
				Status:      models.TradePlanStatusDraft,
				PlanVersion: 1,
				PoolID:      pool.ID,
			}, nil
		},
		evalRisk: func(plan *models.TradePlan) (*RiskProposalResult, error) {
			require.Equal(t, uint(21), plan.ID)
			return &RiskProposalResult{
				TradePlanID:      plan.ID,
				TradePlanVersion: plan.PlanVersion,
				Passed:           true,
				CheckedAt:        time.Now(),
			}, nil
		},
	}

	res, err := wf.Run("2026-07-24") // Friday
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, "2026-07-24", res.SourceDate)
	require.Equal(t, "2026-07-27", res.TradeDate)
	require.Equal(t, uint(11), res.CandidatePoolID)
	require.Equal(t, uint(21), res.TradePlanID)
	require.Equal(t, 1, res.PlanVersion)
	require.True(t, res.RiskPassed)
	require.Empty(t, res.FailedStep)
}

func TestRunAfterClosePlanWorkflow_NonTradingSourceDate(t *testing.T) {
	// Saturday → next Monday
	calledTradeDate := ""
	wf := &AfterClosePlanWorkflow{
		Calendar: tradingcalendar.Calendar{},
		buildPool: func(tradeDate string, _ ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			calledTradeDate = tradeDate
			return &models.CandidatePool{
				ID: 1, TradeDate: tradeDate, Status: models.CandidatePoolStatusReady, ItemCount: 1,
				Items: []models.CandidatePoolItem{{StockCode: "sz000001"}},
			}, nil
		},
		buildDraft: func(pool *models.CandidatePool) (*models.TradePlan, error) {
			return &models.TradePlan{ID: 2, TradeDate: pool.TradeDate, Status: models.TradePlanStatusDraft, PlanVersion: 1}, nil
		},
		evalRisk: func(plan *models.TradePlan) (*RiskProposalResult, error) {
			return &RiskProposalResult{TradePlanID: plan.ID, TradePlanVersion: 1, Passed: true}, nil
		},
	}

	res, err := wf.Run("2026-07-25") // Saturday
	require.NoError(t, err)
	require.True(t, res.OK)
	require.Equal(t, "2026-07-25", res.SourceDate)
	require.Equal(t, "2026-07-27", res.TradeDate)
	require.Equal(t, "2026-07-27", calledTradeDate)
}

func TestRunAfterClosePlanWorkflow_RiskFailSoft(t *testing.T) {
	wf := &AfterClosePlanWorkflow{
		Calendar: tradingcalendar.Calendar{},
		buildPool: func(tradeDate string, _ ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			return &models.CandidatePool{
				ID: 3, TradeDate: tradeDate, Status: models.CandidatePoolStatusReady, ItemCount: 1,
				Items: []models.CandidatePoolItem{{StockCode: "sz000001"}},
			}, nil
		},
		buildDraft: func(pool *models.CandidatePool) (*models.TradePlan, error) {
			return &models.TradePlan{ID: 4, TradeDate: pool.TradeDate, Status: models.TradePlanStatusDraft, PlanVersion: 2}, nil
		},
		evalRisk: func(plan *models.TradePlan) (*RiskProposalResult, error) {
			return &RiskProposalResult{
				TradePlanID:      plan.ID,
				TradePlanVersion: plan.PlanVersion,
				Passed:           false,
				RiskReasons:      []string{string(risk.ReasonMarketLevelBlocked)},
			}, nil
		},
	}

	res, err := wf.Run("2026-07-24")
	require.NoError(t, err) // soft failure
	require.False(t, res.OK)
	require.False(t, res.RiskPassed)
	require.Equal(t, "risk", res.FailedStep)
	require.Equal(t, uint(3), res.CandidatePoolID)
	require.Equal(t, uint(4), res.TradePlanID)
	require.Equal(t, 2, res.PlanVersion)
}

func TestRunAfterClosePlanWorkflow_RepeatIncrementsPlanVersion(t *testing.T) {
	version := 0
	wf := &AfterClosePlanWorkflow{
		Calendar: tradingcalendar.Calendar{},
		buildPool: func(tradeDate string, _ ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			return &models.CandidatePool{
				ID: uint(10 + version), TradeDate: tradeDate, Status: models.CandidatePoolStatusReady, ItemCount: 1,
				Items: []models.CandidatePoolItem{{StockCode: "sz000001"}},
			}, nil
		},
		buildDraft: func(pool *models.CandidatePool) (*models.TradePlan, error) {
			version++
			return &models.TradePlan{
				ID:          uint(100 + version),
				TradeDate:   pool.TradeDate,
				Status:      models.TradePlanStatusDraft,
				PlanVersion: version,
				PoolID:      pool.ID,
			}, nil
		},
		evalRisk: func(plan *models.TradePlan) (*RiskProposalResult, error) {
			return &RiskProposalResult{TradePlanID: plan.ID, TradePlanVersion: plan.PlanVersion, Passed: true}, nil
		},
	}

	first, err := wf.Run("2026-07-24")
	require.NoError(t, err)
	require.Equal(t, 1, first.PlanVersion)

	second, err := wf.Run("2026-07-24")
	require.NoError(t, err)
	require.Equal(t, 2, second.PlanVersion)
	require.NotEqual(t, first.TradePlanID, second.TradePlanID)
	require.Equal(t, first.TradeDate, second.TradeDate)
}

func TestRunAfterClosePlanWorkflow_InvalidSourceDate(t *testing.T) {
	wf := NewAfterClosePlanWorkflow()
	res, err := wf.Run("not-a-date")
	require.Error(t, err)
	require.NotNil(t, res)
	require.Equal(t, "calendar", res.FailedStep)
	require.False(t, res.OK)
}

func TestAfterCloseWorkflow_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("after_close_workflow.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"ApproveTradePlan(",
		"FreezeTradePlan(",
		"RunDailyCandidateAndPlan(",
		"TryBeginExecute(",
		"RunPaperOpenBuyOnce(",
		"TradingPreflight",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("workflow must not reference %s", token)
		}
	}
	require.Contains(t, src, "BuildCandidatePool")
	require.Contains(t, src, "BuildDraftTradePlanFromCandidatePool")
	require.Contains(t, src, "EvaluateDraftTradePlanRisk")
	require.Contains(t, src, "NextTradingDayString")
}

func TestRunAfterClosePlanWorkflow_CandidateHardFail(t *testing.T) {
	wf := &AfterClosePlanWorkflow{
		Calendar: tradingcalendar.Calendar{},
		buildPool: func(string, ...BuildCandidatePoolOption) (*models.CandidatePool, error) {
			return nil, fmt.Errorf("pool boom")
		},
	}
	res, err := wf.Run("2026-07-24")
	require.Error(t, err)
	require.Equal(t, "candidate", res.FailedStep)
	require.False(t, res.OK)
}
