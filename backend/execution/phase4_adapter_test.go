package execution

import (
	"context"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"

	"github.com/stretchr/testify/require"
)

func TestPhase4_RealBroker_DelegatesToStubAdapter(t *testing.T) {
	stub := broker.NewConnectedStubAdapter()
	real := NewRealBrokerWithAdapter(stub)

	order, err := real.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, order.Status)
	require.Equal(t, real.Adapter(), stub)

	st, err := stub.Query(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, broker.StubStatusPending, st.Status)

	h := NewFakeExecutionReportHandler(real)
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeACK, ReportID: "p4-ack", ClientOrderID: order.ClientOrderID,
	}))
	got, err := real.QueryOrder(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.NotEmpty(t, got.BrokerOrderID)

	st, err = stub.Query(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, got.BrokerOrderID, st.BrokerOrderID)

	require.NoError(t, real.Cancel(context.Background(), order.ID))
	st, err = stub.Query(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, broker.StubStatusCancelled, st.Status)

	oms, err := real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusPending, oms.Status)
	require.Equal(t, data.BrokerStatusCancelPending, oms.BrokerStatus)

	// CANCEL 回报确认后才落 OMS 终态。
	require.NoError(t, h.OnExecutionReport(context.Background(), ExecutionReport{
		ReportType: ExecReportTypeCANCEL, ReportID: "p4-cx", ClientOrderID: order.ClientOrderID,
	}))
	oms, err = real.QueryOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, data.PaperOrderStatusCancelled, oms.Status)
}

func TestPhase4_HydrateRestoresStubAdapter(t *testing.T) {
	setupCutoverTestDB(t)
	real1, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	order, err := real1.Submit(context.Background(), SubmitIntent{
		StockCode: "sz000002", Side: "buy", Price: 11, Volume: 50,
	})
	require.NoError(t, err)

	real2, err := NewRealBrokerPersistent()
	require.NoError(t, err)
	stub, ok := real2.Adapter().(*broker.StubAdapter)
	require.True(t, ok)
	st, err := stub.Query(context.Background(), order.ClientOrderID)
	require.NoError(t, err)
	require.Equal(t, data.BrokerStatusPending, st.Status)
	require.Equal(t, int64(50), st.LeavesQuantity)
}
