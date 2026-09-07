package strategy

import (
	"testing"

	"go-stock/backend/models"
	"go-stock/backend/tradingrule"

	"github.com/stretchr/testify/require"
)

func TestIntentVolumeMeetsLot_FlagOff_Legacy100(t *testing.T) {
	tradingrule.ResetEnableQuantityPolicyForTest()
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	require.True(t, intentVolumeMeetsLot("sz000001", 100))
	require.True(t, intentVolumeMeetsLot("sh688981", 199))
	require.False(t, intentVolumeMeetsLot("sh113052", 10)) // CB 10 < morningLotSize 100
}

func TestIntentVolumeMeetsLot_FlagOn_BoardMatrix(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	require.True(t, intentVolumeMeetsLot("sz000001", 100))
	require.False(t, intentVolumeMeetsLot("sz000001", 101))
	require.False(t, intentVolumeMeetsLot("sh688981", 199))
	require.True(t, intentVolumeMeetsLot("sh688981", 200))
	require.True(t, intentVolumeMeetsLot("sh688981", 201))
	require.True(t, intentVolumeMeetsLot("sh510300", 100))
	require.False(t, intentVolumeMeetsLot("sh510300", 99))
	require.False(t, intentVolumeMeetsLot("sh113052", 9))
	require.True(t, intentVolumeMeetsLot("sh113052", 10))
	require.False(t, intentVolumeMeetsLot("sh113052", 11))
}

func TestCountFullyMaterializedItems_FlagOn(t *testing.T) {
	tradingrule.SetEnableQuantityPolicyForTest(true)
	t.Cleanup(tradingrule.ResetEnableQuantityPolicyForTest)

	plan := &models.TradePlan{Items: []models.TradePlanItem{
		{StockCode: "sz000001", Side: "buy", IntentStatus: morningIntentStatusPriced, LimitPrice: 1, TargetVolume: 100},
		{StockCode: "sh688981", Side: "buy", IntentStatus: morningIntentStatusPriced, LimitPrice: 1, TargetVolume: 201},
		{StockCode: "sh510300", Side: "buy", IntentStatus: morningIntentStatusPriced, LimitPrice: 1, TargetVolume: 100},
		{StockCode: "sh113052", Side: "buy", IntentStatus: morningIntentStatusPriced, LimitPrice: 1, TargetVolume: 10},
		{StockCode: "sh688001", Side: "buy", IntentStatus: morningIntentStatusPriced, LimitPrice: 1, TargetVolume: 199}, // not full
	}}
	require.Equal(t, 4, countFullyMaterializedItems(plan))
}
