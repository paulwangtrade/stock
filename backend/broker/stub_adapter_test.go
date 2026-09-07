package broker_test

import (
	"context"
	"strings"
	"testing"

	"go-stock/backend/broker"

	"github.com/stretchr/testify/require"
)

func TestStubAdapter_ConnectSubmitQueryCancel(t *testing.T) {
	a := broker.NewStubAdapter()
	_, err := a.Submit(context.Background(), &broker.SubmitRequest{
		ClientOrderID: "real_x", StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.Error(t, err, "未 Connect 不得报单")

	require.NoError(t, a.Connect(context.Background()))
	resp, err := a.Submit(context.Background(), &broker.SubmitRequest{
		ClientOrderID: "real_x", StockCode: "sz000001", Side: "buy", Price: 10, Volume: 100,
	})
	require.NoError(t, err)
	require.Equal(t, broker.StubStatusPending, resp.Status)
	require.Empty(t, resp.BrokerOrderID)

	st, err := a.Query(context.Background(), "real_x")
	require.NoError(t, err)
	require.Equal(t, broker.StubStatusPending, st.Status)

	bid := broker.NewStubBrokerOrderID("real_x")
	require.True(t, strings.HasPrefix(bid, "real_stub_"))
	a.BindBrokerOrderID("real_x", bid)
	st, err = a.Query(context.Background(), "real_x")
	require.NoError(t, err)
	require.Equal(t, bid, st.BrokerOrderID)
	require.Equal(t, broker.StubStatusWorking, st.Status)

	require.NoError(t, a.Cancel(context.Background(), "real_x"))
	st, err = a.Query(context.Background(), "real_x")
	require.NoError(t, err)
	require.Equal(t, broker.StubStatusCancelled, st.Status)

	require.NoError(t, a.Disconnect(context.Background()))
}
