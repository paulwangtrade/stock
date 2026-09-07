package api_test

import (
	"bytes"
	"context"
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

func postJSON(t *testing.T, mux http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func newResearchTradeMux(facade execution.ResearchTradeExecutor) *http.ServeMux {
	mux := http.NewServeMux()
	api.RegisterResearchTradeRoutes(mux, facade)
	return mux
}

type apiFakePort struct {
	calls int
	order *broker.TradeOrder
	err   error
	last  execution.SubmitIntent
}

func (p *apiFakePort) Submit(ctx context.Context, intent execution.SubmitIntent) (*broker.TradeOrder, error) {
	_ = ctx
	p.calls++
	p.last = intent
	return p.order, p.err
}

func (p *apiFakePort) QueryOrder(ctx context.Context, orderID string) (*broker.TradeOrder, error) {
	_ = ctx
	_ = orderID
	return nil, nil
}

func (p *apiFakePort) Cancel(ctx context.Context, orderID string) error {
	_ = ctx
	_ = orderID
	return nil
}

func newFacadeWithPort(port execution.ExecutionPort) execution.ResearchTradeExecutor {
	return execution.NewResearchTradeFacade(execution.NewExecutionService(port), nil)
}

func TestResearchTradeAPI_CreateIntentDraft(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureResearchTradeIntentTables())

	mux := newResearchTradeMux(nil)
	rec := postJSON(t, mux, "/api/research_trade/intent", map[string]any{
		"symbol":                "sh600036",
		"stock_name":            "招商银行",
		"price":                 40.1,
		"volume":                100,
		"account_id":            1,
		"candidate_snapshot_id": 88,
		"signal_score":          97,
		"signal_tag":            "强",
		"reason":                "api create",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.ResearchTradeIntentIDResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotZero(t, resp.ID)
	require.Equal(t, models.ResearchTradeIntentStatusDraft, resp.Status)

	got, err := data.NewResearchTradeIntentRepo().GetResearchTradeIntent(resp.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusDraft, got.Status)
	require.Equal(t, models.ResearchSourceSignalScanSnapshot, got.ResearchSource)
	require.Equal(t, uint(88), got.CandidateSnapshotID)
	require.InDelta(t, 97.0, got.SignalScore, 0.01)
	require.Equal(t, "强", got.SignalTag)
}

func TestResearchTradeAPI_ConfirmAndIllegalTransition(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureResearchTradeIntentTables())
	mux := newResearchTradeMux(nil)

	createRec := postJSON(t, mux, "/api/research_trade/intent", map[string]any{
		"symbol": "sz000001", "price": 10, "volume": 100,
	})
	require.Equal(t, http.StatusOK, createRec.Code)
	var created api.ResearchTradeIntentIDResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	confirmRec := postJSON(t, mux, "/api/research_trade/intent/confirm", map[string]any{
		"intent_id": created.ID,
	})
	require.Equal(t, http.StatusOK, confirmRec.Code)
	var confirmed api.ResearchTradeIntentIDResponse
	require.NoError(t, json.Unmarshal(confirmRec.Body.Bytes(), &confirmed))
	require.Equal(t, models.ResearchTradeIntentStatusConfirmed, confirmed.Status)

	repo := data.NewResearchTradeIntentRepo()
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(created.ID, models.ResearchTradeIntentStatusSubmitted, &data.ResearchTradeIntentStatusPatch{
		OrderID: 1, ClientOrderID: "x",
	}))
	bad := postJSON(t, mux, "/api/research_trade/intent/confirm", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestResearchTradeAPI_ExecuteSuccessViaPort(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureResearchTradeIntentTables())
	acc, err := data.NewPaperTradingApi().ResetAccount(1_000_000)
	require.NoError(t, err)

	port := &apiFakePort{
		order: &broker.TradeOrder{
			ID:            "501",
			ClientOrderID: "cli-api-1",
			Status:        data.PaperOrderStatusFilled,
			StockCode:     "sh600036",
		},
	}
	mux := newResearchTradeMux(newFacadeWithPort(port))

	createRec := postJSON(t, mux, "/api/research_trade/intent", map[string]any{
		"symbol": "sh600036", "stock_name": "招商银行", "price": 10, "volume": 100,
		"account_id": acc.ID, "candidate_snapshot_id": 7, "signal_tag": "强",
	})
	require.Equal(t, http.StatusOK, createRec.Code)
	var created api.ResearchTradeIntentIDResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	confirmRec := postJSON(t, mux, "/api/research_trade/intent/confirm", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusOK, confirmRec.Code)

	execRec := postJSON(t, mux, "/api/research_trade/intent/execute", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusOK, execRec.Code)

	var execResp api.ExecuteResearchTradeIntentResponse
	require.NoError(t, json.Unmarshal(execRec.Body.Bytes(), &execResp))
	require.Equal(t, models.ResearchTradeIntentStatusSubmitted, execResp.Status)
	require.Equal(t, uint(501), execResp.OrderID)
	require.Equal(t, "cli-api-1", execResp.ClientOrderID)
	require.Equal(t, 1, port.calls)
	require.Equal(t, "sh600036", port.last.StockCode)
	require.Equal(t, models.ResearchTradeStrategyTag, port.last.StrategyTag)
}

func TestResearchTradeAPI_ExecuteFailureCashInsufficient(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureResearchTradeIntentTables())
	acc, err := data.NewPaperTradingApi().ResetAccount(1_000_000)
	require.NoError(t, err)

	port := &apiFakePort{order: &broker.TradeOrder{ID: "9"}}
	mux := newResearchTradeMux(newFacadeWithPort(port))

	// 价量乘积远超账户现金 → PreTradeCheck CASH_INSUFFICIENT，Port 不调用
	createRec := postJSON(t, mux, "/api/research_trade/intent", map[string]any{
		"symbol": "sh600036", "price": 99999, "volume": 100, "account_id": acc.ID,
	})
	var created api.ResearchTradeIntentIDResponse
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	postJSON(t, mux, "/api/research_trade/intent/confirm", map[string]any{"intent_id": created.ID})

	execRec := postJSON(t, mux, "/api/research_trade/intent/execute", map[string]any{"intent_id": created.ID})
	require.Equal(t, http.StatusOK, execRec.Code)

	var execResp api.ExecuteResearchTradeIntentResponse
	require.NoError(t, json.Unmarshal(execRec.Body.Bytes(), &execResp))
	require.Equal(t, models.ResearchTradeIntentStatusRejected, execResp.Status)
	require.Equal(t, data.PaperOrderRejectCashInsufficient, execResp.ErrorCode)
	require.Equal(t, 0, port.calls)
}

func TestResearchTradeAPI_SourceHasNoPaperTradingBypass(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	src := filepath.Join(filepath.Dir(thisFile), "research_trade.go")
	b, err := os.ReadFile(src)
	require.NoError(t, err)
	body := string(b)
	for _, needle := range []string{"SubmitPaperOrder", "FillPaperOrder", "NewPaperTradingApi", "PaperTrading"} {
		require.NotContains(t, body, needle)
	}
	require.Contains(t, body, "ConfirmAndExecuteResearchBuy")
}
