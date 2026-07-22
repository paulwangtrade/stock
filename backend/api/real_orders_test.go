package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-stock/backend/api"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/execution"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAPITestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:phase3_api_%s?mode=memory&cache=shared&_busy_timeout=10000", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, data.MigratePaperTrading(testDB))
	db.Dao = testDB
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func TestRealOrdersAPI_ListAndGet(t *testing.T) {
	setupAPITestDB(t)
	real, err := execution.NewRealBrokerPersistent()
	require.NoError(t, err)
	h := execution.NewFakeExecutionReportHandler(real)

	order, err := real.Submit(context.Background(), execution.SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), execution.ExecutionReport{
		ReportType: execution.ExecReportTypeACK, ReportID: "api-ack", ClientOrderID: order.ClientOrderID,
	}))
	require.NoError(t, h.OnExecutionReport(context.Background(), execution.ExecutionReport{
		ReportType: execution.ExecReportTypeTRADE, ReportID: "api-t1", ExecID: "API-E1",
		ClientOrderID: order.ClientOrderID, LastQty: 40, LastPrice: 10.2,
	}))

	mux := http.NewServeMux()
	api.RegisterRealOrderRoutes(mux)

	listReq := httptest.NewRequest(http.MethodGet, "/api/real/orders", nil)
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	var list api.RealOrderListResponse
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &list))
	require.Len(t, list.Orders, 1)
	require.Equal(t, order.ClientOrderID, list.Orders[0].ClientOrderID)
	require.Equal(t, int64(40), list.Orders[0].FilledVolume)
	require.Equal(t, data.BrokerStatusPartiallyFilled, list.Orders[0].BrokerStatus)

	detailReq := httptest.NewRequest(http.MethodGet, "/api/real/order/"+order.ID, nil)
	detailRec := httptest.NewRecorder()
	mux.ServeHTTP(detailRec, detailReq)
	require.Equal(t, http.StatusOK, detailRec.Code)

	var detail api.RealOrderDetailResponse
	require.NoError(t, json.Unmarshal(detailRec.Body.Bytes(), &detail))
	require.Equal(t, order.ClientOrderID, detail.Order.ClientOrderID)
	require.NotEmpty(t, detail.Order.BrokerOrderID)
	require.Len(t, detail.Fills, 1)
	require.Equal(t, int64(40), detail.Fills[0].FillQty)
	require.InDelta(t, 10.2, detail.Fills[0].FillPrice, 1e-9)

	// 支持 client_order_id 查询
	byClient := httptest.NewRequest(http.MethodGet, "/api/real/order/"+order.ClientOrderID, nil)
	byClientRec := httptest.NewRecorder()
	mux.ServeHTTP(byClientRec, byClient)
	require.Equal(t, http.StatusOK, byClientRec.Code)
}

func TestRealOrdersAPI_StatusFilter(t *testing.T) {
	setupAPITestDB(t)
	real, err := execution.NewRealBrokerPersistent()
	require.NoError(t, err)
	h := execution.NewFakeExecutionReportHandler(real)

	o1, err := real.Submit(context.Background(), execution.SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	o2, err := real.Submit(context.Background(), execution.SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 11, Volume: 100,
	})
	require.NoError(t, err)
	require.NoError(t, h.OnExecutionReport(context.Background(), execution.ExecutionReport{
		ReportType: execution.ExecReportTypeREJECT, ReportID: "api-rj",
		ClientOrderID: o2.ClientOrderID, RejectCode: "TEST",
	}))
	_ = o1

	mux := http.NewServeMux()
	api.RegisterRealOrderRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/real/orders?status=rejected", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var list api.RealOrderListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Orders, 1)
	require.Equal(t, data.PaperOrderStatusRejected, list.Orders[0].Status)
}

func TestRealOrdersAPI_NotFound(t *testing.T) {
	setupAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterRealOrderRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/real/order/missing-id", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRealOrdersAPI_EmptyList(t *testing.T) {
	setupAPITestDB(t)
	mux := http.NewServeMux()
	api.RegisterRealOrderRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/real/orders", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var list api.RealOrderListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.NotNil(t, list.Orders)
	require.Len(t, list.Orders, 0)
}
