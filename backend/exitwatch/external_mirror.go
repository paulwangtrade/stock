package exitwatch

import (
	"math"
	"strconv"
	"strings"
	"time"

	"go-stock/backend/externalmirror"
)

// EvaluateMirror projects mirror rows and quote snapshots only.
// A foreign source is fail-closed and is not relabeled as paper_sim.
// Sell intent is never allowed. Plan and signal reasons are not invented.
func EvaluateMirror(rows []externalmirror.View, quotes []QuoteSnap, opt Options) []Item {
	opt = opt.normalized()
	pol := opt.Policy.normalized()
	asOf := opt.AsOf
	bar := opt.Bar
	byCode := map[string]QuoteSnap{}
	for _, q := range quotes {
		key := normCode(q.Code)
		if key == "" {
			continue
		}
		byCode[key] = q
	}
	out := make([]Item, 0, len(rows))
	for _, row := range rows {
		src := ""
		if strings.TrimSpace(row.Source) == externalmirror.Source {
			src = SourceExternalMirror
		}
		posID := ""
		if row.ID > 0 {
			posID = strconv.FormatUint(uint64(row.ID), 10)
		}
		q, hasQuote := byCode[normCode(row.StockCode)]
		priceKnown := false
		stale := false
		if hasQuote {
			priceKnown = finitePrice(q.Price)
			stale = quoteStale(q.FetchedAt, asOf)
		}
		costKnown := finitePrice(row.CostPrice)
		var ret *float64
		if priceKnown && costKnown && !stale {
			r := (q.Price - row.CostPrice) / row.CostPrice
			ret = &r
		}
		days, daysOK := holdingDaysBetween(row.EntryDate, bar)
		out = append(out, Project(Facts{
			Source:             src,
			PositionID:         posID,
			StockCode:          strings.TrimSpace(row.StockCode),
			StockName:          row.StockName,
			Quantity:           row.Quantity,
			QuantityKnown:      row.Quantity > 0,
			RequireQuantity:    true,
			CostKnown:          costKnown,
			PriceKnown:         priceKnown,
			PriceStale:         stale,
			UnrealizedReturn:   ret,
			HoldingDays:        days,
			HoldingDaysKnown:   daysOK,
			RequireHoldingDays: true,
			T1Locked:           daysOK && strings.TrimSpace(row.EntryDate) == bar,
			Bar:                bar,
			AsOf:               asOf,
			Policy:             pol,
		}))
	}
	return sortItems(out)
}

func finitePrice(v float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	return v > 0
}

func quoteStale(fetched, asOf time.Time) bool {
	if fetched.IsZero() || asOf.IsZero() {
		return true
	}
	age := asOf.Sub(fetched)
	if age < -time.Minute {
		return true
	}
	if age < 0 {
		age = 0
	}
	return age > QuoteStaleAfter
}

func holdingDaysBetween(entry, bar string) (int, bool) {
	a, err1 := time.Parse("2006-01-02", strings.TrimSpace(entry))
	b, err2 := time.Parse("2006-01-02", strings.TrimSpace(bar))
	if err1 != nil || err2 != nil {
		return 0, false
	}
	if b.Before(a) {
		return 0, false
	}
	return int(b.Sub(a).Hours() / 24), true
}
