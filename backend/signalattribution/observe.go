package signalattribution

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"go-stock/backend/tradingcalendar"
)

func floatPtr(v float64) *float64 {
	x := v
	return &x
}

func formatReturnPct(rate float64) string {
	pct := rate * 100
	if pct < 0 {
		return fmt.Sprintf("%.2f%%", pct)
	}
	return fmt.Sprintf("+%.2f%%", pct)
}

func formatClose(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func insufficientCell(horizon int, reason string) HorizonCell {
	return HorizonCell{
		Horizon: horizon,
		Status:  StatusInsufficient,
		Text:    TextInsufficient,
		Reason:  reason,
	}
}

// gapTrusted reports whether nextDay can be the next trading session after prevDay.
// Every calendar day strictly between them must be a non-trading day, and nextDay
// itself must be a trading day. A weekday sitting in the gap (holiday table missing,
// or a missing bar) fails closed.
func gapTrusted(prevDay, nextDay string, cal tradingcalendar.Calendar) bool {
	prev, err1 := tradingcalendar.ParseDate(prevDay)
	next, err2 := tradingcalendar.ParseDate(nextDay)
	if err1 != nil || err2 != nil {
		return false
	}
	prev = cal.TruncateDay(prev)
	next = cal.TruncateDay(next)
	if !next.After(prev) {
		return false
	}
	if !cal.IsTradingDay(next) {
		return false
	}
	for cur := prev.AddDate(0, 0, 1); cur.Before(next); cur = cur.AddDate(0, 0, 1) {
		if cal.IsTradingDay(cur) {
			return false
		}
	}
	return true
}

func indexBars(bars []DayBar) map[string]float64 {
	out := make(map[string]float64, len(bars))
	for _, b := range bars {
		d := NormalizeDay(b.Date)
		if d == "" || b.Close <= 0 || math.IsNaN(b.Close) || math.IsInf(b.Close, 0) {
			continue
		}
		out[d] = b.Close
	}
	return out
}

type sessionClose struct {
	date  string
	close float64
}

type forwardStop struct {
	closes map[int]sessionClose
	// gap is set when the next bar is not a trusted trading session.
	// Bars after the gap are ignored. Horizons already reached stay usable;
	// 迄今 fails closed because the latest bar is no longer known.
	gap bool
}

func walkForward(asOf string, byDate map[string]float64, cal tradingcalendar.Calendar) forwardStop {
	dates := make([]string, 0, len(byDate))
	for d := range byDate {
		if d > asOf {
			dates = append(dates, d)
		}
	}
	sort.Strings(dates)
	out := forwardStop{closes: map[int]sessionClose{}}
	cursor := asOf
	offset := 0
	for _, d := range dates {
		if !gapTrusted(cursor, d, cal) {
			out.gap = true
			return out
		}
		offset++
		out.closes[offset] = sessionClose{date: d, close: byDate[d]}
		cursor = d
	}
	return out
}

func horizonsFromWalk(asOf string, entry float64, hasEntry bool, walk forwardStop) []HorizonCell {
	cells := make([]HorizonCell, 0, len(Horizons))
	if asOf == "" {
		for _, h := range Horizons {
			cells = append(cells, insufficientCell(h, ReasonBadAsOf))
		}
		return cells
	}
	if !hasEntry || entry <= 0 || math.IsNaN(entry) || math.IsInf(entry, 0) {
		for _, h := range Horizons {
			cells = append(cells, insufficientCell(h, ReasonNoEntry))
		}
		return cells
	}
	for _, h := range Horizons {
		hit, ok := walk.closes[h]
		if !ok || hit.close <= 0 {
			reason := ReasonNoFuture
			if walk.gap {
				reason = ReasonCalendarGap
			}
			cells = append(cells, insufficientCell(h, reason))
			continue
		}
		rate := (hit.close - entry) / entry
		cells = append(cells, HorizonCell{
			Horizon:     h,
			Status:      StatusOK,
			Text:        formatReturnPct(rate),
			ReturnRate:  floatPtr(rate),
			FutureDate:  hit.date,
			FutureClose: floatPtr(hit.close),
		})
	}
	return cells
}

func toDateFromWalk(asOf string, entry float64, hasEntry bool, walk forwardStop) HorizonCell {
	if asOf == "" {
		return insufficientCell(0, ReasonBadAsOf)
	}
	if !hasEntry || entry <= 0 || math.IsNaN(entry) || math.IsInf(entry, 0) {
		return insufficientCell(0, ReasonNoEntry)
	}
	if walk.gap || len(walk.closes) == 0 {
		reason := ReasonNoFuture
		if walk.gap {
			reason = ReasonCalendarGap
		}
		return insufficientCell(0, reason)
	}
	lastN := 0
	var last sessionClose
	for n, c := range walk.closes {
		if n >= lastN && c.close > 0 {
			lastN = n
			last = c
		}
	}
	if lastN == 0 || last.close <= 0 {
		return insufficientCell(0, ReasonNoFuture)
	}
	rate := (last.close - entry) / entry
	return HorizonCell{
		Horizon:     0,
		Status:      StatusOK,
		Text:        formatReturnPct(rate),
		ReturnRate:  floatPtr(rate),
		FutureDate:  last.date,
		FutureClose: floatPtr(last.close),
	}
}

func observeRow(meta SnapshotMeta, hit HitInput, bars []DayBar, cal tradingcalendar.Calendar) HitRow {
	asOf := NormalizeDay(meta.TradeDate)
	row := HitRow{
		Code:         hit.Code,
		Name:         hit.Name,
		KlineCode:    strings.TrimSpace(hit.KlineCode),
		Tag:          strings.TrimSpace(hit.Tag),
		StrategyID:   meta.StrategyID,
		StrategyName: meta.StrategyName,
		AsOfDate:     asOf,
		CloseText:    TextInsufficient,
	}
	if hit.Code == "" {
		row.Code = "—"
	}
	byDate := indexBars(bars)
	var entry float64
	hasEntry := false
	if asOf != "" {
		if c, ok := byDate[asOf]; ok && c > 0 {
			entry = c
			hasEntry = true
			row.PriceBasis = BasisLocalClose
			row.PriceBasisLabel = BasisLabelLocal
		} else if hit.SnapshotPrice > 0 && !math.IsNaN(hit.SnapshotPrice) && !math.IsInf(hit.SnapshotPrice, 0) {
			entry = hit.SnapshotPrice
			hasEntry = true
			row.PriceBasis = BasisSnapshot
			row.PriceBasisLabel = BasisLabelSnapshot
		}
	}
	if hasEntry {
		row.Close = floatPtr(entry)
		row.CloseText = formatClose(entry)
	}
	walk := walkForward(asOf, byDate, cal)
	row.Horizons = horizonsFromWalk(asOf, entry, hasEntry, walk)
	row.ToDate = toDateFromWalk(asOf, entry, hasEntry, walk)
	row.Features = asOfFeatures(asOf, hit, byDate, cal)
	return row
}

func median(values []float64) float64 {
	s := append([]float64(nil), values...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func cellByHorizon(row HitRow, horizon int) (HorizonCell, bool) {
	for _, c := range row.Horizons {
		if c.Horizon == horizon {
			return c, true
		}
	}
	return HorizonCell{}, false
}

func collectRates(rows []HitRow, pick func(HitRow) (HorizonCell, bool)) []float64 {
	vals := make([]float64, 0, len(rows))
	for _, row := range rows {
		c, ok := pick(row)
		if !ok || c.Status != StatusOK || c.ReturnRate == nil {
			continue
		}
		vals = append(vals, *c.ReturnRate)
	}
	return vals
}

func horizonStat(horizon int, rows []HitRow, pick func(HitRow) (HorizonCell, bool)) HorizonStat {
	stat := HorizonStat{
		Horizon:    horizon,
		MeanText:   EmptyStatText,
		MedianText: EmptyStatText,
	}
	vals := collectRates(rows, pick)
	stat.Complete = len(vals)
	if len(vals) == 0 {
		return stat
	}
	var total float64
	for _, v := range vals {
		total += v
	}
	mean := total / float64(len(vals))
	med := median(vals)
	stat.Mean = floatPtr(mean)
	stat.Median = floatPtr(med)
	stat.MeanText = formatReturnPct(mean)
	stat.MedianText = formatReturnPct(med)
	return stat
}

func summarize(rows []HitRow) Summary {
	sum := Summary{
		HitCount:         len(rows),
		SnapshotHitCount: len(rows),
		Label:            ResearchStatLabel,
		Horizons:         make([]HorizonStat, 0, len(Horizons)),
	}
	for _, row := range rows {
		complete := true
		for _, h := range Horizons {
			c, ok := cellByHorizon(row, h)
			if !ok || c.Status != StatusOK {
				complete = false
				break
			}
		}
		if complete {
			sum.CompleteAll++
		}
	}
	for _, h := range Horizons {
		h := h
		sum.Horizons = append(sum.Horizons, horizonStat(h, rows, func(row HitRow) (HorizonCell, bool) {
			return cellByHorizon(row, h)
		}))
	}
	sum.ToDate = horizonStat(0, rows, func(row HitRow) (HorizonCell, bool) {
		if row.ToDate.Status == "" {
			return HorizonCell{}, false
		}
		return row.ToDate, true
	})
	return sum
}

func sortValue(row HitRow, key string) (float64, bool) {
	switch key {
	case "1", "3", "10":
		h, err := strconv.Atoi(key)
		if err != nil {
			return 0, false
		}
		c, ok := cellByHorizon(row, h)
		if !ok || c.Status != StatusOK || c.ReturnRate == nil {
			return 0, false
		}
		return *c.ReturnRate, true
	case "toDate":
		if row.ToDate.Status != StatusOK || row.ToDate.ReturnRate == nil {
			return 0, false
		}
		return *row.ToDate.ReturnRate, true
	default:
		return 0, false
	}
}

func sortRows(rows []HitRow, key string, desc bool) {
	if strings.TrimSpace(key) == "" {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ai, aok := sortValue(rows[i], key)
		bi, bok := sortValue(rows[j], key)
		if aok != bok {
			return aok
		}
		if !aok || ai == bi {
			return false
		}
		if desc {
			return ai > bi
		}
		return ai < bi
	})
}

func normalizePage(page, pageSize, total int) (int, int) {
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if page < 1 {
		page = 1
	}
	if total <= 0 {
		return page, pageSize
	}
	maxPage := (total + pageSize - 1) / pageSize
	if page > maxPage {
		page = maxPage
	}
	return page, pageSize
}

func baseView(meta SnapshotMeta) *View {
	return &View{
		OK:                true,
		Disclaimer:        Disclaimer,
		ScopeNote:         ScopeNote,
		CalendarNote:      CalendarNote,
		BarNote:           BarNote,
		ToDateNote:        ToDateNote,
		ResearchStatLabel: ResearchStatLabel,
		SnapshotID:        meta.ID,
		TradeDate:         NormalizeDay(meta.TradeDate),
		Session:           meta.Session,
		StrategyID:        meta.StrategyID,
		StrategyName:      meta.StrategyName,
		Rows:              []HitRow{},
	}
}

// Assemble builds the observation table from already-loaded local bars.
// It does not read the network or any trading-plan store.
// hits are already the filtered subset. Optional AssembleOptions carry sort and the raw snapshot count.
func Assemble(meta SnapshotMeta, hits []HitInput, bars map[string][]DayBar, cal tradingcalendar.Calendar, page, pageSize int, opts ...AssembleOptions) *View {
	var opt AssembleOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	view := baseView(meta)
	if bars == nil {
		bars = map[string][]DayBar{}
	}
	all := make([]HitRow, 0, len(hits))
	for _, hit := range hits {
		all = append(all, observeRow(meta, hit, bars[hit.BarKey], cal))
	}
	sortRows(all, opt.SortKey, opt.SortDesc)
	view.Summary = summarize(all)
	if opt.SnapshotHitCount > 0 {
		view.Summary.SnapshotHitCount = opt.SnapshotHitCount
	}
	large := len(all) > LargeSampleLimit
	view.LargeSample = large
	if large {
		view.LargeSampleWarning = LargeSampleWarning
	}
	view.Cohort = BuildCohort(all, large)
	view.WhatIf = BuildWhatIf(all, view.Cohort, WhatIfInput{
		Explicit: opt.WhatIfSet,
		Keys:     opt.WhatIfKeys,
	})
	view.Total = len(all)
	page, pageSize = normalizePage(page, pageSize, len(all))
	view.Page = page
	view.PageSize = pageSize
	if len(all) == 0 {
		if view.Summary.SnapshotHitCount > 0 {
			view.Message = "当前信号筛选下没有命中"
		} else {
			view.Message = "该快照没有命中股票"
		}
		return view
	}
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > len(all) {
		start = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	view.Rows = all[start:end]
	return view
}

// EmptyView is the fail-closed shell when a snapshot cannot be read.
func EmptyView(message string) *View {
	view := baseView(SnapshotMeta{})
	view.OK = false
	view.Message = message
	view.Summary = summarize(nil)
	view.Cohort = BuildCohort(nil, false)
	view.WhatIf = BuildWhatIf(nil, view.Cohort, WhatIfInput{})
	view.Page = 1
	view.PageSize = 50
	return view
}
