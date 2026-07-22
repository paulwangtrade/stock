package data

import (
	"encoding/json"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"

	"github.com/stretchr/testify/require"
)

func TestCalcResearchSignalScore_StrongToday(t *testing.T) {
	score := CalcResearchSignalScore("强", nil, 28)
	require.Equal(t, 97.0, score) // 92 + 5
}

func TestCalcResearchSignalScore_BelowThreshold(t *testing.T) {
	days := 5
	score := CalcResearchSignalScore("冰", &days, 40)
	require.True(t, score <= 60, "ice with lag should be <=60, got %v", score)
}

func TestPredictDirectionFromTag(t *testing.T) {
	require.Equal(t, "看多", PredictDirectionFromTag("强"))
	require.Equal(t, "看空", PredictDirectionFromTag("卖"))
	require.Equal(t, "中性", PredictDirectionFromTag(""))
}

func TestNormalizeFollowableCode(t *testing.T) {
	require.Equal(t, "sh600000", normalizeFollowableCode("600000.SH", "600000"))
	require.Equal(t, "sz000001", normalizeFollowableCode("000001.SZ", ""))
}

func TestListResearchCandidates_FiltersAndSorts(t *testing.T) {
	setupSignalScanListTestDB(t)
	days0 := 0
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "600000.SH", SECURITY_CODE: "600000", SECURITY_NAME_ABBR: "浦发", Tag: "强", DaysAgo: &days0, RSI: 28, NEW_PRICE: "10"},
			{SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安", Tag: "冰", DaysAgo: &days0, RSI: 40, NEW_PRICE: "11"},
			{SECUCODE: "300001.SZ", SECURITY_CODE: "300001", SECURITY_NAME_ABBR: "特锐德", Tag: "趋", DaysAgo: &days0, RSI: 32, NEW_PRICE: "12"},
		},
		HitTotal: 3,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-03-01",
		Session:    "close",
		Status:     "done",
		HitTotal:   3,
		ResultJSON: string(raw),
	}).Error)

	list := ListResearchCandidatesFromLatestSnapshot(60)
	require.NotNil(t, list)
	require.GreaterOrEqual(t, list.ItemCount, 2)
	require.Equal(t, "sh600000", list.Items[0].StockCode)
	for i := 1; i < len(list.Items); i++ {
		require.GreaterOrEqual(t, list.Items[i-1].SignalScore, list.Items[i].SignalScore)
	}
	for _, it := range list.Items {
		require.Greater(t, it.SignalScore, 60.0)
	}
}

func TestListResearchCandidates_MapFormat(t *testing.T) {
	setupSignalScanListTestDB(t)
	raw := `{
		"600036": {"name": "招商银行", "score": 82.3, "direction": "买入", "reason": "资金净流入"},
		"000001": {"name": "平安", "score": 40, "direction": "中性", "reason": "低分"}
	}`
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-03-02",
		Session:    "close",
		Status:     "done",
		HitTotal:   2,
		ResultJSON: raw,
	}).Error)

	list := ListResearchCandidatesFromLatestSnapshot(60)
	require.Equal(t, 1, list.ItemCount)
	require.Equal(t, "sh600036", list.Items[0].StockCode)
	require.InDelta(t, 82.3, list.Items[0].SignalScore, 0.01)
	require.Equal(t, "买入", list.Items[0].Direction)
}

func TestGetCandidatePoolScoreThreshold_Default(t *testing.T) {
	setupSettingsCacheTestDB(t)
	require.Equal(t, 60.0, GetCandidatePoolScoreThreshold())
}
