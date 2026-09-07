package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/execution"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func newManualTradeMux(facade execution.ManualTradeExecutor) *http.ServeMux {
	mux := http.NewServeMux()
	api.RegisterManualTradeRoutes(mux, facade)
	return mux
}

func newManualFacadeWithPort(port execution.ExecutionPort) execution.ManualTradeExecutor {
	return execution.NewManualTradeFacade(execution.NewExecutionService(port), nil)
}

func parseManualEnvelope(t *testing.T, rec *httptest.ResponseRecorder) (api.ManualTradeAPIEnvelope, json.RawMessage) {
	t.Helper()
	var env api.ManualTradeAPIEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	raw, _ := json.Marshal(env.Data)
	return env, raw
}

func TestManualTradeAPI_CreateReturnsDraft(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())

	mux := newManualTradeMux(nil)
	rec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"account_id": 1,
		"symbol":     "sh600036",
		"stock_name": "招商银行",
		"side":       "buy",
		"price":      40.1,
		"volume":     100,
		"reason":     "模拟执行台",
		"order_kind": "normal",
	})
	require.Equal(t, http.StatusOK, rec.Code)
	env, dataRaw := parseManualEnvelope(t, rec)
	require.Equal(t, 0, env.Code)
	var idData api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(dataRaw, &idData))
	require.NotZero(t, idData.ID)
	require.Equal(t, models.ManualTradeIntentStatusDraft, idData.Status)

	got, err := data.NewManualTradeIntentRepo().GetManualTradeIntent(idData.ID)
	require.NoError(t, err)
	require.Equal(t, models.ManualSourcePaperTradingPanel, got.ManualSource)
	require.Equal(t, models.ManualTradeOrderKindNormal, got.OrderKind)
}

func TestManualTradeAPI_ConfirmDraftOK(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())
	mux := newManualTradeMux(nil)

	createRec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sz000001", "side": "buy", "price": 10, "volume": 100,
	})
	env, raw := parseManualEnvelope(t, createRec)
	require.Equal(t, 0, env.Code)
	var created api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw, &created))

	confirmRec := postJSON(t, mux, "/api/manual_trade/intent/confirm", map[string]any{
		"intent_id": created.ID,
	})
	require.Equal(t, http.StatusOK, confirmRec.Code)
	env2, raw2 := parseManualEnvelope(t, confirmRec)
	require.Equal(t, 0, env2.Code)
	var confirmed api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw2, &confirmed))
	require.Equal(t, models.ManualTradeIntentStatusConfirmed, confirmed.Status)
}

func TestManualTradeAPI_ConfirmSubmittedFails(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())
	mux := newManualTradeMux(nil)

	createRec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sz000001", "price": 10, "volume": 100,
	})
	_, raw := parseManualEnvelope(t, createRec)
	var created api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw, &created))
	postJSON(t, mux, "/api/manual_trade/intent/confirm", map[string]any{"intent_id": created.ID})

	repo := data.NewManualTradeIntentRepo()
	require.NoError(t, repo.UpdateManualTradeIntentStatus(created.ID, models.ManualTradeIntentStatusSubmitted, &data.ManualTradeIntentStatusPatch{
		OrderID: 1, ClientOrderID: "x",
	}))

	bad := postJSON(t, mux, "/api/manual_trade/intent/confirm", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusBadRequest, bad.Code)
	env, _ := parseManualEnvelope(t, bad)
	require.NotEqual(t, 0, env.Code)
	require.NotEmpty(t, env.Error)
}

func TestManualTradeAPI_ExecuteConfirmedSuccess(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())
	acc, err := data.NewPaperTradingApi().ResetAccount(1_000_000)
	require.NoError(t, err)

	port := &apiFakePort{
		order: &broker.TradeOrder{
			ID: "601", ClientOrderID: "cli-manual-1", Status: data.PaperOrderStatusFilled,
		},
	}
	mux := newManualTradeMux(newManualFacadeWithPort(port))

	createRec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sh600036", "side": "buy", "price": 10, "volume": 100, "account_id": acc.ID,
	})
	_, raw := parseManualEnvelope(t, createRec)
	var created api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw, &created))
	postJSON(t, mux, "/api/manual_trade/intent/confirm", map[string]any{"intent_id": created.ID})

	execRec := postJSON(t, mux, "/api/manual_trade/intent/execute", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusOK, execRec.Code)
	env, dataRaw := parseManualEnvelope(t, execRec)
	require.Equal(t, 0, env.Code)
	var execData api.ExecuteManualTradeIntentData
	require.NoError(t, json.Unmarshal(dataRaw, &execData))
	require.Equal(t, models.ManualTradeIntentStatusSubmitted, execData.Status)
	require.Equal(t, uint(601), execData.OrderID)
	require.Equal(t, "cli-manual-1", execData.ClientOrderID)
	require.Equal(t, 1, port.calls)
	require.Equal(t, models.ManualTradeStrategyTag, port.last.StrategyTag)
}

func TestManualTradeAPI_ExecuteDraftRejected(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())
	port := &apiFakePort{order: &broker.TradeOrder{ID: "1"}}
	mux := newManualTradeMux(newManualFacadeWithPort(port))

	createRec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sz000001", "price": 10, "volume": 100,
	})
	_, raw := parseManualEnvelope(t, createRec)
	var created api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw, &created))

	execRec := postJSON(t, mux, "/api/manual_trade/intent/execute", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusBadRequest, execRec.Code)
	require.Equal(t, 0, port.calls)
}

func TestManualTradeAPI_ExecuteCallsManualFacade(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureManualTradeIntentTables())
	acc, err := data.NewPaperTradingApi().ResetAccount(1_000_000)
	require.NoError(t, err)

	port := &apiFakePort{
		order: &broker.TradeOrder{ID: "7", ClientOrderID: "c7"},
	}
	mux := newManualTradeMux(newManualFacadeWithPort(port))

	createRec := postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sh600000", "side": "sell", "price": 10, "volume": 100, "account_id": acc.ID,
	})
	// sell needs position — use buy for this chain call proof
	_ = createRec
	createRec = postJSON(t, mux, "/api/manual_trade/intent", map[string]any{
		"symbol": "sh600000", "side": "buy", "price": 10, "volume": 100, "account_id": acc.ID,
	})
	_, raw := parseManualEnvelope(t, createRec)
	var created api.ManualTradeIntentIDData
	require.NoError(t, json.Unmarshal(raw, &created))
	postJSON(t, mux, "/api/manual_trade/intent/confirm", map[string]any{"intent_id": created.ID})
	postJSON(t, mux, "/api/manual_trade/intent/execute", map[string]any{"intent_id": created.ID})
	require.Equal(t, 1, port.calls, "必须经 ManualTradeFacade → ExecutionService → Port")
	require.Equal(t, "sh600000", port.last.StockCode)
}

func TestManualTradeAPI_SourceHasNoPaperTradingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "manual_trade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)
	for _, needle := range []string{"SubmitPaperOrder", "FillPaperOrder", "NewPaperTradingApi", "PaperTradingApi"} {
		require.NotContains(t, body, needle)
	}
	require.Contains(t, body, "ConfirmAndExecuteManualTrade")
}
