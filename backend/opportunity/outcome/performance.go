package outcome

import (
	"strings"
	"time"
)

func computePerformance(entryPrice, exitPrice float64, entryFee, exitFee float64, qty int64, entryDate, exitDate string, asOf time.Time, closed bool) PerformanceBlock {
	if qty <= 0 {
		return PerformanceBlock{}
	}
	out := PerformanceBlock{
		EntryPrice: entryPrice,
		Quantity:   qty,
	}
	if closed && exitPrice > 0 {
		out.ExitPrice = &exitPrice
		cost := entryPrice*float64(qty) + entryFee
		proceeds := exitPrice*float64(qty) - exitFee
		if cost > 0 {
			pct := (proceeds - cost) / cost * 100
			out.RealizedReturnPct = &pct
		}
		endDate := exitDate
		if endDate == "" {
			endDate = entryDate
		}
		out.HoldingDays = calendarHoldingDays(entryDate, endDate)
		return out
	}
	out.HoldingDays = calendarHoldingDays(entryDate, formatAsOfDate(asOf))
	return out
}

func formatAsOfDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}

// calendarHoldingDays returns natural-day span (endDate − startDate).
func calendarHoldingDays(startDate, endDate string) int {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	if startDate == "" || endDate == "" {
		return 0
	}
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return 0
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return 0
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}
