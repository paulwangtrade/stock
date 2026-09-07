package data

import (
	"testing"

	"go-stock/backend/db"

	"github.com/stretchr/testify/require"
)

// Phase2-C PR1 防回退：身份与 OMS 语义冻结（不改 Submit/Fill/Cancel 行为）。

func TestPhase2C_PaperIdentityFreeze_NoBrokerIDs(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sz000001",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  true,
	})
	require.NoError(t, err)
	require.NotNil(t, order)

	require.Equal(t, PaperExecBackendPaper, order.ExecBackend)
	require.Equal(t, PaperBrokerStatusPaper, order.BrokerStatus)
	require.Empty(t, order.BrokerOrderID, "Paper 不得生成 broker_order_id")
	require.Empty(t, order.ExternalOrderID, "Paper 不得生成 external_order_id")
	require.True(t, IsPaperClientOrderIDFormat(order.ClientOrderID),
		"client_order_id 格式须稳定为 paper_<32hex>，got %q", order.ClientOrderID)
	require.True(t, IsPaperOMSStatus(order.Status))
	require.NotEqual(t, BrokerStatusAccepted, order.Status)
	require.NotEqual(t, "accepted", order.Status)

	var dbOrder PaperOrder
	require.NoError(t, db.Dao.First(&dbOrder, order.ID).Error)
	require.Equal(t, PaperExecBackendPaper, dbOrder.ExecBackend)
	require.Equal(t, PaperBrokerStatusPaper, dbOrder.BrokerStatus)
	require.Empty(t, dbOrder.BrokerOrderID)
	require.Empty(t, dbOrder.ExternalOrderID)
	require.True(t, IsPaperClientOrderIDFormat(dbOrder.ClientOrderID))
}

func TestPhase2C_ClientOrderIDFormatStable(t *testing.T) {
	require.True(t, IsPaperClientOrderIDFormat(PaperClientOrderIDPrefix+"0123456789abcdef0123456789abcdef"))
	require.False(t, IsPaperClientOrderIDFormat("paper_short"))
	require.False(t, IsPaperClientOrderIDFormat("real_0123456789abcdef0123456789abcdef"))
	require.False(t, IsPaperClientOrderIDFormat(PaperClientOrderIDPrefix+"0123456789ABCDEF0123456789ABCDEF")) // 须小写
	require.False(t, IsPaperClientOrderIDFormat(""))

	for i := 0; i < 20; i++ {
		id := newPaperClientOrderID()
		require.True(t, IsPaperClientOrderIDFormat(id), "generated %q", id)
	}
}

func TestPhase2C_OMSStatusFrozen_NoAccepted(t *testing.T) {
	for _, s := range PaperOMSStatuses {
		require.True(t, IsPaperOMSStatus(s))
		require.NotEqual(t, "accepted", s)
	}
	require.False(t, IsPaperOMSStatus("accepted"))
	require.False(t, IsPaperOMSStatus(BrokerStatusAccepted))
	require.False(t, IsPaperOMSStatus(BrokerStatusWorking))
	require.False(t, IsPaperOMSStatus(BrokerStatusPartiallyFilled))
	require.False(t, IsPaperOMSStatus(PaperOrderStatusProcessing))
}

func TestPhase2C_TradeOrderMappingPreservesIdentityFreeze(t *testing.T) {
	setupPaperTradingTestDB(t)
	api := NewPaperTradingApi()
	account, err := api.ResetAccount(100_000)
	require.NoError(t, err)

	order, err := api.SubmitPaperOrder(PaperSubmitOrderReq{
		AccountID: account.ID,
		StockCode: "sh600000",
		Side:      "buy",
		Price:     10,
		Volume:    100,
		AutoFill:  false,
	})
	require.NoError(t, err)
	require.Equal(t, PaperOrderStatusPending, order.Status)

	mapped := toTradeOrder(*order)
	require.Equal(t, order.ClientOrderID, mapped.ClientOrderID)
	require.Equal(t, PaperExecBackendPaper, mapped.ExecBackend)
	require.Equal(t, PaperBrokerStatusPaper, mapped.BrokerStatus)
	require.Empty(t, mapped.BrokerOrderID)
	require.Empty(t, mapped.ExternalOrderID)
	require.True(t, IsPaperClientOrderIDFormat(mapped.ClientOrderID))
	require.True(t, IsPaperOMSStatus(mapped.Status))
}
