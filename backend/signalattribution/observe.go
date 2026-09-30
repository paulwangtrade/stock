package signalattribution

import (
	"fmt"
	"math"
	"sort"

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

type forwardStop struct {
	closes map[int]struct {
		date  string
		close float64
	}
	// reason applies to horizons beyond len(closes). Empty when the walk
	// simply ran out of bars.
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
	out := forwardStop{closes: map[int]struct {
		date  string
		close float64
	}{}}
	cursor := asOf
	offset := 0
	for _, d := range dates {
		if !gapTrusted(cursor, d, cal) {
			out.gap = true
			return out
		}
		offset++
		out.closes[offset] = struct {
			date  string
			close float64
		}{date: d, close: byDate[d]}
		cursor = d
		if offset >= 10 {
			return out
		}
	}
	return out
}

func observeHorizons(asOf string, entry float64, hasEntry bool, bars []DayBar, cal tradingcalendar.Calendar) []HorizonCell {
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
	walk := walkForward(asOf, indexBars(bars), cal)
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

func observeRow(meta SnapshotMeta, hit HitInput, bars []DayBar, cal tradingcalendar.Calendar) HitRow {
	asOf := NormalizeDay(meta.TradeDate)
	row := HitRow{
		Code:         hit.Code,
		Name:         hit.Name,
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
	row.Horizons = observeHorizons(asOf, entry, hasEntry, bars, cal)
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

func summarize(rows []HitRow) Summary {
	sum := Summary{
		HitCount: len(rows),
		Label:    ResearchStatLabel,
		Horizons: make([]HorizonStat, 0, len(Horizons)),
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
		stat := HorizonStat{
			Horizon:    h,
			MeanText:   EmptyStatText,
			MedianText: EmptyStatText,
		}
		vals := make([]float64, 0, len(rows))
		for _, row := range rows {
			c, ok := cellByHorizon(row, h)
			if !ok || c.Status != StatusOK || c.ReturnRate == nil {
				continue
			}
			vals = append(vals, *c.ReturnRate)
		}
		stat.Complete = len(vals)
		if len(vals) > 0 {
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
		}
		sum.Horizons = append(sum.Horizons, stat)
	}
	return sum
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
func Assemble(meta SnapshotMeta, hits []HitInput, bars map[string][]DayBar, cal tradingcalendar.Calendar, page, pageSize int) *View {
	view := baseView(meta)
	if bars == nil {
		bars = map[string][]DayBar{}
	}
	all := make([]HitRow, 0, len(hits))
	for _, hit := range hits {
		all = append(all, observeRow(meta, hit, bars[hit.BarKey], cal))
	}
	view.Summary = summarize(all)
	view.Total = len(all)
	page, pageSize = normalizePage(page, pageSize, len(all))
	view.Page = page
	view.PageSize = pageSize
	if len(all) == 0 {
		view.Message = "该快照没有命中股票"
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
	view.Page = 1
	view.PageSize = 50
	return view
}
