package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func postFreeze(t *testing.T, mux *http.ServeMux, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/tradeplans/freeze", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func approvePlanForFreeze(t *testing.T, planID uint, actor string) {
	t.Helper()
	at := time.Now()
	ok, err := data.NewTradePlanRepo().ApproveDraftGate(planID, actor, "http_api", at)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestTradePlansFreezeAPI_Success(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	approvePlanForFreeze(t, plan.ID, "alice")
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "reason": "eod freeze", "source": "ui",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.Equal(t, "FROZEN", resp.ResultCode)
	require.Equal(t, plan.ID, resp.PlanID)
	require.Equal(t, models.TradePlanStatusReady, resp.Status)
	require.True(t, resp.IsFrozen)
	require.NotEmpty(t, resp.FreezeAt)
	require.Equal(t, "alice", resp.FreezeBy)
	require.Equal(t, "eod freeze", resp.FreezeReason)
	require.Equal(t, "ui", resp.FreezeSource)
	require.NotEmpty(t, resp.ApprovedAt)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.True(t, got.IsFrozen())
	require.Equal(t, models.TradePlanStatusReady, got.Status)
	require.Equal(t, "alice", got.FreezeBy)
	require.NotNil(t, got.ApprovedAt)
}

func TestTradePlansFreezeAPI_NotApproved(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "ui",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeNotApproved, resp.Code)
	require.False(t, resp.OK)
	require.Equal(t, "NOT_APPROVED", resp.ResultCode)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Nil(t, got.FreezeAt)
}

func TestTradePlansFreezeAPI_RiskDeny(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	approvePlanForFreeze(t, plan.ID, "alice")
	mux := registerApproveTestMux(t, approveEligOpts(failRiskForAPI))

	rec := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "ui",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var resp api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeRiskDeny, resp.Code)
	require.False(t, resp.OK)
	require.Equal(t, "DENIED", resp.ResultCode)
	require.NotEmpty(t, resp.Blockers)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, got.Status)
	require.Nil(t, got.FreezeAt)
}

func TestTradePlansFreezeAPI_AlreadyFrozenIdempotent(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	approvePlanForFreeze(t, plan.ID, "alice")
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	first := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "reason": "first", "source": "ui",
	})
	require.Equal(t, http.StatusOK, first.Code)

	second := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "carol", "reason": "second", "source": "ui",
	})
	require.Equal(t, http.StatusOK, second.Code)

	var resp api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &resp))
	require.Equal(t, api.TradePlanCodeOK, resp.Code)
	require.True(t, resp.OK)
	require.Equal(t, "ALREADY_FROZEN", resp.ResultCode)
	require.True(t, resp.AlreadyFrozen)
	require.Equal(t, "alice", resp.FreezeBy)
	require.Equal(t, "first", resp.FreezeReason)
}

func TestTradePlansFreezeAPI_Validation(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	missingID := postFreeze(t, mux, map[string]any{"actor": "alice", "source": "ui"})
	require.Equal(t, http.StatusBadRequest, missingID.Code)
	var badID api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(missingID.Body.Bytes(), &badID))
	require.Equal(t, api.TradePlanCodeInvalidPlanID, badID.Code)

	missingActor := postFreeze(t, mux, map[string]any{"plan_id": 1, "source": "ui"})
	require.Equal(t, http.StatusBadRequest, missingActor.Code)
	var badActor api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(missingActor.Body.Bytes(), &badActor))
	require.Equal(t, api.TradePlanCodeBadActor, badActor.Code)

	missing := postFreeze(t, mux, map[string]any{"plan_id": 999999, "actor": "alice", "source": "ui"})
	require.Equal(t, http.StatusOK, missing.Code)
	var notFound api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(missing.Body.Bytes(), &notFound))
	require.Equal(t, api.TradePlanCodeNoPlan, notFound.Code)
}

func TestTradePlansFreezeAPI_DoesNotCreateOrders(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	approvePlanForFreeze(t, plan.ID, "alice")
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	rec := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "ui",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, got.Status)
	require.NotEqual(t, models.TradePlanStatusExecuting, got.Status)
	require.Nil(t, got.ExecutedAt)
	for _, it := range got.Items {
		require.Equal(t, models.TradePlanItemPending, it.Status)
	}
}

func TestTradePlansFreezeAPI_CallsFreezeServiceBoundary(t *testing.T) {
	raw, err := os.ReadFile("tradeplans_freeze.go")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, "strategy.FreezeTradePlan")
	require.NotContains(t, src, "PromoteDraftToFrozen")
	require.NotContains(t, src, "TryBeginExecute")
	require.NotContains(t, src, "RunPaperOpenBuyOnce")
}

func TestTradePlansApproveThenFreeze_StateMachine(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, data.EnsureTradePlanTables())
	plan := seedAPIMaterializedDraft(t)
	mux := registerApproveTestMux(t, approveEligOpts(passRiskForAPI))

	before, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, before.Status)
	require.Nil(t, before.ApprovedAt)
	require.Nil(t, before.FreezeAt)

	apr := postApprove(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "ui",
	})
	require.Equal(t, http.StatusOK, apr.Code)
	var aprResp api.TradePlanApproveResponse
	require.NoError(t, json.Unmarshal(apr.Body.Bytes(), &aprResp))
	require.True(t, aprResp.OK)
	require.Equal(t, models.TradePlanStatusDraft, aprResp.Status)
	require.NotEmpty(t, aprResp.ApprovedAt)

	mid, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusDraft, mid.Status)
	require.NotNil(t, mid.ApprovedAt)
	require.Nil(t, mid.FreezeAt)

	frz := postFreeze(t, mux, map[string]any{
		"plan_id": plan.ID, "actor": "alice", "source": "ui",
	})
	require.Equal(t, http.StatusOK, frz.Code)
	var frzResp api.TradePlanFreezeResponse
	require.NoError(t, json.Unmarshal(frz.Body.Bytes(), &frzResp))
	require.True(t, frzResp.OK)
	require.Equal(t, models.TradePlanStatusReady, frzResp.Status)
	require.NotEmpty(t, frzResp.FreezeAt)

	after, err := data.NewTradePlanRepo().GetByID(plan.ID)
	require.NoError(t, err)
	require.Equal(t, models.TradePlanStatusReady, after.Status)
	require.NotNil(t, after.ApprovedAt)
	require.NotNil(t, after.FreezeAt)
	require.True(t, after.IsFrozen())
	require.Nil(t, after.ExecutedAt)
}