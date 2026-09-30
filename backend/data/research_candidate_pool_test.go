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

// Phase16.15-E: later universe empty snapshot must not override earlier scope=all.
func TestListResearchCandidates_IgnoresUniverseEmptySnapshot(t *testing.T) {
	setupSignalScanListTestDB(t)
	days0 := 0
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "600000.SH", SECURITY_CODE: "600000", SECURITY_NAME_ABBR: "浦发", Tag: "强", DaysAgo: &days0, RSI: 28, NEW_PRICE: "10"},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	allSnap := models.SignalScanSnapshot{
		CreatedAt:  time.Date(2026, 9, 3, 20, 12, 0, 0, time.Local),
		TradeDate:  "2026-09-03",
		Session:    "close",
		Scope:      models.SignalScanScopeAll,
		Status:     "done",
		HitTotal:   1,
		ResultJSON: string(raw),
	}
	require.NoError(t, db.Dao.Create(&allSnap).Error)

	universeEmpty := models.SignalScanSnapshot{
		CreatedAt:  time.Date(2026, 9, 4, 0, 19, 0, 0, time.Local),
		TradeDate:  "2026-09-02",
		Session:    "close",
		Scope:      models.SignalScanScopeUniverse,
		Status:     "done",
		HitTotal:   0,
		ResultJSON: `{"items":[],"hitTotal":0}`,
	}
	require.NoError(t, db.Dao.Create(&universeEmpty).Error)

	list := ListResearchCandidatesFromLatestSnapshot(60)
	require.NotNil(t, list)
	require.Equal(t, allSnap.ID, list.SnapshotID)
	require.Equal(t, "2026-09-03", list.TradeDate)
	require.Equal(t, 1, list.ItemCount)
	require.Equal(t, "sh600000", list.Items[0].StockCode)

	byDate := ListResearchCandidatesForTradeDate("2026-09-02", 60)
	require.NotNil(t, byDate)
	require.Equal(t, uint(0), byDate.SnapshotID)
	require.Equal(t, 0, byDate.ItemCount)
	require.Contains(t, byDate.Message, "universe")
}

func TestListResearchCandidates_AllowsLegacyEmptyScope(t *testing.T) {
	setupSignalScanListTestDB(t)
	days0 := 0
	payload := models.SignalScanResultPayload{
		Items: []models.SignalScanHit{
			{SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安", Tag: "强", DaysAgo: &days0, RSI: 28, NEW_PRICE: "11"},
		},
		HitTotal: 1,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Now(),
		TradeDate:  "2099-04-01",
		Session:    "close",
		Scope:      "", // legacy
		Status:     "done",
		HitTotal:   1,
		ResultJSON: string(raw),
	}).Error)

	list := ListResearchCandidatesFromLatestSnapshot(60)
	require.Equal(t, 1, list.ItemCount)
	require.Equal(t, "sz000001", list.Items[0].StockCode)
}

func TestListResearchStrategyCandidates_MixedStrategiesFailClosed(t *testing.T) {
	setupSignalScanListTestDB(t)
	days0 := 0
	mk := func(code, name, tag string) string {
		payload := models.SignalScanResultPayload{
			Items: []models.SignalScanHit{
				{SECUCODE: code, SECURITY_CODE: code, SECURITY_NAME_ABBR: name, Tag: tag, DaysAgo: &days0, RSI: 28, NEW_PRICE: "10"},
			},
			HitTotal: 1,
		}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		return string(raw)
	}

	day := "2099-06-01"
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:    time.Date(2099, 6, 1, 9, 0, 0, 0, time.UTC),
		TradeDate:    day,
		Session:      "close",
		Scope:        models.SignalScanScopeAll,
		StrategyID:   "ext_xsmom_v1",
		StrategyName: "旧截面",
		Status:       "done",
		HitTotal:     1,
		ResultJSON:   mk("600000.SH", "旧行", "强"),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:    time.Date(2099, 6, 1, 10, 0, 0, 0, time.UTC),
		TradeDate:    day,
		Session:      "close",
		Scope:        models.SignalScanScopeAll,
		StrategyID:   "ext_xsmom_v1",
		StrategyName: "截面动量V1",
		Status:       "done",
		HitTotal:     1,
		ResultJSON:   mk("600000.SH", "浦发", "强"),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:    time.Date(2099, 6, 1, 11, 0, 0, 0, time.UTC),
		TradeDate:    day,
		Session:      "close",
		Scope:        models.SignalScanScopeAll,
		StrategyID:   "alpha_v1",
		StrategyName: "",
		Status:       "done",
		HitTotal:     1,
		ResultJSON:   mk("000001.SZ", "平安", "趋"),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Date(2099, 6, 1, 10, 30, 0, 0, time.UTC),
		TradeDate:  day,
		Session:    "close",
		Scope:      models.SignalScanScopeAll,
		StrategyID: "   ",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: mk("300001.SZ", "特锐德", "强"),
	}).Error)
	// 更早交易日不进入「最新研究日」的合并结果。
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Date(2099, 6, 1, 8, 0, 0, 0, time.UTC),
		TradeDate:  "2099-05-01",
		Session:    "close",
		Scope:      models.SignalScanScopeAll,
		StrategyID: "other_day",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: mk("601398.SH", "工行", "强"),
	}).Error)
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:  time.Date(2099, 6, 1, 12, 0, 0, 0, time.UTC),
		TradeDate:  day,
		Session:    "close",
		Scope:      models.SignalScanScopeUniverse,
		StrategyID: "universe_only",
		Status:     "done",
		HitTotal:   1,
		ResultJSON: mk("601988.SH", "中行", "强"),
	}).Error)

	list := ListResearchStrategyCandidates("", 60)
	require.Equal(t, day, list.TradeDate)
	require.Equal(t, "", list.StrategyName)
	require.Equal(t, 3, list.ItemCount)

	byKey := map[string]models.ResearchSnapshotCandidate{}
	for _, it := range list.Items {
		byKey[it.StrategyID+"|"+it.StockCode] = it
	}
	mom := byKey["ext_xsmom_v1|sh600000"]
	require.Equal(t, "浦发", mom.StockName)
	require.Equal(t, "截面动量V1", mom.StrategyName)
	require.NotZero(t, mom.SnapshotID)

	alpha := byKey["alpha_v1|sz000001"]
	require.Equal(t, "alpha_v1", alpha.StrategyID)
	require.Equal(t, "", alpha.StrategyName)

	unlabeled := byKey["|sz300001"]
	require.Equal(t, "", unlabeled.StrategyID)
	require.Equal(t, "", unlabeled.StrategyName)
	_, leaked := byKey["other_day|sh601398"]
	require.False(t, leaked)
	_, universe := byKey["universe_only|sh601988"]
	require.False(t, universe)

	// Same stock under two strategies stays two rows; ids are not collapsed.
	require.NoError(t, db.Dao.Create(&models.SignalScanSnapshot{
		CreatedAt:    time.Date(2099, 6, 1, 11, 30, 0, 0, time.UTC),
		TradeDate:    day,
		Session:      "close",
		Scope:        models.SignalScanScopeAll,
		StrategyID:   "beta_v1",
		StrategyName: "Beta",
		Status:       "done",
		HitTotal:     1,
		ResultJSON:   mk("600000.SH", "浦发乙", "强"),
	}).Error)
	again := ListResearchStrategyCandidates(day, 60)
	count600 := 0
	for _, it := range again.Items {
		if it.StockCode == "sh600000" {
			count600++
		}
	}
	require.Equal(t, 2, count600)
}
