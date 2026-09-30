package signalattribution

import (
	"fmt"
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

func tradingDays(start string, n int) []string {
	t, err := time.Parse("2006-01-02", start)
	if err != nil {
		panic(err)
	}
	out := make([]string, 0, n)
	for len(out) < n {
		if t.Weekday() != time.Saturday && t.Weekday() != time.Sunday {
			out = append(out, t.Format("2006-01-02"))
		}
		t = t.AddDate(0, 0, 1)
	}
	return out
}

func TestObserve_ToDateUsesLatestContiguousBar(t *testing.T) {
	bars := tenDayBars()
	bars = append(bars, DayBar{Date: "2026-08-03", Close: 150})
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-17"}, HitInput{Code: "600000"}, bars, weekendCal())
	h10 := horizon(row, 10)
	if h10.Status != StatusOK || h10.FutureDate != "2026-07-31" {
		t.Fatalf("+10 must stay on the 10th session: %+v", h10)
	}
	if row.ToDate.Status != StatusOK || row.ToDate.FutureDate != "2026-08-03" || row.ToDate.Text != "+50.00%" || row.ToDate.ReturnRate == nil {
		t.Fatalf("to-date %+v", row.ToDate)
	}
}

func TestObserve_ToDateFailsClosedOnGap(t *testing.T) {
	bars := []DayBar{
		{Date: "2026-07-13", Close: 100},
		{Date: "2026-07-14", Close: 110},
		{Date: "2026-07-16", Close: 120},
	}
	row := observeRow(SnapshotMeta{TradeDate: "2026-07-13"}, HitInput{Code: "000001"}, bars, weekendCal())
	if row.ToDate.Status != StatusInsufficient || row.ToDate.ReturnRate != nil || row.ToDate.Reason != ReasonCalendarGap {
		t.Fatalf("gap must not become 迄今: %+v", row.ToDate)
	}
}

func TestObserve_Prior20IgnoresFutureBars(t *testing.T) {
	days := tradingDays("2026-06-01", 22)
	bars := make([]DayBar, 0, len(days)+1)
	for _, d := range days {
		bars = append(bars, DayBar{Date: d, Close: 100})
	}
	asOf := days[len(days)-1]
	next := tradingDays(asOf, 2)[1]
	bars = append(bars, DayBar{Date: next, Close: 200})
	row := observeRow(SnapshotMeta{TradeDate: asOf}, HitInput{
		Code: "600000", Industry: "电子", Tag: "强", HasVolumeRatio: true, VolumeRatio: 1.8,
	}, bars, weekendCal())
	if row.Features.Prior20Return == nil || math.Abs(*row.Features.Prior20Return) > 1e-9 {
		t.Fatalf("future bar must not enter prior 20d: %+v", row.Features.Prior20Return)
	}
	if row.Features.DistMA20 == nil || math.Abs(*row.Features.DistMA20) > 1e-9 {
		t.Fatalf("future bar must not enter MA20: %+v", row.Features.DistMA20)
	}
	h1 := horizon(row, 1)
	if h1.Status != StatusOK || h1.Text != "+100.00%" {
		t.Fatalf("+1 still uses the next bar: %+v", h1)
	}
	if row.Features.Industry != "电子" || row.Features.VolumeRatio == nil || *row.Features.VolumeRatio != 1.8 {
		t.Fatalf("snapshot features %+v", row.Features)
	}
}

func TestObserve_Prior20GapFailsClosed(t *testing.T) {
	days := tradingDays("2026-06-01", 25)
	bars := make([]DayBar, 0, len(days))
	asOf := days[len(days)-1]
	for _, d := range days {
		if d == days[len(days)-3] {
			continue
		}
		bars = append(bars, DayBar{Date: d, Close: 100})
	}
	row := observeRow(SnapshotMeta{TradeDate: asOf}, HitInput{Code: "600000"}, bars, weekendCal())
	if row.Features.Prior20Return != nil || row.Features.DistMA20 != nil {
		t.Fatalf("lookback hole must not invent MA/prior: %+v", row.Features)
	}
}

func TestAssemble_SortMissingLast(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17"}
	bars := map[string][]DayBar{
		"A": {{Date: "2026-07-17", Close: 100}, {Date: "2026-07-20", Close: 101}},
		"B": {{Date: "2026-07-17", Close: 100}, {Date: "2026-07-20", Close: 80}},
		"C": nil,
	}
	hits := []HitInput{{Code: "C", BarKey: "C"}, {Code: "A", BarKey: "A"}, {Code: "B", BarKey: "B"}}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 10, AssembleOptions{SortKey: "1", SortDesc: true})
	if len(view.Rows) != 3 || view.Rows[0].Code != "A" || view.Rows[1].Code != "B" || view.Rows[2].Code != "C" {
		t.Fatalf("desc %+v", codes(view.Rows))
	}
	view = Assemble(meta, hits, bars, weekendCal(), 1, 10, AssembleOptions{SortKey: "1", SortDesc: false})
	if view.Rows[0].Code != "B" || view.Rows[1].Code != "A" || view.Rows[2].Code != "C" {
		t.Fatalf("asc %+v", codes(view.Rows))
	}
	view = Assemble(meta, hits, bars, weekendCal(), 1, 10, AssembleOptions{SortKey: "toDate", SortDesc: true})
	if view.Rows[0].Code != "A" || view.Rows[2].Code != "C" || view.Rows[2].ToDate.ReturnRate != nil {
		t.Fatalf("toDate sort %+v", codes(view.Rows))
	}
}

func codes(rows []HitRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Code
	}
	return out
}

func TestAssemble_LargeSampleDoesNotHighlight(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17", StrategyID: "default"}
	hits := make([]HitInput, 0, LargeSampleLimit+1)
	bars := map[string][]DayBar{}
	for i := 0; i < LargeSampleLimit+1; i++ {
		key := fmt.Sprintf("K%03d", i)
		industry := "电子"
		vol := 2.0
		next := 110.0
		if i%2 == 0 {
			industry = "银行"
			vol = 0.5
			next = 90
		}
		hits = append(hits, HitInput{
			Code: key, BarKey: key, Tag: "强", Industry: industry,
			HasVolumeRatio: true, VolumeRatio: vol,
		})
		bars[key] = []DayBar{
			{Date: "2026-07-17", Close: 100},
			{Date: "2026-07-20", Close: next},
		}
	}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 50)
	if !view.LargeSample || view.LargeSampleWarning != LargeSampleWarning {
		t.Fatalf("large %+v %q", view.LargeSample, view.LargeSampleWarning)
	}
	if !view.Cohort.LargeSample || view.Cohort.BrowseNote == "" {
		t.Fatalf("cohort large %+v", view.Cohort.BrowseNote)
	}
	for _, c := range view.Cohort.Contrasts {
		if c.Highlight {
			t.Fatalf("must not claim commonality: %+v", c)
		}
	}
	if view.Summary.HitCount != LargeSampleLimit+1 {
		t.Fatalf("summary %d", view.Summary.HitCount)
	}
}

func TestCohort_SmokeUpDownContrast(t *testing.T) {
	meta := SnapshotMeta{TradeDate: "2026-07-17", StrategyID: "s1", StrategyName: "策略甲"}
	hits := []HitInput{
		{Code: "U1", BarKey: "U1", Tag: "强", Industry: "电子", HasVolumeRatio: true, VolumeRatio: 2.4, HasRSI: true, RSI: 62},
		{Code: "U2", BarKey: "U2", Tag: "强", Industry: "电子", HasVolumeRatio: true, VolumeRatio: 2.0, HasRSI: true, RSI: 58},
		{Code: "D1", BarKey: "D1", Tag: "强", Industry: "银行", HasVolumeRatio: true, VolumeRatio: 0.8, HasRSI: true, RSI: 40},
		{Code: "D2", BarKey: "D2", Tag: "强", Industry: "银行", HasVolumeRatio: true, VolumeRatio: 0.6, HasRSI: true, RSI: 42},
		{Code: "X", BarKey: "X", Tag: "强", Industry: "电子"},
	}
	bars := map[string][]DayBar{
		"U1": {{Date: "2026-07-17", Close: 10}, {Date: "2026-07-20", Close: 11}},
		"U2": {{Date: "2026-07-17", Close: 10}, {Date: "2026-07-20", Close: 12}},
		"D1": {{Date: "2026-07-17", Close: 10}, {Date: "2026-07-20", Close: 9}},
		"D2": {{Date: "2026-07-17", Close: 10}, {Date: "2026-07-20", Close: 8}},
	}
	view := Assemble(meta, hits, bars, weekendCal(), 1, 50)
	panel := view.Cohort
	if !panel.OK || panel.Up.Count != 2 || panel.Down.Count != 2 || panel.Excluded != 1 {
		t.Fatalf("groups %+v excluded %d msg %s", panel, panel.Excluded, panel.Message)
	}
	if panel.Note == "" || panel.Warning == "" || panel.SizeNote == "" {
		t.Fatalf("copy note=%q warn=%q size=%q", panel.Note, panel.Warning, panel.SizeNote)
	}
	highlighted := 0
	for _, c := range panel.Contrasts {
		t.Logf("smoke contrast %s up=%s down=%s %s highlight=%v", c.Label, c.UpText, c.DownText, c.DiffText, c.Highlight)
		if c.Highlight {
			highlighted++
		}
	}
	t.Logf("smoke note: %s", panel.Note)
	t.Logf("smoke warning: %s", panel.Warning)
	if highlighted == 0 {
		t.Fatal("small sample should still mark the largest gaps")
	}
	if panel.Up.TopIndustry != "电子" || panel.Down.TopIndustry != "银行" {
		t.Fatalf("industry up=%s down=%s", panel.Up.TopIndustry, panel.Down.TopIndustry)
	}
}

func TestFilterHits_TagReboundAndEmpty(t *testing.T) {
	hits := []HitInput{
		{Code: "a", Tag: "强"},
		{Code: "b", Tag: "超"},
		{Code: "c", Tag: "弹", Name: "ST测试", HasRSI: true, RSI: 40},
		{Code: "d", Tag: "弹", Name: "正常", HasRSI: true, RSI: 80},
		{Code: "e", Tag: "减"},
	}
	if got := FilterHits(hits, nil, nil); len(got) != len(hits) {
		t.Fatalf("empty filter keeps all: %d", len(got))
	}
	strong := FilterHits(hits, []string{"趋"}, nil)
	if len(strong) != 1 || strong[0].Code != "b" {
		t.Fatalf("超 should match 趋: %+v", strong)
	}
	max := 60.0
	rebound := FilterHits(hits, []string{"弹"}, &max)
	if len(rebound) != 0 {
		t.Fatalf("ST and high RSI 弹 dropped: %+v", rebound)
	}
	sell := FilterHits(hits, []string{"卖"}, nil)
	if len(sell) != 1 || sell[0].Code != "e" {
		t.Fatalf("卖 matches 减: %+v", sell)
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
