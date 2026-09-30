package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

func TestExitWatchAPI_GetOnlyFailClosed(t *testing.T) {
	orig := db.Dao
	db.Dao = nil
	t.Cleanup(func() { db.Dao = orig })

	mux := http.NewServeMux()
	api.RegisterExitWatchRoutes(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/exit-watch", nil))
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/exit-watch?source=live_broker", nil))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	var bad map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &bad))
	require.Equal(t, "invalid_source", bad["reason"])

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/exit-watch", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		OK        bool `json:"ok"`
		ExitWatch struct {
			Items           []any  `json:"items"`
			AutoSell        bool   `json:"auto_sell"`
			PersistTrailing bool   `json:"persist_trailing"`
			PersistHWM      bool   `json:"persist_hwm"`
			NewLedger       bool   `json:"new_ledger"`
			WritesTradePlan bool   `json:"writes_trade_plan"`
			WritesPaperSim  bool   `json:"writes_paper_sim"`
			Disclaimer      string `json:"disclaimer"`
			PolicyRef       string `json:"policy_ref"`
			Sources         []struct {
				Source string `json:"source"`
				OK     bool   `json:"ok"`
			} `json:"sources"`
		} `json:"exit_watch"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.OK)
	require.Empty(t, body.ExitWatch.Items)
	require.False(t, body.ExitWatch.AutoSell)
	require.False(t, body.ExitWatch.PersistTrailing)
	require.False(t, body.ExitWatch.PersistHWM)
	require.False(t, body.ExitWatch.NewLedger)
	require.False(t, body.ExitWatch.WritesTradePlan)
	require.False(t, body.ExitWatch.WritesPaperSim)
	require.Equal(t, "观察≠卖出指令", body.ExitWatch.Disclaimer)
	require.Equal(t, "default_v1@v1", body.ExitWatch.PolicyRef)
	require.NotEmpty(t, body.ExitWatch.Sources)
	for _, st := range body.ExitWatch.Sources {
		require.False(t, st.OK)
	}
}
