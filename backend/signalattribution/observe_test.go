package signalattribution

import (
	"math"
	"testing"
	"time"

	"go-stock/backend/tradingcalendar"
)

func weekendCal() tradingcalendar.Calendar {
	return tradingcalendar.Calendar{}
}

func approx(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func horizon(row HitRow, h int) HorizonCell {
	for _, c := range row.Horizons {
		if c.Horizon == h {
			return c
		}
	}
	return HorizonCell{}
}

func tenDayBars() []DayBar {
	bars := []DayBar{{Date: "2026-07-17", Close: 100}}
	days := []string{
		"2026-07-20", "2026-07-21", "2026-07-22", "2026-07-23", "2026-07-24",
		"2026-07-27", "2026-07-28", "2026-07-29", "2026-07-30", "2026-07-31",
	}
	for i, d := range days {
		bars = append(bars, DayBar{Date: d, Close: 100 * (1 + 0.01*float64(i+1))})
	}
	return bars
}

func TestObserve_TenTradingDays(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17", StrategyID: "s1", StrategyName: "策略甲"}
	row := observeRow(meta, HitInput{Code: "600000.SH", Name: "浦发银行", SnapshotPrice: 1, BarKey: "600000.SH"}, tenDayBars(), weekendCal())
	if row.PriceBasis != BasisLocalClose || row.CloseText != "100.00" {
		t.Fatalf("entry %+v", row)
	}
	if row.Close == nil || *row.Close != 100 {
		t.Fatal("local close should win over snapshot price")
	}
	h1 := horizon(row, 1)
	if h1.Status != StatusOK || h1.Text != "+1.00%" || h1.FutureDate != "2026-07-20" || h1.ReturnRate == nil {
		t.Fatalf("+1 %+v", h1)
	}
	approx(t, *h1.ReturnRate, 0.01)
	h3 := horizon(row, 3)
	if h3.Status != StatusOK || h3.FutureDate != "2026-07-22" || h3.Text != "+3.00%" {
		t.Fatalf("+3 %+v", h3)
	}
	h10 := horizon(row, 10)
	if h10.Status != StatusOK || h10.FutureDate != "2026-07-31" || h10.Text != "+10.00%" {
		t.Fatalf("+10 %+v", h10)
	}
}

func TestObserve_MissingHorizonFailsClosed(t *testing.T) {
	bars := tenDayBars()[:4] // as-of + 3 sessions
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000"}, bars, weekendCal())
	if horizon(row, 1).Status != StatusOK || horizon(row, 3).Status != StatusOK {
		t.Fatalf("early horizons should exist: %+v", row.Horizons)
	}
	h10 := horizon(row, 10)
	if h10.Status != StatusInsufficient || h10.Text != TextInsufficient || h10.ReturnRate != nil || h10.Reason != ReasonNoFuture {
		t.Fatalf("+10 should be insufficient: %+v", h10)
	}
}

func TestObserve_WeekdayHoleDoesNotSkip(t *testing.T) {
	bars := []DayBar{
		{Date: "2026-07-13", Close: 100},
		{Date: "2026-07-14", Close: 110},
		{Date: "2026-07-16", Close: 120}, // Wednesday 15 missing
	}
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-13"}, HitInput{Code: "000001.SZ"}, bars, weekendCal())
	h1 := horizon(row, 1)
	if h1.Status != StatusOK || h1.FutureDate != "2026-07-14" {
		t.Fatalf("+1 %+v", h1)
	}
	approx(t, *h1.ReturnRate, 0.10)
	h3 := horizon(row, 3)
	if h3.Status != StatusInsufficient || h3.ReturnRate != nil || h3.Reason != ReasonCalendarGap || h3.Text != TextInsufficient {
		t.Fatalf("hole must not become +3: %+v", h3)
	}
	if horizon(row, 10).ReturnRate != nil {
		t.Fatal("must not invent +10")
	}
}

func TestObserve_HolidayGapFailsWithoutCalendar(t *testing.T) {
	bars := []DayBar{
		{Date: "2026-09-30", Close: 100},
		{Date: "2026-10-05", Close: 110},
	}
	row := observeRow(SnapshotMeta{TradeDate: "2026-09-30"}, HitInput{Code: "600000"}, bars, weekendCal())
	h1 := horizon(row, 1)
	if h1.Status != StatusInsufficient || h1.ReturnRate != nil || h1.Reason != ReasonCalendarGap {
		t.Fatalf("weekend-only calendar must not treat post-holiday bar as +1: %+v", h1)
	}

	cal := tradingcalendar.Calendar{Holidays: tradingcalendar.StaticHolidays{NonTrading: map[string]bool{
		"2026-10-01": true,
		"2026-10-02": true,
	}}}
	row = observeRow(SnapshotMeta{TradeDate: "2026-09-30"}, HitInput{Code: "600000"}, bars, cal)
	h1 = horizon(row, 1)
	if h1.Status != StatusOK || h1.FutureDate != "2026-10-05" || h1.Text != "+10.00%" {
		t.Fatalf("with holiday table +1 should be Oct 5: %+v", h1)
	}
}

func TestObserve_SnapshotPriceOnlyWhenCloseMissing(t *testing.T) {
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000", Name: "", SnapshotPrice: 12.5}, nil, weekendCal())
	if row.PriceBasis != BasisSnapshot || row.CloseText != "12.50" || row.PriceBasisLabel != BasisLabelSnapshot {
		t.Fatalf("basis %+v", row)
	}
	h1 := horizon(row, 1)
	if h1.Status != StatusInsufficient || h1.Reason != ReasonNoFuture || h1.ReturnRate != nil {
		t.Fatalf("no future bar: %+v", h1)
	}
}

func TestObserve_NoPrice(t *testing.T) {
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000", SnapshotPrice: 0}, nil, weekendCal())
	if row.Close != nil || row.CloseText != TextInsufficient || row.PriceBasis != "" {
		t.Fatalf("missing price %+v", row)
	}
	if horizon(row, 1).Reason != ReasonNoEntry {
		t.Fatalf("reason %+v", horizon(row, 1))
	}
}

func TestObserve_AsOfBarIsNotPlusOne(t *testing.T) {
	bars := []DayBar{{Date: "2026-07-17", Close: 10}}
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000"}, bars, weekendCal())
	if row.CloseText != "10.00" {
		t.Fatal(row.CloseText)
	}
	if horizon(row, 1).Status != StatusInsufficient || horizon(row, 1).ReturnRate != nil {
		t.Fatalf("as-of bar is not +1: %+v", horizon(row, 1))
	}
}

func TestObserve_WeekendGapTrusted(t *testing.T) {
	bars := []DayBar{
		{Date: "2026-07-17", Close: 10},
		{Date: "2026-07-20", Close: 9},
	}
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000"}, bars, weekendCal())
	h1 := horizon(row, 1)
	if h1.Status != StatusOK || h1.Text != "-10.00%" || h1.FutureDate != "2026-07-20" {
		t.Fatalf("%+v", h1)
	}
}

func TestAssemble_SummaryMedianMeanAndPage(t *testing.T) {
	meta := SnapshotMeta{ID: 7, TradeDate: "2026-07-17", Session: "close", StrategyID: "s1", StrategyName: "策略甲"}
	bars := map[string][]DayBar{
		"A": tenDayBars(),
		"B": {
			{Date: "2026-07-17", Close: 100},
			{Date: "2026-07-20", Close: 130},
			{Date: "2026-07-21", Close: 100},
			{Date: "2026-07-22", Close: 110},
		},
		"C": nil,
	}
	// A +1 = +1%, B +1 = +30%, C insufficient
	hits := []HitInput{
		{Code: "A", BarKey: "A", SnapshotPrice: 50},
		{Code: "B", BarKey: "B"},
		{Code: "C", BarKey: "C"},
	}
	view := Assemble(meta, hits, bars, weekendCal(), 2, 1)
	if !view.OK || view.Total != 3 || view.Page != 2 || len(view.Rows) != 1 || view.Rows[0].Code != "B" {
		t.Fatalf("page %+v", view)
	}
	if view.Summary.HitCount != 3 || view.Summary.Label != ResearchStatLabel {
		t.Fatalf("summary %+v", view.Summary)
	}
	if view.Summary.CompleteAll != 1 {
		t.Fatalf("completeAll %d", view.Summary.CompleteAll)
	}
	var h1 HorizonStat
	for _, s := range view.Summary.Horizons {
		if s.Horizon == 1 {
			h1 = s
		}
	}
	if h1.Complete != 2 || h1.Mean == nil || h1.Median == nil {
		t.Fatalf("stat %+v", h1)
	}
	approx(t, *h1.Mean, 0.155) // (0.01+0.30)/2
	approx(t, *h1.Median, 0.155)
	if h1.MeanText == TextInsufficient || h1.MeanText == "" {
		t.Fatal(h1.MeanText)
	}
	if view.Disclaimer != Disclaimer {
		t.Fatal(view.Disclaimer)
	}
}

func TestAssemble_EmptyHits(t *testing.T) {
	view := Assemble(SnapshotMeta{TradeDate: "2026-07-17"}, nil, nil, weekendCal(), 1, 50)
	if !view.OK || view.Message == "" || view.Summary.HitCount != 0 || len(view.Rows) != 0 {
		t.Fatalf("%+v", view)
	}
}

func TestGapTrusted_RejectsWeekendBar(t *testing.T) {
	if gapTrusted("2026-07-17", "2026-07-18", weekendCal()) {
		t.Fatal("Saturday bar is not a trading day")
	}
	if !gapTrusted("2026-07-17", "2026-07-20", weekendCal()) {
		t.Fatal("Friday to Monday should be trusted")
	}
}

func TestNormalizeDay(t *testing.T) {
	if NormalizeDay("20260717") != "2026-07-17" {
		t.Fatal(NormalizeDay("20260717"))
	}
	if NormalizeDay("2026-07-17 15:00:00") != "2026-07-17" {
		t.Fatal("datetime")
	}
	if NormalizeDay("not-a-day") != "" {
		t.Fatal("junk")
	}
	_ = time.Local
}
