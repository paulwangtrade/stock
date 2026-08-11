package papertrading_test

import (
	"testing"
	"time"

	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func evalAsOf(t *testing.T) time.Time {
	t.Helper()
	loc := time.Local
	return time.Date(2026, 8, 8, 12, 0, 0, 0, loc)
}

func TestHoldingEvaluation_SingleLotNormal(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		AccountID:           1,
		AsOf:                asOf,
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:    "sh600363",
			StockName:    "联创光电",
			TotalVolume:  3000,
			CurrentPrice: 11.0,
			Lots: []papertrading.PositionLotDTO{{
				PlanID: 5, PlanItemID: 12, OrderID: 20, FillID: 34,
				FillPrice: 10.5, Volume: 3000, CostAmount: 10.5 * 3000,
				TradeDate: "2026-08-05",
			}},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 3000, AttributedVolume: 3000, Status: papertrading.ReconcileStatusMatched,
			},
		}},
	}
	prices := papertrading.MapPriceProvider{"sh600363": 11.0}

	view := papertrading.ProjectHoldingEvaluation(attr, prices, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.True(t, view.Enabled)
	require.True(t, view.ReconcileAllMatched)
	require.Len(t, view.Holdings, 1)
	require.Empty(t, view.Unattributable)

	h := view.Holdings[0]
	require.Equal(t, uint(34), h.LotID)
	require.Equal(t, uint(5), h.PlanID)
	require.Equal(t, uint(12), h.PlanItemID)
	require.Equal(t, "sh600363", h.StockCode)
	require.Equal(t, "联创光电", h.StockNameSnapshot)
	require.Equal(t, int64(3000), h.Volume)
	require.InDelta(t, 10.5, h.CostPrice, 1e-9)
	require.Equal(t, "2026-08-05", h.BuyDate)
	require.Equal(t, 3, h.HoldingDays) // Aug 5 → Aug 8
	require.NotNil(t, h.CurrentPrice)
	require.InDelta(t, 11.0, *h.CurrentPrice, 1e-9)
	require.NotNil(t, h.MarketValue)
	require.InDelta(t, 11.0*3000, *h.MarketValue, 1e-6)
	require.NotNil(t, h.UnrealizedPnL)
	require.InDelta(t, (11.0-10.5)*3000, *h.UnrealizedPnL, 1e-6)
	require.NotNil(t, h.UnrealizedReturnPct)
	require.InDelta(t, (11.0-10.5)/10.5*100, *h.UnrealizedReturnPct, 1e-6)
	require.Equal(t, papertrading.EvalStateNormal, h.EvalState)
	require.Equal(t, 1, view.Summary.LotCount)
	require.Equal(t, 1, view.Summary.ByEvalState[papertrading.EvalStateNormal])
}

func TestHoldingEvaluation_MultiLotSameStock(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		AccountID:           1,
		AsOf:                asOf,
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:    "sh600363",
			StockName:    "联创光电",
			TotalVolume:  5000,
			CurrentPrice: 11.5,
			Lots: []papertrading.PositionLotDTO{
				{PlanID: 32, PlanItemID: 1, FillID: 100, FillPrice: 10.0, Volume: 3000, CostAmount: 30000, TradeDate: "2026-08-01"},
				{PlanID: 35, PlanItemID: 2, FillID: 101, FillPrice: 12.0, Volume: 2000, CostAmount: 24000, TradeDate: "2026-08-03"},
			},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 5000, AttributedVolume: 5000, Status: papertrading.ReconcileStatusMatched,
			},
			PlanIDs: []uint{32, 35},
		}},
	}
	prices := papertrading.MapPriceProvider{"sh600363": 11.5}

	view := papertrading.ProjectHoldingEvaluation(attr, prices, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.Len(t, view.Holdings, 2)
	require.True(t, view.ReconcileAllMatched)

	byPlan := map[uint]papertrading.HoldingLotEvaluation{}
	for _, h := range view.Holdings {
		byPlan[h.PlanID] = h
		require.Equal(t, papertrading.EvalStateNormal, h.EvalState)
		require.Equal(t, "sh600363", h.StockCode)
		require.NotNil(t, h.CurrentPrice)
		require.InDelta(t, 11.5, *h.CurrentPrice, 1e-9)
	}
	require.Equal(t, int64(3000), byPlan[32].Volume)
	require.Equal(t, uint(100), byPlan[32].LotID)
	require.InDelta(t, (11.5-10.0)*3000, *byPlan[32].UnrealizedPnL, 1e-6)
	require.Equal(t, 7, byPlan[32].HoldingDays) // Aug 1 → Aug 8

	require.Equal(t, int64(2000), byPlan[35].Volume)
	require.Equal(t, uint(101), byPlan[35].LotID)
	require.InDelta(t, (11.5-12.0)*2000, *byPlan[35].UnrealizedPnL, 1e-6)
	require.Equal(t, 5, byPlan[35].HoldingDays)

	require.Equal(t, int64(5000), view.Summary.TotalVolume)
	require.NotNil(t, view.Summary.TotalUnrealizedPnL)
	require.InDelta(t, (11.5-10.0)*3000+(11.5-12.0)*2000, *view.Summary.TotalUnrealizedPnL, 1e-6)
}

func TestHoldingEvaluation_MissingPrice(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:   "sz000001",
			StockName:   "平安银行",
			TotalVolume: 1000,
			Lots: []papertrading.PositionLotDTO{{
				PlanID: 1, PlanItemID: 1, FillID: 7, FillPrice: 10.0, Volume: 1000, TradeDate: "2026-08-01",
			}},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched, PositionVolume: 1000, AttributedVolume: 1000},
		}},
	}
	// empty provider → missing mark
	view := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.Len(t, view.Holdings, 1)
	h := view.Holdings[0]
	require.Equal(t, uint(7), h.LotID)
	require.Nil(t, h.CurrentPrice)
	require.Nil(t, h.MarketValue)
	require.Nil(t, h.UnrealizedPnL)
	require.Nil(t, h.UnrealizedReturnPct)
	require.Equal(t, "missing", h.PriceQuality)
	require.Equal(t, papertrading.EvalStateNormal, h.EvalState, "missing price must not invent exit state")
	require.Nil(t, view.Summary.TotalMarketValue)
	require.Nil(t, view.Summary.TotalUnrealizedPnL)
}

func TestHoldingEvaluation_ReconcileFailure(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		AccountID:           1,
		AsOf:                asOf,
		ReconcileAllMatched: false,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:    "sz000001",
			StockName:    "平安银行",
			TotalVolume:  1500,
			CurrentPrice: 10.0,
			Lots: []papertrading.PositionLotDTO{{
				PlanID: 9, PlanItemID: 3, FillID: 55, FillPrice: 10.0, Volume: 1000, TradeDate: "2026-08-05",
			}},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 1500, AttributedVolume: 1000, UnattributedVolume: 500,
				Status: papertrading.ReconcileStatusUnattributed,
			},
			Unattributed: &papertrading.UnattributedQty{
				Volume: 500, ReasonCode: "POSITION_GT_FILLS", Message: "position_volume=1500 exceeds attributed buy fills=1000",
			},
		}},
	}
	prices := papertrading.MapPriceProvider{"sz000001": 10.0}

	view := papertrading.ProjectHoldingEvaluation(attr, prices, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.False(t, view.ReconcileAllMatched)
	require.Len(t, view.Holdings, 1, "only attributed fill lots")
	require.Equal(t, uint(55), view.Holdings[0].LotID)
	require.NotEqual(t, uint(0), view.Holdings[0].LotID)
	require.Equal(t, int64(1000), view.Holdings[0].Volume)

	require.Len(t, view.Unattributable, 1)
	require.Equal(t, int64(500), view.Unattributable[0].Volume)
	require.Equal(t, "POSITION_GT_FILLS", view.Unattributable[0].ReasonCode)
	require.Equal(t, 1, view.Summary.UnattributableCount)
	require.Equal(t, papertrading.EvalStateNormal, view.Holdings[0].EvalState)
}

func TestHoldingEvaluation_NameSnapshotOverride(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "sh600601", StockName: "position-name",
			Lots: []papertrading.PositionLotDTO{{
				FillID: 1, PlanID: 1, PlanItemID: 1, FillPrice: 1, Volume: 100, TradeDate: "2026-08-01",
			}},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched},
		}},
	}
	view := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"sh600601": 1}, papertrading.HoldingEvaluationOptions{
		AsOf:         asOf,
		NameByFillID: map[uint]string{1: "方正科技"},
	})
	require.Equal(t, "方正科技", view.Holdings[0].StockNameSnapshot)
}

func TestCalendarHoldingDays_viaProjection(t *testing.T) {
	asOf := time.Date(2026, 8, 8, 0, 0, 0, 0, time.Local)
	attr := &papertrading.PositionAttributionView{
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "x",
			Lots: []papertrading.PositionLotDTO{
				{FillID: 1, FillPrice: 1, Volume: 1, TradeDate: "2026-08-08"}, // 0 days
				{FillID: 2, FillPrice: 1, Volume: 1, TradeDate: "2026-08-09"}, // future → 0
				{FillID: 3, FillPrice: 1, Volume: 1, TradeDate: ""},            // missing → 0
			},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched},
		}},
	}
	view := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"x": 1}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	byID := map[uint]int{}
	for _, h := range view.Holdings {
		byID[h.LotID] = h.HoldingDays
	}
	require.Equal(t, 0, byID[1])
	require.Equal(t, 0, byID[2])
	require.Equal(t, 0, byID[3])
}

func TestClassifyRiskState(t *testing.T) {
	th := papertrading.DefaultRiskStateThresholds
	require.Equal(t, papertrading.RiskStateNormal, papertrading.ClassifyRiskState(nil, th))
	r := 0.05
	require.Equal(t, papertrading.RiskStateNormal, papertrading.ClassifyRiskState(&r, th))
	r = -0.05
	require.Equal(t, papertrading.RiskStateNormal, papertrading.ClassifyRiskState(&r, th))
	r = -0.0500001
	require.Equal(t, papertrading.RiskStateWatch, papertrading.ClassifyRiskState(&r, th))
	r = -0.10
	require.Equal(t, papertrading.RiskStateWatch, papertrading.ClassifyRiskState(&r, th))
	r = -0.1000001
	require.Equal(t, papertrading.RiskStateDanger, papertrading.ClassifyRiskState(&r, th))
}

func TestD14_SingleLotProfit(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled: true, AsOf: asOf, ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "sh600363", StockName: "联创光电", TotalVolume: 3000, CurrentPrice: 11.0,
			Lots: []papertrading.PositionLotDTO{{
				FillID: 1, PlanID: 1, PlanItemID: 1, FillPrice: 10.0, Volume: 3000, TradeDate: "2026-08-01",
			}},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched, PositionVolume: 3000, AttributedVolume: 3000},
		}},
	}
	flat := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"sh600363": 11.0}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	obs := papertrading.AggregateHoldingEvaluationObservation(flat, attr)
	require.Len(t, obs.Holdings, 1)
	row := obs.Holdings[0]
	require.Equal(t, 7, row.HoldingDays) // Aug1→Aug8
	require.Equal(t, "2026-08-01", row.FirstBuyDate)
	require.Equal(t, papertrading.TrendStateUnknown, row.TrendState)
	require.Equal(t, papertrading.RiskStateNormal, row.RiskState)
	require.Equal(t, 7, row.Lots[0].HoldingDays)
	require.NotNil(t, row.UnrealizedReturn)
	require.Greater(t, *row.UnrealizedReturn, 0.0)
}

func TestD14_SingleLotLoss_WatchAndDanger(t *testing.T) {
	asOf := evalAsOf(t)
	mk := func(mark float64) *papertrading.HoldingEvalStockRow {
		attr := &papertrading.PositionAttributionView{
			AsOf: asOf, ReconcileAllMatched: true,
			Positions: []papertrading.PositionAttributionRow{{
				StockCode: "sz000001", StockName: "平安银行", TotalVolume: 1000, CurrentPrice: mark,
				Lots: []papertrading.PositionLotDTO{{
					FillID: 1, PlanID: 1, PlanItemID: 1, FillPrice: 10.0, Volume: 1000, TradeDate: "2026-08-01",
				}},
				Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched},
			}},
		}
		flat := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"sz000001": mark}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
		obs := papertrading.AggregateHoldingEvaluationObservation(flat, attr)
		require.Len(t, obs.Holdings, 1)
		return &obs.Holdings[0]
	}
	// -6% → WATCH
	w := mk(9.4)
	require.InDelta(t, -0.06, *w.UnrealizedReturn, 1e-9)
	require.Equal(t, papertrading.RiskStateWatch, w.RiskState)
	require.Equal(t, papertrading.TrendStateUnknown, w.TrendState)
	// -12% → DANGER
	d := mk(8.8)
	require.InDelta(t, -0.12, *d.UnrealizedReturn, 1e-9)
	require.Equal(t, papertrading.RiskStateDanger, d.RiskState)
}

func TestD14_MultiLotAggregateHoldingDays(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		AsOf: asOf, ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "sh600363", StockName: "联创光电", TotalVolume: 5000, CurrentPrice: 11.0,
			Lots: []papertrading.PositionLotDTO{
				{FillID: 10, PlanID: 32, PlanItemID: 1, FillPrice: 10.0, Volume: 3000, TradeDate: "2026-08-01"},
				{FillID: 11, PlanID: 35, PlanItemID: 2, FillPrice: 12.0, Volume: 2000, TradeDate: "2026-08-05"},
			},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched},
		}},
	}
	flat := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"sh600363": 11.0}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	obs := papertrading.AggregateHoldingEvaluationObservation(flat, attr)
	require.Len(t, obs.Holdings, 1)
	row := obs.Holdings[0]
	require.Equal(t, "2026-08-01", row.FirstBuyDate)
	require.Equal(t, 7, row.HoldingDays, "stock holding_days from earliest lot")
	require.Len(t, row.Lots, 2)
	byFill := map[uint]int{}
	for _, lot := range row.Lots {
		byFill[lot.FillID] = lot.HoldingDays
	}
	require.Equal(t, 7, byFill[10])
	require.Equal(t, 3, byFill[11])
	require.Equal(t, papertrading.TrendStateUnknown, row.TrendState)
}

func TestD14_MissingPrice(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		AsOf: asOf, ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "sz000001", StockName: "平安银行", TotalVolume: 1000, CurrentPrice: 0,
			Lots: []papertrading.PositionLotDTO{{
				FillID: 7, PlanID: 1, PlanItemID: 1, FillPrice: 10.0, Volume: 1000, TradeDate: "2026-08-01",
			}},
			Reconcile: papertrading.AttributionReconcile{Status: papertrading.ReconcileStatusMatched},
		}},
	}
	flat := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	obs := papertrading.AggregateHoldingEvaluationObservation(flat, attr)
	require.Len(t, obs.Holdings, 1)
	row := obs.Holdings[0]
	require.Nil(t, row.MarketPrice)
	require.Nil(t, row.UnrealizedReturn)
	require.Equal(t, papertrading.RiskStateNormal, row.RiskState, "missing return must not invent DANGER")
	require.Equal(t, papertrading.TrendStateUnknown, row.TrendState)
	require.Equal(t, 7, row.HoldingDays)
}

func TestD14_Unattributed(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		AsOf: asOf, ReconcileAllMatched: false,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode: "sz000001", StockName: "平安银行", TotalVolume: 1500, CurrentPrice: 10.0,
			Lots: []papertrading.PositionLotDTO{{
				FillID: 55, PlanID: 9, PlanItemID: 3, FillPrice: 10.0, Volume: 1000, TradeDate: "2026-08-05",
			}},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 1500, AttributedVolume: 1000, UnattributedVolume: 500,
				Status: papertrading.ReconcileStatusUnattributed,
			},
			Unattributed: &papertrading.UnattributedQty{Volume: 500, ReasonCode: "POSITION_GT_FILLS"},
		}},
	}
	flat := papertrading.ProjectHoldingEvaluation(attr, papertrading.MapPriceProvider{"sz000001": 10.0}, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	obs := papertrading.AggregateHoldingEvaluationObservation(flat, attr)
	require.False(t, obs.ReconcileAllMatched)
	require.Len(t, obs.Holdings, 1)
	require.Equal(t, int64(1000), obs.Holdings[0].TotalVolume)
	require.Len(t, obs.Holdings[0].Lots, 1)
	require.Equal(t, uint(55), obs.Holdings[0].Lots[0].FillID)
	require.Len(t, obs.Unattributable, 1)
	require.Equal(t, int64(500), obs.Unattributable[0].Volume)
	require.Equal(t, papertrading.TrendStateUnknown, obs.Holdings[0].TrendState)
}
