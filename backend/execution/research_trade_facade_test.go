package execution

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go-stock/backend/broker"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupResearchFacadeTestDB(t *testing.T) {
	t.Helper()
	original := db.Dao
	dsn := fmt.Sprintf("file:research_facade_%s?mode=memory&cache=shared", t.Name())
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	require.NoError(t, err)
	sqlDB, err := testDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	db.Dao = testDB
	require.NoError(t, data.MigratePaperTrading(testDB))
	require.NoError(t, data.EnsureResearchTradeIntentTables())
	t.Cleanup(func() {
		db.Dao = original
		_ = sqlDB.Close()
	})
}

func mustConfirmedResearchIntent(t *testing.T, patch *models.ResearchTradeIntent) *models.ResearchTradeIntent {
	t.Helper()
	repo := data.NewResearchTradeIntentRepo()
	intent := &models.ResearchTradeIntent{
		Symbol:              "sh600036",
		StockName:           "招商银行",
		Side:                "buy",
		Price:               10,
		Volume:              100,
		CandidateSnapshotID: 42,
		SignalScore:         97,
		SignalTag:           "强",
		Reason:              "测试买入",
	}
	if patch != nil {
		if patch.Symbol != "" {
			intent.Symbol = patch.Symbol
		}
		if patch.Price > 0 {
			intent.Price = patch.Price
		}
		if patch.Volume > 0 {
			intent.Volume = patch.Volume
		}
		if patch.AccountID > 0 {
			intent.AccountID = patch.AccountID
		}
	}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusConfirmed, nil))
	got, err := repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	return got
}

func TestResearchTradeFacade_ConfirmedExecutesViaPort(t *testing.T) {
	setupResearchFacadeTestDB(t)
	intent := mustConfirmedResearchIntent(t, nil)

	port := &stubPort{
		order: &broker.TradeOrder{
			ID:            "77",
			ClientOrderID: "cli-research-1",
			Status:        data.PaperOrderStatusFilled,
			StockCode:     intent.Symbol,
			StrategyTag:   models.ResearchTradeStrategyTag,
		},
	}
	svc := NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewResearchTradeFacade(svc, nil)

	require.NoError(t, facade.ConfirmAndExecuteResearchBuy(context.Background(), intent.ID))
	require.Equal(t, 1, port.calls)
	require.Equal(t, intent.Symbol, port.lastIntent.StockCode)
	require.Equal(t, models.ResearchTradeStrategyTag, port.lastIntent.StrategyTag)
	require.Contains(t, port.lastIntent.Reason, "research_source=")
	require.Contains(t, port.lastIntent.Reason, "snapshot_id=42")
	require.Contains(t, port.lastIntent.Reason, "signal_tag=强")
	require.True(t, port.lastIntent.AutoFill)

	got, err := data.NewResearchTradeIntentRepo().GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusSubmitted, got.Status)
	require.Equal(t, "cli-research-1", got.ClientOrderID)
	require.Equal(t, uint(77), got.OrderID)
}

func TestResearchTradeFacade_DraftNotExecutable(t *testing.T) {
	setupResearchFacadeTestDB(t)
	repo := data.NewResearchTradeIntentRepo()
	intent := &models.ResearchTradeIntent{Symbol: "sz000001", Price: 10, Volume: 100}
	require.NoError(t, repo.CreateResearchTradeIntent(intent))

	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	svc := NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewResearchTradeFacade(svc, nil)

	err := facade.ConfirmAndExecuteResearchBuy(context.Background(), intent.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrResearchTradeIntentNotExecutable))
	require.Equal(t, 0, port.calls)

	got, err := repo.GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusDraft, got.Status)
}

func TestResearchTradeFacade_RejectedNotExecutable(t *testing.T) {
	setupResearchFacadeTestDB(t)
	intent := mustConfirmedResearchIntent(t, nil)
	repo := data.NewResearchTradeIntentRepo()
	require.NoError(t, repo.UpdateResearchTradeIntentStatus(intent.ID, models.ResearchTradeIntentStatusRejected, &data.ResearchTradeIntentStatusPatch{
		ErrorCode:    "manual",
		ErrorMessage: "pre-rejected",
	}))

	port := &stubPort{order: &broker.TradeOrder{ID: "1"}}
	svc := NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewResearchTradeFacade(svc, nil)

	err := facade.ConfirmAndExecuteResearchBuy(context.Background(), intent.ID)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrResearchTradeIntentNotExecutable))
	require.Equal(t, 0, port.calls)
}

func TestResearchTradeFacade_ExecutionFailureRejectsIntent(t *testing.T) {
	setupResearchFacadeTestDB(t)
	intent := mustConfirmedResearchIntent(t, nil)

	port := &stubPort{
		err: preTradeReject(data.PaperOrderRejectCashInsufficient, "cash insufficient from port"),
	}
	svc := NewExecutionService(port).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewResearchTradeFacade(svc, nil)

	err := facade.ConfirmAndExecuteResearchBuy(context.Background(), intent.ID)
	require.Error(t, err)
	require.Equal(t, 1, port.calls)

	got, err := data.NewResearchTradeIntentRepo().GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusRejected, got.Status)
	require.Equal(t, data.PaperOrderRejectCashInsufficient, got.ErrorCode)
	require.Contains(t, got.ErrorMessage, "cash insufficient")
}

func TestResearchTradeFacade_PaperOrderViaExecutionNotDirectAPI(t *testing.T) {
	setupResearchFacadeTestDB(t)
	acc, err := data.NewPaperTradingApi().GetOrCreateDefaultAccount(0)
	require.NoError(t, err)

	intent := mustConfirmedResearchIntent(t, &models.ResearchTradeIntent{
		AccountID: acc.ID,
		Price:     10,
		Volume:    100,
	})

	svc := NewExecutionService(NewPaperBroker(nil)).withSnapshotLoader(richCashLoader(1_000_000))
	facade := NewResearchTradeFacade(svc, nil)
	require.NoError(t, facade.ConfirmAndExecuteResearchBuy(context.Background(), intent.ID))

	got, err := data.NewResearchTradeIntentRepo().GetResearchTradeIntent(intent.ID)
	require.NoError(t, err)
	require.Equal(t, models.ResearchTradeIntentStatusSubmitted, got.Status)
	require.NotZero(t, got.OrderID)
	require.NotEmpty(t, got.ClientOrderID)

	var order data.PaperOrder
	require.NoError(t, db.Dao.First(&order, got.OrderID).Error)
	require.Equal(t, intent.Symbol, order.StockCode)
	require.Equal(t, models.ResearchTradeStrategyTag, order.StrategyTag)
	require.Contains(t, order.Reason, "snapshot_id=")
}
