package data

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"go-stock/backend/db"
	"go-stock/backend/models"
	"go-stock/backend/signalattribution"
)

func attributionTestDB(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("file:attr_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db.Init(name)
	if err := db.Dao.AutoMigrate(&models.SignalScanSnapshot{}, &KLineCacheRecord{}); err != nil {
		t.Fatal(err)
	}
	if err := MigrateStockKLineTables(db.Dao); err != nil {
		t.Fatal(err)
	}
}

func insertAttributionSnapshot(t *testing.T, strategyID, strategyName, status string, hits []models.SignalScanHit) models.SignalScanSnapshot {
	t.Helper()
	raw, err := json.Marshal(models.SignalScanResultPayload{
		Items:        hits,
		TradeDate:    "2026-07-17",
		Session:      "close",
		StrategyID:   strategyID,
		StrategyName: strategyName,
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := models.SignalScanSnapshot{
		TradeDate:    "2026-07-17",
		Session:      "close",
		StrategyID:   strategyID,
		StrategyName: strategyName,
		Status:       status,
		HitTotal:     len(hits),
		ResultJSON:   string(raw),
	}
	if err := db.Dao.Create(&snap).Error; err != nil {
		t.Fatal(err)
	}
	return snap
}

func horizonCell(row signalattribution.HitRow, h int) signalattribution.HorizonCell {
	for _, c := range row.Horizons {
		if c.Horizon == h {
			return c
		}
	}
	t := signalattribution.HorizonCell{}
	return t
}

func TestSignalScanAttribution_LocalCacheAndFailClosed(t *testing.T) {
	attributionTestDB(t)
	bars := []KLineData{
		{Day: "2026-07-17", Open: "10", Close: "10", High: "10", Low: "10"},
		{Day: "2026-07-20", Open: "11", Close: "11", High: "11", Low: "11"},
	}
	klineCachePut("1.600000", "101", "", "latest", len(bars), &bars)

	snap := insertAttributionSnapshot(t, "alpha", "测试策略", "done", []models.SignalScanHit{{
		SECUCODE:           "600000.SH",
		SECURITY_CODE:      "600000",
		SECURITY_NAME_ABBR: "浦发银行",
		NEW_PRICE:          "1",
	}})
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	if !view.OK || len(view.Rows) != 1 {
		t.Fatalf("%+v", view)
	}
	row := view.Rows[0]
	if row.Code != "600000" || row.Name != "浦发银行" || row.StrategyID != "alpha" || row.AsOfDate != "2026-07-17" {
		t.Fatalf("row %+v", row)
	}
	if row.PriceBasis != signalattribution.BasisLocalClose || row.CloseText != "10.00" {
		t.Fatalf("close should ignore snapshot price 1, got %+v", row)
	}
	h1 := horizonCell(row, 1)
	if h1.Status != signalattribution.StatusOK || h1.Text != "+10.00%" || h1.FutureDate != "2026-07-20" {
		t.Fatalf("+1 %+v", h1)
	}
	h3 := horizonCell(row, 3)
	if h3.Status != signalattribution.StatusInsufficient || h3.Text != signalattribution.TextInsufficient || h3.ReturnRate != nil {
		t.Fatalf("+3 %+v", h3)
	}
	if view.Summary.HitCount != 1 || view.Summary.CompleteAll != 0 {
		t.Fatalf("summary %+v", view.Summary)
	}
	if view.Disclaimer != signalattribution.Disclaimer {
		t.Fatal(view.Disclaimer)
	}
}

func TestSignalScanAttribution_FormalBarsOverrideCache(t *testing.T) {
	attributionTestDB(t)
	cached := []KLineData{
		{Day: "2026-07-17", Close: "10"},
		{Day: "2026-07-20", Close: "11"},
	}
	klineCachePut("1.600000", "101", "", "latest", len(cached), &cached)
	repo := NewStockKLineRepo()
	loc := time.Local
	_, err := repo.UpsertBars([]KLineBar{
		{Market: MarketCN, TSCode: "600000.SH", Symbol: "600000", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
			BarTime: time.Date(2026, 7, 17, 0, 0, 0, 0, loc), Open: 20, High: 20, Low: 20, Close: 20, Volume: 1, Source: "test"},
		{Market: MarketCN, TSCode: "600000.SH", Symbol: "600000", Period: KLinePeriod1D, AdjustType: KLineAdjustNone,
			BarTime: time.Date(2026, 7, 20, 0, 0, 0, 0, loc), Open: 22, High: 22, Low: 22, Close: 22, Volume: 1, Source: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := insertAttributionSnapshot(t, "alpha", "测试策略", "done", []models.SignalScanHit{{
		SECUCODE: "600000.SH", SECURITY_CODE: "600000", SECURITY_NAME_ABBR: "浦发银行",
	}})
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	row := view.Rows[0]
	if row.CloseText != "20.00" || row.PriceBasis != signalattribution.BasisLocalClose {
		t.Fatalf("formal close should win: %+v", row)
	}
	h1 := horizonCell(row, 1)
	if h1.Text != "+10.00%" || h1.FutureClose == nil || *h1.FutureClose != 22 {
		t.Fatalf("formal +1 %+v", h1)
	}
}

func TestSignalScanAttribution_SnapshotPriceWhenBarsMissing(t *testing.T) {
	attributionTestDB(t)
	snap := insertAttributionSnapshot(t, "alpha", "测试策略", "done", []models.SignalScanHit{{
		SECUCODE: "600000.SH", SECURITY_CODE: "600000", NEW_PRICE: "9.5", SignalPrice: 8,
	}})
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	row := view.Rows[0]
	if row.PriceBasis != signalattribution.BasisSnapshot || row.CloseText != "8.00" {
		t.Fatalf("%+v", row)
	}
	if horizonCell(row, 1).Text != signalattribution.TextInsufficient || horizonCell(row, 1).ReturnRate != nil {
		t.Fatalf("%+v", horizonCell(row, 1))
	}
}

func TestSignalScanAttribution_StrategyAndUnfinished(t *testing.T) {
	attributionTestDB(t)
	insertAttributionSnapshot(t, "alpha", "甲", "done", []models.SignalScanHit{{
		SECUCODE: "600000.SH", SECURITY_CODE: "600000",
	}})
	insertAttributionSnapshot(t, "beta", "乙", "done", []models.SignalScanHit{{
		SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行",
	}})
	view := BuildSignalScanAttribution(&signalattribution.Query{TradeDate: "2026-07-17", StrategyID: "beta"})
	if !view.OK || view.StrategyID != "beta" || len(view.Rows) != 1 || view.Rows[0].Code != "000001" {
		t.Fatalf("%+v rows=%v", view, view.Rows)
	}
	running := insertAttributionSnapshot(t, "alpha", "甲", "running", []models.SignalScanHit{{
		SECUCODE: "600000.SH",
	}})
	blocked := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: running.ID})
	if blocked.OK || !strings.Contains(blocked.Message, "尚未完成") {
		t.Fatalf("%+v", blocked)
	}
}

func TestSignalScanAttribution_ConflictingCacheClosesFailClosed(t *testing.T) {
	attributionTestDB(t)
	latest := []KLineData{
		{Day: "2026-07-17", Close: "10"},
		{Day: "2026-07-20", Close: "11"},
	}
	other := []KLineData{
		{Day: "2026-07-17", Close: "99"},
		{Day: "2026-07-20", Close: "11"},
	}
	klineCachePut("1.600000", "101", "", "latest", len(latest), &latest)
	klineCachePut("1.600000", "101", "", "20260720", len(other), &other)
	snap := insertAttributionSnapshot(t, "alpha", "测试策略", "done", []models.SignalScanHit{{
		SECUCODE: "600000.SH", SECURITY_CODE: "600000", NEW_PRICE: "10",
	}})
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	row := view.Rows[0]
	if row.PriceBasis != signalattribution.BasisSnapshot || row.CloseText != "10.00" {
		t.Fatalf("conflicting as-of close must not be picked: %+v", row)
	}
	h1 := horizonCell(row, 1)
	if h1.Status != signalattribution.StatusOK || h1.Text != "+10.00%" || h1.FutureClose == nil || *h1.FutureClose != 11 {
		t.Fatalf("agreed later close should still count: %+v", h1)
	}
}

func TestSignalScanAttribution_WeekdayHoleInCache(t *testing.T) {
	attributionTestDB(t)
	bars := []KLineData{
		{Day: "2026-07-13", Close: "100"},
		{Day: "2026-07-14", Close: "110"},
		{Day: "2026-07-16", Close: "120"},
	}
	klineCachePut("1.600000", "101", "", "latest", len(bars), &bars)
	raw, err := json.Marshal(models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{SECUCODE: "600000.SH", SECURITY_CODE: "600000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := models.SignalScanSnapshot{
		TradeDate: "2026-07-13", Session: "close", StrategyID: "alpha", Status: "done", ResultJSON: string(raw),
	}
	if err := db.Dao.Create(&snap).Error; err != nil {
		t.Fatal(err)
	}
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	h1 := horizonCell(view.Rows[0], 1)
	h3 := horizonCell(view.Rows[0], 3)
	if h1.Status != signalattribution.StatusOK || h1.Text != "+10.00%" {
		t.Fatalf("+1 %+v", h1)
	}
	if h3.Status != signalattribution.StatusInsufficient || h3.ReturnRate != nil || h3.Reason != signalattribution.ReasonCalendarGap {
		t.Fatalf("hole +3 %+v", h3)
	}
	if view.Rows[0].ToDate.Status != signalattribution.StatusInsufficient || view.Rows[0].ToDate.Reason != signalattribution.ReasonCalendarGap {
		t.Fatalf("迄今 must fail closed on the hole: %+v", view.Rows[0].ToDate)
	}
}

func TestSignalScanAttribution_BlankStrategyFallsBack(t *testing.T) {
	attributionTestDB(t)
	raw, err := json.Marshal(models.SignalScanResultPayload{
		Items:        []models.SignalScanHit{{SECUCODE: "600000.SH", SECURITY_CODE: "600000", SECURITY_NAME_ABBR: "浦发银行"}},
		StrategyName: "结果里的策略",
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := models.SignalScanSnapshot{
		TradeDate: "2026-07-17", Session: "close", Status: "done", ResultJSON: string(raw),
	}
	if err := db.Dao.Create(&snap).Error; err != nil {
		t.Fatal(err)
	}
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	if view.StrategyID != "default" || view.StrategyName != "结果里的策略" {
		t.Fatalf("strategy id/name %+v / %+v", view.StrategyID, view.StrategyName)
	}
	if len(view.Rows) != 1 || view.Rows[0].StrategyID != "default" || view.Rows[0].StrategyName != "结果里的策略" {
		t.Fatalf("row strategy %+v", view.Rows[0])
	}
	if view.Rows[0].KlineCode == "" {
		t.Fatal("kline code should follow the normalized bar key")
	}
}

func TestSignalScanAttribution_LegacyBlankStrategyID(t *testing.T) {
	attributionTestDB(t)
	raw, err := json.Marshal(models.SignalScanResultPayload{
		Items: []models.SignalScanHit{{SECUCODE: "600000.SH", SECURITY_CODE: "600000"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := models.SignalScanSnapshot{
		TradeDate: "2026-07-17", Session: "close", Status: "done", ResultJSON: string(raw),
	}
	if err := db.Dao.Create(&snap).Error; err != nil {
		t.Fatal(err)
	}
	view := BuildSignalScanAttribution(&signalattribution.Query{SnapshotID: snap.ID})
	if view.StrategyID != "default" || view.StrategyName != "" {
		t.Fatalf("legacy blank id becomes default without inventing a name: %+v %q", view.StrategyID, view.StrategyName)
	}
}

func TestSignalScanAttribution_SignalTagFiltersCohort(t *testing.T) {
	attributionTestDB(t)
	bars := []KLineData{
		{Day: "2026-07-17", Close: "10"},
		{Day: "2026-07-20", Close: "11"},
	}
	klineCachePut("1.600000", "101", "", "latest", len(bars), &bars)
	klineCachePut("0.000001", "101", "", "latest", len(bars), &bars)
	snap := insertAttributionSnapshot(t, "default", "", "done", []models.SignalScanHit{
		{SECUCODE: "600000.SH", SECURITY_CODE: "600000", SECURITY_NAME_ABBR: "浦发银行", Tag: "强", INDUSTRY: "银行"},
		{SECUCODE: "000001.SZ", SECURITY_CODE: "000001", SECURITY_NAME_ABBR: "平安银行", Tag: "趋", INDUSTRY: "银行"},
	})
	view := BuildSignalScanAttribution(&signalattribution.Query{
		SnapshotID: snap.ID,
		SignalTags: []string{"强"},
	})
	if !view.OK || view.Summary.HitCount != 1 || view.Summary.SnapshotHitCount != 2 || len(view.Rows) != 1 {
		t.Fatalf("filtered summary %+v rows %d", view.Summary, len(view.Rows))
	}
	if view.Rows[0].Code != "600000" || view.Rows[0].Tag != "强" {
		t.Fatalf("row %+v", view.Rows[0])
	}
	if view.Cohort.Up.Count+view.Cohort.Down.Count+view.Cohort.Flat.Count+view.Cohort.Excluded != 1 {
		t.Fatalf("cohort must use the filtered subset: %+v", view.Cohort)
	}
}
