package papertrading_test

import (
	"errors"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func TestQuoteOverlay_UsesLiveQuoteNotMarkPrice(t *testing.T) {
	asOf := evalAsOf(t)
	fetched := time.Date(2026, 8, 11, 14, 32, 5, 0, time.Local)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		AccountID:           1,
		AsOf:                asOf,
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:    "sz000001",
			StockName:    "平安银行",
			TotalVolume:  1000,
			CurrentPrice: 10.05, // persisted fill/mark
			Lots: []papertrading.PositionLotDTO{{
				PlanID: 1, PlanItemID: 1, OrderID: 1, FillID: 1,
				FillPrice: 10.05, Volume: 1000, CostAmount: 10.05 * 1000,
				TradeDate: "2026-08-11",
			}},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 1000, AttributedVolume: 1000, Status: papertrading.ReconcileStatusMatched,
			},
		}},
	}
	stub := &stubObsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.20, Open: 10.50, FetchedAt: fetched,
	}}}
	provider := papertrading.NewQuoteOverlayPriceProvider(attr, stub)
	view := papertrading.ProjectHoldingEvaluation(attr, provider, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.Len(t, view.Holdings, 1)
	h := view.Holdings[0]
	require.NotNil(t, h.CurrentPrice)
	require.InDelta(t, 11.20, *h.CurrentPrice, 1e-9)
	require.NotEqual(t, 10.05, *h.CurrentPrice)
	require.Equal(t, papertrading.QuoteSourceTencent, h.PriceSource)
	require.NotNil(t, h.UnrealizedPnL)
	require.InDelta(t, (11.20-10.05)*1000, *h.UnrealizedPnL, 1e-6)
	require.NotNil(t, h.QuoteTime)
	require.True(t, h.QuoteTime.Equal(fetched))
}

func TestQuoteOverlay_FallbackMarkWhenQuoteFails(t *testing.T) {
	asOf := evalAsOf(t)
	attr := &papertrading.PositionAttributionView{
		Enabled:             true,
		ReconcileAllMatched: true,
		Positions: []papertrading.PositionAttributionRow{{
			StockCode:    "sz000001",
			TotalVolume:  1000,
			CurrentPrice: 10.05,
			Lots: []papertrading.PositionLotDTO{{
				FillID: 1, FillPrice: 10.05, Volume: 1000, CostAmount: 10.05 * 1000,
				TradeDate: "2026-08-11",
			}},
			Reconcile: papertrading.AttributionReconcile{
				PositionVolume: 1000, AttributedVolume: 1000, Status: papertrading.ReconcileStatusMatched,
			},
		}},
	}
	stub := &stubObsQuoteService{err: errors.New("upstream down")}
	provider := papertrading.NewQuoteOverlayPriceProvider(attr, stub)
	view := papertrading.ProjectHoldingEvaluation(attr, provider, papertrading.HoldingEvaluationOptions{AsOf: asOf})
	require.Len(t, view.Holdings, 1)
	h := view.Holdings[0]
	require.NotNil(t, h.CurrentPrice)
	require.InDelta(t, 10.05, *h.CurrentPrice, 1e-9)
	require.Equal(t, papertrading.QuoteSourcePositionMark, h.PriceSource)
	require.NotNil(t, h.UnrealizedPnL)
	require.InDelta(t, 0, *h.UnrealizedPnL, 1e-9)
}

func TestBuildHoldingEvaluationObservation_QuotePnLAndNoMarkWrite(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	plan, item := seedAttrPlanItem(t, "2026-08-11", "sz000001", "平安银行", "s")
	_, fill := seedAttrOrderFill(t, acc.ID, plan, item, 10.05, 1000)
	require.NotZero(t, fill.ID)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 0, LockedVolume: 1000,
		AvgCost: 10.05, MarkPrice: 10.05,
	}).Error)

	fetched := time.Date(2026, 8, 11, 14, 32, 5, 0, time.Local)
	stub := &stubObsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.20, FetchedAt: fetched,
	}}}
	papertrading.SetHoldingEvalQuoteServiceForTest(stub)
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	view, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	require.NoError(t, err)
	require.Len(t, view.Holdings, 1)
	row := view.Holdings[0]
	require.NotNil(t, row.CurrentPrice)
	require.InDelta(t, 11.20, *row.CurrentPrice, 1e-9)
	require.NotNil(t, row.MarketPrice)
	require.InDelta(t, 11.20, *row.MarketPrice, 1e-9)
	require.Equal(t, papertrading.QuoteSourceTencent, row.QuoteSource)
	require.NotNil(t, row.QuoteTime)
	require.True(t, row.QuoteTime.Equal(fetched))
	require.NotNil(t, row.UnrealizedPnL)
	require.InDelta(t, (11.20-10.05)*1000, *row.UnrealizedPnL, 1e-6)
	require.NotNil(t, row.UnrealizedReturn)
	require.InDelta(t, (11.20-10.05)/10.05, *row.UnrealizedReturn, 1e-9)
	require.NotNil(t, row.MarketValue)
	require.InDelta(t, 11.20*1000, *row.MarketValue, 1e-6)
	require.Len(t, row.Lots, 1)
	require.Equal(t, papertrading.QuoteSourceTencent, row.Lots[0].QuoteSource)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.05, pos.MarkPrice, 1e-9)
}

func TestBuildHoldingEvaluationObservation_QuoteFailFallbackNoMarkWrite(t *testing.T) {
	setupTestDB(t)
	enablePaperTrading(t)
	acc := seedAttrAccount(t)
	plan, item := seedAttrPlanItem(t, "2026-08-11", "sz000001", "平安银行", "s")
	seedAttrOrderFill(t, acc.ID, plan, item, 10.05, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.05, MarkPrice: 10.05,
	}).Error)

	papertrading.SetHoldingEvalQuoteServiceForTest(&stubObsQuoteService{err: errors.New("down")})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	view, err := papertrading.BuildHoldingEvaluationObservation(papertrading.HoldingEvaluationBuildOptions{})
	require.NoError(t, err)
	require.Len(t, view.Holdings, 1)
	row := view.Holdings[0]
	require.NotNil(t, row.CurrentPrice)
	require.InDelta(t, 10.05, *row.CurrentPrice, 1e-9)
	require.Equal(t, papertrading.QuoteSourcePositionMark, row.QuoteSource)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.05, pos.MarkPrice, 1e-9)
}
