package strategy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go-stock/backend/models"
	"go-stock/backend/risk"

	"github.com/stretchr/testify/require"
)

func draftPlanForRiskEval(id uint, version int, items ...models.TradePlanItem) *models.TradePlan {
	return &models.TradePlan{
		ID:             id,
		TradeDate:      "2026-07-28",
		PoolID:         7,
		Status:         models.TradePlanStatusDraft,
		PlanVersion:    version,
		EnableExecute:  false,
		AmountPerStock: 100_000,
		MaxNames:       5,
		SourceSession:  models.TradePlanSourceAfterClose,
		Items:          items,
	}
}

func TestEvaluateDraftTradePlanRisk_PassBypassed(t *testing.T) {
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:        false,
			MarketLevel:    3,
			AmountPerStock: amount,
			MaxNames:       maxNames,
			Cash:           1_000_000,
			EquityBase:     1_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })

	plan := draftPlanForRiskEval(11, 1,
		models.TradePlanItem{StockCode: "sz000001", StockName: "平安", Priority: 1, TargetAmount: 100_000, Status: models.TradePlanItemPending},
		models.TradePlanItem{StockCode: "sh600519", StockName: "茅台", Priority: 2, TargetAmount: 100_000, Status: models.TradePlanItemPending},
	)
	statusBefore := plan.Status

	res, err := EvaluateDraftTradePlanRisk(plan)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, uint(11), res.TradePlanID)
	require.Equal(t, 1, res.TradePlanVersion)
	require.True(t, res.Passed)
	require.NotNil(t, res.PlanFilterResult)
	require.Equal(t, risk.PlanRiskStatusBypassed, res.PlanFilterResult.RiskStatus)
	require.Empty(t, res.RiskReasons)
	require.False(t, res.CheckedAt.IsZero())
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Equal(t, statusBefore, plan.Status)
	require.False(t, plan.EnableExecute)
	require.Nil(t, plan.FreezeAt)
}

func TestEvaluateDraftTradePlanRisk_RejectBlocked(t *testing.T) {
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:         true,
			MarketLevel:     1,
			BlockNewEntries: true,
			AmountPerStock:  amount,
			MaxNames:        maxNames,
			Cash:            1_000_000,
			EquityBase:      1_000_000,
			ScanLimit:       10,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })

	plan := draftPlanForRiskEval(12, 2,
		models.TradePlanItem{StockCode: "sz000001", Priority: 1, TargetAmount: 100_000},
	)

	res, err := EvaluateDraftTradePlanRisk(plan)
	require.NoError(t, err)
	require.False(t, res.Passed)
	require.Equal(t, risk.PlanRiskStatusBlocked, res.PlanFilterResult.RiskStatus)
	require.NotEmpty(t, res.RiskReasons)
	require.Contains(t, res.RiskReasons[0], string(risk.ReasonMarketLevelBlocked))
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.False(t, plan.IsExecutableStatus())
}

func TestEvaluateDraftTradePlanRisk_RepeatEvaluation(t *testing.T) {
	prev := planFilterContextFn
	planFilterContextFn = func(amount float64, maxNames int) risk.PlanContext {
		return risk.PlanContext{
			Enabled:        false,
			MarketLevel:    3,
			AmountPerStock: amount,
			MaxNames:       maxNames,
			Cash:           1_000_000,
			EquityBase:     1_000_000,
		}
	}
	t.Cleanup(func() { planFilterContextFn = prev })

	plan := draftPlanForRiskEval(13, 3,
		models.TradePlanItem{StockCode: "sz000001", Priority: 1, TargetAmount: 100_000},
	)

	first, err := EvaluateDraftTradePlanRisk(plan)
	require.NoError(t, err)
	time.Sleep(2 * time.Millisecond)
	second, err := EvaluateDraftTradePlanRisk(plan)
	require.NoError(t, err)

	require.True(t, first.Passed)
	require.True(t, second.Passed)
	require.Equal(t, first.TradePlanID, second.TradePlanID)
	require.Equal(t, first.TradePlanVersion, second.TradePlanVersion)
	require.False(t, second.CheckedAt.Before(first.CheckedAt))
	require.Equal(t, models.TradePlanStatusDraft, plan.Status)
	require.Nil(t, plan.FreezeAt)
	require.Nil(t, plan.ApprovedAt)
}

func TestEvaluateDraftTradePlanRisk_RejectsNonDraft(t *testing.T) {
	plan := draftPlanForRiskEval(14, 1,
		models.TradePlanItem{StockCode: "sz000001", Priority: 1, TargetAmount: 100_000},
	)
	plan.Status = models.TradePlanStatusReady
	_, err := EvaluateDraftTradePlanRisk(plan)
	require.Error(t, err)
}

func TestEvaluateDraftTradePlanRisk_SourceBoundary(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("evaluate_draft_risk_proposal.go"))
	require.NoError(t, err)
	src := string(b)
	forbidden := []string{
		"RunDailyCandidateAndPlan(",
		"TryBeginExecute(",
		"TradePlanStatusReady",
		"PreTradeCheck(",
		"TradingPreflight",
		"ApprovedAt:",
		"FreezeAt:",
	}
	for _, token := range forbidden {
		if strings.Contains(src, token) {
			t.Fatalf("risk proposal eval must not reference %s", token)
		}
	}
	require.Contains(t, src, "PlanFilter")
	require.Contains(t, src, "IsDraft")
}
