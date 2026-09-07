package opportunity_test

import (
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/opportunity"

	"github.com/stretchr/testify/require"
)

func TestListWatching_OnlyLatestWATCH(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	batch := "2026-09-05|close|default"
	oid := opportunity.BuildOpportunityID(batch, "600363.SH", "2026-09-05", "强")

	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionWatch,
	})
	require.NoError(t, err)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Equal(t, 1, view.WatchingCount)
	require.Len(t, view.Items, 1)
	require.Equal(t, "sh600363", view.Items[0].StockCode)
	require.Equal(t, opportunity.ActionWatch, view.Items[0].LatestAction)
	require.False(t, view.Items[0].InTradePlan)
	require.Equal(t, opportunity.WatchlistTradePlanNone, view.Items[0].TradePlanStatus)
	require.Contains(t, view.Items[0].Source, "2026-09-05")

	_, err = opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:     acc.ID,
		ScanBatchKey:  batch,
		OpportunityID: oid,
		StockCode:     "sh600363",
		Action:        opportunity.ActionIgnore,
	})
	require.NoError(t, err)

	view2, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Equal(t, 0, view2.WatchingCount)
	require.Empty(t, view2.Items)
}

func TestListWatching_Empty(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Equal(t, 0, view.WatchingCount)
	require.Empty(t, view.Items)
}

func TestListWatching_EnrichFromSnapshot(t *testing.T) {
	acc := setupOpportunityTestDB(t)
	require.NoError(t, db.Dao.AutoMigrate(&models.SignalScanSnapshot{}))
	snap := &models.SignalScanSnapshot{
		TradeDate: "2026-09-05", Session: "close", StrategyID: "default",
		Status: "done", HitTotal: 1,
		ResultJSON: `{"items":[{"SECUCODE":"300620.SZ","SECURITY_NAME_ABBR":"光库科技","Tag":"突","SignalTime":"2026-09-05"}],"hitTotal":1}`,
	}
	require.NoError(t, db.Dao.Create(snap).Error)
	batch := opportunity.BatchKeyFromSnapshot(snap.ID, snap.TradeDate, snap.Session, snap.StrategyID)

	_, err := opportunity.SaveUserOpportunityAction(opportunity.SaveUserOpportunityActionInput{
		AccountID:    acc.ID,
		ScanBatchKey: batch,
		Secucode:     "300620.SZ",
		SignalTime:   "2026-09-05",
		SignalTag:    "突",
		Action:       opportunity.ActionWatch,
	})
	require.NoError(t, err)

	// ensure watch_time ordering stable
	time.Sleep(5 * time.Millisecond)

	view, err := opportunity.ListWatching(opportunity.ListWatchingQuery{AccountID: acc.ID})
	require.NoError(t, err)
	require.Equal(t, 1, view.WatchingCount)
	require.Equal(t, "光库科技", view.Items[0].StockName)
	require.Contains(t, view.Items[0].Source, "2026-09-05")
}
