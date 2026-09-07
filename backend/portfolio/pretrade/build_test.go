package pretrade_test

import (
	"testing"
	"time"

	"go-stock/backend/portfolio/pretrade"

	"github.com/stretchr/testify/require"
)

func TestBuild_NormalCash(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		PlanID: 1, TradeDate: "2026-08-18", AsOf: time.Now(),
		Account: pretrade.AccountInput{Cash: 500_000, Equity: 2_000_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", StockName: "平安", Side: "buy", TargetAmount: 100_000, Status: "pending"},
		},
	})
	require.Equal(t, pretrade.LevelPass, view.Level)
	require.True(t, view.Allowed)
	require.Equal(t, pretrade.StatusPass, view.CashCheck.Status)
	require.True(t, view.CashCheck.CashEnough)
	require.Equal(t, 1, view.ExistingPositionCheck.NewCount)
	require.Equal(t, pretrade.ActionNew, view.PositionActions[0].Action)
	require.False(t, view.RequiresConfirm)
}

func TestBuild_InsufficientCash(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		PlanID: 2,
		Account: pretrade.AccountInput{Cash: 50_000, Equity: 1_000_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", Side: "buy", TargetAmount: 100_000},
			{StockCode: "sz000002", Side: "buy", TargetAmount: 100_000},
		},
	})
	require.Equal(t, pretrade.LevelBlocked, view.Level)
	require.False(t, view.Allowed)
	require.Equal(t, pretrade.StatusBlocked, view.CashCheck.Status)
	require.InDelta(t, 200_000, view.CashCheck.RequiredCash, 1e-6)
	require.Contains(t, view.CashCheck.Reason, "需要")
	require.Contains(t, view.Summary, "现金")
}

func TestBuild_ExistingPosition_ADD(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		PlanID:  3,
		Account: pretrade.AccountInput{Cash: 500_000, Equity: 2_000_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", StockName: "平安", Side: "buy", TargetAmount: 100_000},
		},
		Positions: []pretrade.PositionInput{
			{StockCode: "sz000001", StockName: "平安", TotalVolume: 500},
		},
	})
	require.Equal(t, pretrade.LevelWarning, view.Level)
	require.True(t, view.Allowed)
	require.True(t, view.RequiresConfirm)
	require.Equal(t, pretrade.StatusWarning, view.ExistingPositionCheck.Status)
	require.Equal(t, 1, view.ExistingPositionCheck.AddCount)
	require.Equal(t, pretrade.ActionAdd, view.PositionActions[0].Action)
	require.Equal(t, int64(500), view.PositionActions[0].ExistingVolume)
}

func TestBuild_HighConcentration(t *testing.T) {
	// 400k / 2M equity = 20% → HIGH
	view := pretrade.Build(pretrade.Inputs{
		PlanID:  4,
		Account: pretrade.AccountInput{Cash: 500_000, Equity: 2_000_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", Side: "buy", TargetAmount: 400_000},
		},
	})
	require.Equal(t, pretrade.ConcentrationHigh, view.ConcentrationCheck.Level)
	require.Equal(t, pretrade.StatusWarning, view.ConcentrationCheck.Status)
	require.Equal(t, pretrade.LevelWarning, view.Level)
	require.True(t, view.RequiresConfirm)
}

func TestBuild_NoPositions_HOLDEmpty(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		PlanID:    5,
		Account:   pretrade.AccountInput{Cash: 1_000_000, Equity: 1_000_000},
		Lines:     []pretrade.PlanLine{{StockCode: "sz000001", Side: "buy", TargetAmount: 50_000}},
		Positions: nil,
	})
	require.Equal(t, pretrade.LevelPass, view.Level)
	require.Equal(t, 0, view.ExistingPositionCheck.AddCount)
	require.Equal(t, 1, view.ExistingPositionCheck.NewCount)
	require.Len(t, view.PositionActions, 1)
	require.Equal(t, pretrade.ActionNew, view.PositionActions[0].Action)
}

func TestBuild_HoldUntouchedPosition(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		PlanID:  6,
		Account: pretrade.AccountInput{Cash: 500_000, Equity: 2_000_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", Side: "buy", TargetAmount: 80_000},
		},
		Positions: []pretrade.PositionInput{
			{StockCode: "sz000002", StockName: "万科", TotalVolume: 1000},
		},
	})
	var hold, neu int
	for _, a := range view.PositionActions {
		switch a.Action {
		case pretrade.ActionHold:
			hold++
			require.Equal(t, "sz000002", a.StockCode)
		case pretrade.ActionNew:
			neu++
		}
	}
	require.Equal(t, 1, hold)
	require.Equal(t, 1, neu)
}

func TestBuild_SkipsGapSkipIntent(t *testing.T) {
	view := pretrade.Build(pretrade.Inputs{
		Account: pretrade.AccountInput{Cash: 10_000, Equity: 100_000},
		Lines: []pretrade.PlanLine{
			{StockCode: "sz000001", Side: "buy", TargetAmount: 100_000, IntentStatus: "gap_skip"},
		},
	})
	require.Equal(t, 0.0, view.CashCheck.RequiredCash)
	require.Equal(t, pretrade.StatusPass, view.CashCheck.Status)
}
