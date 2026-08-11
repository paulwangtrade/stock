package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/models"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func seedEvalObsAccount(t *testing.T) *papertrading.PaperSimAccount {
	t.Helper()
	require.NoError(t, data.EnsureTradePlanTables())
	require.NoError(t, papertrading.EnsureSchema(db.Dao))
	papertrading.SetConfigForTest(papertrading.Config{EnablePaperTrading: true, InitialCash: 1_000_000})
	t.Cleanup(papertrading.ResetConfigCache)
	acc := &papertrading.PaperSimAccount{Name: "paper_sim_default", InitialCash: 1_000_000, Cash: 1_000_000}
	require.NoError(t, db.Dao.Create(acc).Error)
	return acc
}

func seedEvalObsPlanItem(t *testing.T, tradeDate, code, name string) (*models.TradePlan, *models.TradePlanItem) {
	t.Helper()
	now := time.Now()
	plan := &models.TradePlan{
		TradeDate: tradeDate, GeneratedAt: now, Status: models.TradePlanStatusReady, PlanVersion: 1,
	}
	require.NoError(t, db.Dao.Create(plan).Error)
	item := &models.TradePlanItem{
		PlanID: plan.ID, TradeDate: tradeDate, StockCode: code, StockName: name,
		Side: "buy", Status: models.TradePlanItemPending,
	}
	require.NoError(t, db.Dao.Create(item).Error)
	return plan, item
}

func seedEvalObsFill(
	t *testing.T,
	accID uint,
	plan *models.TradePlan,
	item *models.TradePlanItem,
	price float64,
	vol int64,
) *papertrading.PaperSimFill {
	t.Helper()
	now := time.Now()
	order := &papertrading.PaperSimOrder{
		AccountID: accID, PlanID: plan.ID, PlanItemID: item.ID, TradeDate: plan.TradeDate,
		StockCode: item.StockCode, StockName: item.StockName, Side: "buy",
		Quantity: vol, OrderPrice: price, Status: papertrading.OrderStatusFilled,
		FilledPrice: price, FilledVolume: vol, OrderTime: now,
	}
	require.NoError(t, db.Dao.Create(order).Error)
	fill := &papertrading.PaperSimFill{
		AccountID: accID, OrderID: order.ID, PlanID: plan.ID, PlanItemID: item.ID,
		StockCode: item.StockCode, StockName: item.StockName, Side: "buy",
		Price: price, Volume: vol, FilledAt: now,
	}
	require.NoError(t, db.Dao.Create(fill).Error)
	return fill
}

func getHoldingsEvaluation(t *testing.T) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/papertrading/observation/holdings/evaluation", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	eval, ok := body["evaluation"].(map[string]any)
	require.True(t, ok)
	return eval
}

func TestHoldingsEvaluationAPI_SingleStockSingleLot(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-05", "sh600363", "联创光电")
	fill := seedEvalObsFill(t, acc.ID, plan, item, 10.5, 3000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 3000, AvgCost: 10.5, MarkPrice: 11.0,
	}).Error)

	eval := getHoldingsEvaluation(t)
	require.True(t, eval["reconcile_all_matched"].(bool))
	holdings := eval["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	require.Equal(t, "sh600363", row["stock_code"])
	require.Equal(t, "联创光电", row["stock_name"])
	require.Equal(t, float64(3000), row["total_volume"])
	require.InDelta(t, 10.5, row["avg_cost"].(float64), 1e-9)
	require.InDelta(t, 11.0, row["market_price"].(float64), 1e-9)
	require.InDelta(t, 11.0*3000, row["market_value"].(float64), 1e-6)
	require.InDelta(t, (11.0-10.5)*3000, row["unrealized_pnl"].(float64), 1e-6)
	require.InDelta(t, (11.0-10.5)/10.5, row["unrealized_return"].(float64), 1e-9)
	require.Equal(t, "NORMAL", row["eval_state"])
	lots := row["lots"].([]any)
	require.Len(t, lots, 1)
	lot := lots[0].(map[string]any)
	require.Equal(t, float64(fill.ID), lot["fill_id"])
	require.Equal(t, float64(plan.ID), lot["plan_id"])
	require.Equal(t, float64(item.ID), lot["plan_item_id"])
	require.Equal(t, "2026-08-05", lot["buy_date"])
	require.InDelta(t, 10.5, lot["cost_price"].(float64), 1e-9)
	require.InDelta(t, (11.0-10.5)*3000, lot["pnl"].(float64), 1e-6)
	require.InDelta(t, (11.0-10.5)/10.5, lot["return_rate"].(float64), 1e-9)
}

func TestHoldingsEvaluationAPI_MultiLotSameStock(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	p32, i32 := seedEvalObsPlanItem(t, "2026-08-01", "sh600363", "联创光电")
	p35, i35 := seedEvalObsPlanItem(t, "2026-08-03", "sh600363", "联创光电")
	f32 := seedEvalObsFill(t, acc.ID, p32, i32, 10.0, 3000)
	f35 := seedEvalObsFill(t, acc.ID, p35, i35, 12.0, 2000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600363", StockName: "联创光电",
		TotalVolume: 5000, AvgCost: 10.8, MarkPrice: 11.5,
	}).Error)

	eval := getHoldingsEvaluation(t)
	holdings := eval["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	require.Equal(t, float64(5000), row["total_volume"])
	lots := row["lots"].([]any)
	require.Len(t, lots, 2)
	fillIDs := map[float64]bool{}
	for _, raw := range lots {
		lot := raw.(map[string]any)
		fillIDs[lot["fill_id"].(float64)] = true
		require.NotZero(t, lot["fill_id"])
		require.NotZero(t, lot["plan_id"])
	}
	require.True(t, fillIDs[float64(f32.ID)])
	require.True(t, fillIDs[float64(f35.ID)])
	require.InDelta(t, (11.5-10.0)*3000+(11.5-12.0)*2000, row["unrealized_pnl"].(float64), 1e-6)
	require.Equal(t, "NORMAL", row["eval_state"])
}

func TestHoldingsEvaluationAPI_MissingPrice(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-05", "sz000001", "平安银行")
	fill := seedEvalObsFill(t, acc.ID, plan, item, 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.0, MarkPrice: 0, // non-positive → missing mark
	}).Error)

	eval := getHoldingsEvaluation(t)
	holdings := eval["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	require.Equal(t, float64(fill.ID), row["lots"].([]any)[0].(map[string]any)["fill_id"])
	require.Nil(t, row["market_price"])
	require.Nil(t, row["market_value"])
	require.Nil(t, row["unrealized_pnl"])
	require.Nil(t, row["unrealized_return"])
	lot := row["lots"].([]any)[0].(map[string]any)
	require.Nil(t, lot["current_price"])
	require.Nil(t, lot["pnl"])
	require.Nil(t, lot["return_rate"])
	require.Equal(t, "NORMAL", row["eval_state"])
}

func TestHoldingsEvaluationAPI_Unattributed(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-05", "sz000001", "平安银行")
	fill := seedEvalObsFill(t, acc.ID, plan, item, 10.0, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1500, AvgCost: 10, MarkPrice: 10,
	}).Error)

	eval := getHoldingsEvaluation(t)
	require.False(t, eval["reconcile_all_matched"].(bool))
	holdings := eval["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	require.Equal(t, float64(1000), row["total_volume"], "attributed lots only")
	lots := row["lots"].([]any)
	require.Len(t, lots, 1)
	require.Equal(t, float64(fill.ID), lots[0].(map[string]any)["fill_id"])
	unattr := eval["unattributable"].([]any)
	require.Len(t, unattr, 1)
	u := unattr[0].(map[string]any)
	require.Equal(t, float64(500), u["volume"])
	require.Equal(t, "POSITION_GT_FILLS", u["reason_code"])
}

func TestHoldingsEvaluationAPI_VolumeReconcileFailure(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	// surplus fills: position 500, fills 1000
	plan, item := seedEvalObsPlanItem(t, "2026-08-05", "sh600519", "贵州茅台")
	seedEvalObsFill(t, acc.ID, plan, item, 1800, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sh600519", StockName: "贵州茅台",
		TotalVolume: 500, AvgCost: 1800, MarkPrice: 1800,
	}).Error)

	eval := getHoldingsEvaluation(t)
	require.False(t, eval["reconcile_all_matched"].(bool))
	holdings := eval["holdings"].([]any)
	require.Len(t, holdings, 1)
	row := holdings[0].(map[string]any)
	require.Equal(t, "surplus_fills", row["reconcile_status"])
	lots := row["lots"].([]any)
	require.Len(t, lots, 1)
	require.NotZero(t, lots[0].(map[string]any)["fill_id"], "must not forge; real fill retained")
	require.Equal(t, float64(1000), row["total_volume"])
}

func TestHoldingsEvaluationAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/holdings/evaluation", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

type stubHoldingsQuoteService struct {
	quotes []marketdata.Quote
	err    error
}

func (s *stubHoldingsQuoteService) GetQuote(code string) (*marketdata.Quote, error) {
	qs, err := s.GetQuotes([]string{code})
	if err != nil {
		return nil, err
	}
	if len(qs) == 0 {
		return nil, marketdata.ErrNoData
	}
	out := qs[0]
	return &out, nil
}

func (s *stubHoldingsQuoteService) GetQuotes(codes []string) ([]marketdata.Quote, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.quotes, nil
}

func TestHoldingsEvaluationAPI_QuoteOverlayPnL(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.05, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvgCost: 10.05, MarkPrice: 10.05,
	}).Error)

	fetched := time.Date(2026, 8, 11, 14, 32, 5, 0, time.Local)
	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.20, FetchedAt: fetched,
	}}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	eval := getHoldingsEvaluation(t)
	row := eval["holdings"].([]any)[0].(map[string]any)
	require.InDelta(t, 11.20, row["current_price"].(float64), 1e-9)
	require.InDelta(t, 11.20, row["market_price"].(float64), 1e-9)
	require.Equal(t, "tencent", row["quote_source"])
	require.InDelta(t, (11.20-10.05)*1000, row["unrealized_pnl"].(float64), 1e-6)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.05, pos.MarkPrice, 1e-9)
}
