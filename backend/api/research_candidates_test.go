package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/research"

	"github.com/stretchr/testify/require"
)

func seedResearchSnapshot(t *testing.T) {
	t.Helper()
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()
	research.SetStoreForTest(research.NewMemoryStoreForTest())
	research.SetExplainStoreForTest(research.NewExplainMemoryStoreForTest())
	t.Cleanup(func() {
		research.ResetStoreForTest()
		research.ResetExplainStoreForTest()
	})

	days0 := 0
	created := time.Date(2026, 8, 17, 15, 0, 0, 0, time.UTC)
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行", Tag: "强", DaysAgo: &days0, RSI: 28, StatusText: "今日强化买点", NEW_PRICE: "11", INDUSTRY: "银行"},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  created,
		TradeDate:  "2026-08-17",
		Session:    "close",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: string(raw),
	}).Error)
}

func TestResearchCandidatesAPI_ListEmpty(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()
	research.SetStoreForTest(research.NewMemoryStoreForTest())
	research.SetExplainStoreForTest(research.NewExplainMemoryStoreForTest())
	t.Cleanup(func() {
		research.ResetStoreForTest()
		research.ResetExplainStoreForTest()
	})

	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp research.ListResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Empty(t, resp.Items)
}

func TestResearchCandidatesAPI_ListAndDetail(t *testing.T) {
	seedResearchSnapshot(t)

	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates?trade_date=2026-08-17", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var list research.ListResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Equal(t, "2026-08-17", list.TradeDate)
	require.GreaterOrEqual(t, len(list.Items), 1)
	id := list.Items[0].ID
	require.Equal(t, "rc:signal:2026-08-17:sz000001", id)
	require.Equal(t, research.StatusNew, list.Items[0].Status)
	require.Equal(t, research.SourceSignalSnapshot, list.Items[0].Source)
	require.NotNil(t, list.Items[0].ExplainRef)
	require.NotEmpty(t, list.Items[0].ExplainSummary)

	req2 := httptest.NewRequest(http.MethodGet, "/api/research/candidates/"+id, nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var detail research.DetailResult
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &detail))
	require.Equal(t, id, detail.Candidate.ID)
	require.True(t, detail.Explanation.Available)
	require.NotEmpty(t, detail.Explanation.Summary)
	require.NotEmpty(t, detail.Explanation.ExplainRef)
	require.NotNil(t, detail.Candidate.ExplainRef)
	require.Nil(t, detail.Links.TradePoolID)
	require.Nil(t, detail.Links.TradePlanID)
}

func TestResearchCandidatesAPI_PatchStatusNoteTags(t *testing.T) {
	seedResearchSnapshot(t)
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	id := "rc:signal:2026-08-17:sz000001"
	body := map[string]any{
		"status": "WATCHING",
		"note":   "研究备注：关注放量",
		"tags":   []string{"银行", "观察"},
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch, "/api/research/candidates/"+id, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var detail research.DetailResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detail))
	require.Equal(t, research.StatusWatching, detail.Candidate.Status)
	require.Equal(t, "研究备注：关注放量", detail.Candidate.Note)
	require.Equal(t, []string{"银行", "观察"}, detail.Candidate.Tags)
	require.NotNil(t, detail.Candidate.UpdatedAt)

	// List reflects overlay + status filter
	req2 := httptest.NewRequest(http.MethodGet, "/api/research/candidates?status=watching", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	var list research.ListResult
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	require.Equal(t, research.StatusWatching, list.Items[0].Status)
}

func TestResearchCandidatesAPI_PatchInvalidStatus(t *testing.T) {
	seedResearchSnapshot(t)
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	id := "rc:signal:2026-08-17:sz000001"
	raw, _ := json.Marshal(map[string]any{"status": "ready"})
	req := httptest.NewRequest(http.MethodPatch, "/api/research/candidates/"+id, bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResearchCandidatesAPI_BadTradeDate(t *testing.T) {
	setupAPITestDB(t)
	research.SetStoreForTest(research.NewMemoryStoreForTest())
	research.SetExplainStoreForTest(research.NewExplainMemoryStoreForTest())
	t.Cleanup(func() {
		research.ResetStoreForTest()
		research.ResetExplainStoreForTest()
	})
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates?trade_date=2026/08/17", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResearchCandidatesAPI_DetailNotFound(t *testing.T) {
	setupAPITestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &data.Settings{}))
	data.ResetSettingCacheForTest()
	research.SetStoreForTest(research.NewMemoryStoreForTest())
	research.SetExplainStoreForTest(research.NewExplainMemoryStoreForTest())
	t.Cleanup(func() {
		research.ResetStoreForTest()
		research.ResetExplainStoreForTest()
	})

	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates/rc:signal:2099-01-01:sz999999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestResearchCandidatesAPI_NoPromoteRoute(t *testing.T) {
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/api/research/candidates/promote", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestResearchExplainAPI_GetAndPatch(t *testing.T) {
	seedResearchSnapshot(t)
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)

	id := "rc:signal:2026-08-17:sz000001"
	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates/"+id+"/explain", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var ex research.Explain
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ex))
	require.True(t, ex.Available)
	require.Equal(t, research.ExplainSchemaVersion, ex.SchemaVersion)
	require.Equal(t, id, ex.CandidateID)
	require.Equal(t, research.MakeExplainID(id), ex.ID)
	require.Equal(t, research.ExplainTypeSignalDerived, ex.ExplainType)
	require.NotEmpty(t, ex.Summary)
	require.NotEmpty(t, ex.Evidence.EvidenceHash)
	require.Nil(t, ex.StrategyIntentRef)
	require.NotNil(t, ex.RiskNote)

	// alias
	reqA := httptest.NewRequest(http.MethodGet, "/api/research/explains/"+ex.ID, nil)
	recA := httptest.NewRecorder()
	mux.ServeHTTP(recA, reqA)
	require.Equal(t, http.StatusOK, recA.Code)

	sum := "人工修订摘要"
	raw, err := json.Marshal(map[string]any{
		"summary": sum,
		"research_reason": map[string]string{
			"kind": "analyst_note",
			"text": "人工研究理由",
		},
		"risk_note": map[string]string{
			"severity": "warn",
			"text":     "注意波动",
		},
	})
	require.NoError(t, err)
	req2 := httptest.NewRequest(http.MethodPatch, "/api/research/candidates/"+id+"/explain", bytes.NewReader(raw))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)

	var ex2 research.Explain
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &ex2))
	require.Equal(t, sum, ex2.Summary)
	require.Equal(t, research.ExplainTypeHybrid, ex2.ExplainType)
	require.Equal(t, research.ReasonKindAnalystNote, ex2.ResearchReason.Kind)
	require.Equal(t, "人工研究理由", ex2.ResearchReason.Text)
	require.NotNil(t, ex2.RiskNote)
	require.Equal(t, research.RiskSeverityWarn, ex2.RiskNote.Severity)
	// evidence still derived
	require.NotEmpty(t, ex2.Evidence.SignalTag)
}

func TestResearchExplainAPI_NotFound(t *testing.T) {
	seedResearchSnapshot(t)
	mux := http.NewServeMux()
	api.RegisterResearchCandidatesRoutes(mux)
	req := httptest.NewRequest(http.MethodGet, "/api/research/candidates/rc:signal:2099-01-01:sz999999/explain", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
