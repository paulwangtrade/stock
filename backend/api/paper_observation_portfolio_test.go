package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/marketdata"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func getPortfolioObs(t *testing.T, query string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	path := "/api/papertrading/observation/portfolio"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	require.Contains(t, body["disclaimer"].(string), "不是交易建议")
	p, ok := body["portfolio"].(map[string]any)
	require.True(t, ok)
	return p
}

func TestPortfolioObservationAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/portfolio", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPortfolioObservationAPI_EmptyBook(t *testing.T) {
	setupAPITestDB(t)
	_ = seedEvalObsAccount(t)
	p := getPortfolioObs(t, "target=identity")
	acct := p["account"].(map[string]any)
	require.InDelta(t, 1_000_000.0, acct["cash"].(float64), 1e-6)
	dec := p["decision"].(map[string]any)
	require.Equal(t, float64(0), dec["normal_count"])
	require.Equal(t, float64(0), dec["watch_count"])
	require.Equal(t, float64(0), dec["review_count"])
	reb := p["rebalance"].(map[string]any)
	require.Equal(t, float64(0), reb["keep_count"])
	require.Equal(t, "none", p["action"])
	pos, _ := p["positions"].([]any)
	require.Empty(t, pos)
}

func TestPortfolioObservationAPI_NormalBookNoMutation(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	recentDate := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	plan, item := seedEvalObsPlanItem(t, recentDate, "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)
	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.00, FetchedAt: time.Now(),
	}}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	p := getPortfolioObs(t, "target=identity")
	dec := p["decision"].(map[string]any)
	require.Equal(t, float64(1), dec["normal_count"])
	reb := p["rebalance"].(map[string]any)
	require.Equal(t, float64(1), reb["keep_count"])
	require.True(t, reb["available"].(bool))
	rows := p["positions"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.Equal(t, "sz000001", row["symbol"])
	require.Equal(t, "HOLD_NORMAL", row["decision_state"])
	require.Equal(t, "KEEP", row["rebalance_action"])
	require.Equal(t, "none", row["action"])
	require.Equal(t, "HEALTHY", row["health_level"])
	require.Equal(t, "SHORT", row["holding_period_bucket"])
	require.False(t, row["is_aging"].(bool))
	health := p["health"].(map[string]any)
	require.Equal(t, "HEALTHY", health["portfolio_health"])
	require.Equal(t, float64(1), health["healthy_positions"])
	oc := p["opportunity_cost"].(map[string]any)
	require.False(t, oc["available"].(bool))

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.00, pos.MarkPrice, 1e-9)
	require.Equal(t, int64(1000), pos.TotalVolume)
	var acc2 papertrading.PaperSimAccount
	require.NoError(t, db.Dao.First(&acc2, acc.ID).Error)
	require.InDelta(t, 1_000_000.0, acc2.Cash, 1e-6)
	require.InDelta(t, acc.Equity, acc2.Equity, 1e-6)
	raw, _ := json.Marshal(p)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)
}

func TestPortfolioObservationAPI_WatchReviewAndRebalance(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)

	p1, i1 := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, p1, i1, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	p2, i2 := seedEvalObsPlanItem(t, "2026-08-11", "sz000002", "万科A")
	seedEvalObsFill(t, acc.ID, p2, i2, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000002", StockName: "万科A",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	p3, i3 := seedEvalObsPlanItem(t, "2026-08-11", "sz000003", "金证股份")
	seedEvalObsFill(t, acc.ID, p3, i3, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000003", StockName: "金证股份",
		TotalVolume: 1000, AvailableVolume: 0, LockedVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)

	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{
		{Code: "sz000001", Price: 11.00, FetchedAt: time.Now()},
		{Code: "sz000002", Price: 9.40, FetchedAt: time.Now()},
		{Code: "sz000003", Price: 8.80, FetchedAt: time.Now()},
	}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	p := getPortfolioObs(t, "drop=sz000003&enter=sz000099")
	dec := p["decision"].(map[string]any)
	require.Equal(t, float64(1), dec["normal_count"])
	require.Equal(t, float64(1), dec["watch_count"])
	require.Equal(t, float64(1), dec["review_count"])
	reb := p["rebalance"].(map[string]any)
	require.Equal(t, float64(1), reb["add_count"])
	require.Equal(t, float64(1), reb["remove_count"])
	require.GreaterOrEqual(t, reb["keep_count"].(float64)+reb["increase_count"].(float64)+reb["decrease_count"].(float64), float64(1))

	foundRemove := false
	for _, raw := range p["positions"].([]any) {
		row := raw.(map[string]any)
		if row["symbol"] == "sz000003" {
			require.Equal(t, "HOLD_REVIEW", row["decision_state"])
			require.Equal(t, "REMOVE", row["rebalance_action"])
			foundRemove = true
		}
	}
	require.True(t, foundRemove)
}

func TestPortfolioObservationAPI_LongHoldingAgingNoSell(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-06-01", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 10.00,
	}).Error)
	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: []marketdata.Quote{{
		Code: "sz000001", Price: 11.00, FetchedAt: time.Now(),
	}}})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	p := getPortfolioObs(t, "target=identity")
	rows := p["positions"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.Equal(t, "LONG", row["holding_period_bucket"])
	require.True(t, row["is_aging"].(bool))
	require.Equal(t, "none", row["action"])
	require.Equal(t, "HEALTHY", row["health_level"])
	health := p["health"].(map[string]any)
	require.Equal(t, float64(1), health["aging_positions"])
	require.Equal(t, "none", p["action"])
	raw, _ := json.Marshal(p)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ? AND stock_code = ?", acc.ID, "sz000001").First(&pos).Error)
	require.InDelta(t, 10.00, pos.MarkPrice, 1e-9)
	require.Equal(t, int64(1000), pos.TotalVolume)
	var acc2 papertrading.PaperSimAccount
	require.NoError(t, db.Dao.First(&acc2, acc.ID).Error)
	require.InDelta(t, acc.Cash, acc2.Cash, 1e-6)
	require.InDelta(t, acc.Equity, acc2.Equity, 1e-6)
}

func TestPortfolioObservationAPI_MissingQuoteNotRiskUpgrade(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, AvgCost: 10.00, MarkPrice: 0, // overlay fallback also missing
	}).Error)
	papertrading.SetHoldingEvalQuoteServiceForTest(&stubHoldingsQuoteService{quotes: nil})
	t.Cleanup(func() { papertrading.SetHoldingEvalQuoteServiceForTest(nil) })

	p := getPortfolioObs(t, "target=identity")
	rows := p["positions"].([]any)
	require.Len(t, rows, 1)
	row := rows[0].(map[string]any)
	require.Equal(t, "UNKNOWN", row["health_level"])
	health := p["health"].(map[string]any)
	require.NotEqual(t, "RISK", health["portfolio_health"])
	require.Equal(t, float64(0), health["risk_positions"])
	require.Equal(t, "none", row["action"])
}
