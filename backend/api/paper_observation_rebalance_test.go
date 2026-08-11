package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"
	"go-stock/backend/papertrading"

	"github.com/stretchr/testify/require"
)

func getRebalanceObs(t *testing.T, query string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	path := "/api/papertrading/observation/rebalance"
	if query != "" {
		path += "?" + query
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body["ok"].(bool))
	return body
}

func TestRebalanceObservationAPI_GETOnly(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterPaperObservationRoutes(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/papertrading/observation/rebalance", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestRebalanceObservationAPI_IdentityKeep(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 1000, LockedVolume: 0, AvgCost: 10, MarkPrice: 10,
	}).Error)

	body := getRebalanceObs(t, "target=identity")
	require.Contains(t, body["note"].(string), "不是交易建议")
	diff := body["diff"].(map[string]any)
	counts := diff["counts"].(map[string]any)
	require.Equal(t, float64(1), counts["keep_count"])
	require.Equal(t, float64(0), counts["add_count"])
	raw, _ := json.Marshal(body)
	up := strings.ToUpper(string(raw))
	require.NotContains(t, up, `"BUY"`)
	require.NotContains(t, up, `"SELL"`)

	var pos papertrading.PaperSimPosition
	require.NoError(t, db.Dao.Where("account_id = ?", acc.ID).First(&pos).Error)
	require.InDelta(t, 10.0, pos.MarkPrice, 1e-9)
	var acc2 papertrading.PaperSimAccount
	require.NoError(t, db.Dao.First(&acc2, acc.ID).Error)
	require.InDelta(t, 1_000_000.0, acc2.Cash, 1e-6)
}

func TestRebalanceObservationAPI_EnterAndDrop(t *testing.T) {
	setupAPITestDB(t)
	acc := seedEvalObsAccount(t)
	plan, item := seedEvalObsPlanItem(t, "2026-08-11", "sz000001", "平安银行")
	seedEvalObsFill(t, acc.ID, plan, item, 10.00, 1000)
	require.NoError(t, db.Dao.Create(&papertrading.PaperSimPosition{
		AccountID: acc.ID, StockCode: "sz000001", StockName: "平安银行",
		TotalVolume: 1000, AvailableVolume: 0, LockedVolume: 1000, AvgCost: 10, MarkPrice: 10,
	}).Error)

	body := getRebalanceObs(t, "drop=sz000001&enter=sz000002")
	diff := body["diff"].(map[string]any)
	counts := diff["counts"].(map[string]any)
	require.Equal(t, float64(1), counts["remove_count"])
	require.Equal(t, float64(1), counts["add_count"])
	items := diff["items"].([]any)
	foundLocked := false
	for _, raw := range items {
		it := raw.(map[string]any)
		if it["action"] == "REMOVE" {
			require.Equal(t, float64(0), it["executable_qty"])
			flags, _ := it["constraint_flags"].([]any)
			joined := ""
			for _, f := range flags {
				joined += f.(string) + ","
			}
			require.Contains(t, joined, "t1_locked")
			require.Equal(t, false, it["switch_ok"])
			foundLocked = true
		}
	}
	require.True(t, foundLocked)
}
