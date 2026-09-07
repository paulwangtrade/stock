package readmodel

import (
	"strings"
	"time"

	"go-stock/backend/marketdata"
	"go-stock/backend/portfolio"
	"go-stock/backend/portfolio/positionstate"
	"go-stock/backend/tradingcalendar"
)

// Options for a pure projection (no DB I/O except caller-supplied quotes fetch).
type Options struct {
	Snapshot       *portfolio.Snapshot
	TradeDate      string
	AsOf           time.Time
	IncludeDisplay bool
	Quotes         map[string]marketdata.Quote // optional; keyed by stock_code
	BuyLotsByCode  map[string][]positionstate.LotRecord
	SellLotsByCode map[string][]positionstate.LotRecord
}

// Build projects View from an already-loaded Snapshot. No writes.
// Accounting equity = cash + Σ(mark_price × total_qty). Display overlay never changes those fields.
func Build(opts Options) *View {
	asOf := opts.AsOf
	if asOf.IsZero() {
		if opts.Snapshot != nil && !opts.Snapshot.AsOf.IsZero() {
			asOf = opts.Snapshot.AsOf
		} else {
			asOf = time.Now()
		}
	}
	td := strings.TrimSpace(opts.TradeDate)
	if td == "" {
		td = tradingcalendar.FormatDate(asOf)
	}
	out := emptyView(asOf, td)
	if opts.Snapshot == nil || !opts.Snapshot.Found {
		return out
	}

	out.Found = true
	out.Cash = opts.Snapshot.Cash
	out.AccountID = opts.Snapshot.AccountID
	out.AccountName = opts.Snapshot.AccountName

	rows := make([]PositionView, 0, len(opts.Snapshot.Positions))
	var mv float64
	for _, p := range opts.Snapshot.Positions {
		qty := p.Volume
		if qty <= 0 {
			continue
		}
		code := strings.TrimSpace(p.StockCode)
		rowMV := p.MarkPrice * float64(qty)
		rowPnL := (p.MarkPrice - p.AvgCost) * float64(qty)
		mv += rowMV

		locked := p.LockedVolume
		ps := positionstate.Calculate(positionstate.SnapshotInput{
			Symbol:       code,
			TotalQty:     qty,
			AvailableQty: p.AvailableVolume,
			LockedQty:    &locked,
			BuyRecords:   lotsFor(opts.BuyLotsByCode, code),
			SellRecords:  lotsFor(opts.SellLotsByCode, code),
			TradeDate:    td,
			CurrentDate:  td,
		})

		row := PositionView{
			StockCode:     code,
			StockName:     strings.TrimSpace(p.StockName),
			TotalQty:      qty,
			AvailableQty:  p.AvailableVolume,
			LockedQty:     p.LockedVolume,
			AvgCost:       p.AvgCost,
			MarkPrice:     p.MarkPrice,
			MarketValue:   rowMV,
			PnL:           rowPnL,
			PnLPercent:    pnlPercent(p.MarkPrice, p.AvgCost),
			PositionState: ps,
		}
		if opts.IncludeDisplay {
			applyDisplay(&row, opts.Quotes, asOf)
		}
		rows = append(rows, row)
	}

	out.MarketValue = mv
	out.Equity = out.Cash + mv
	out.Positions = rows
	out.PositionCount = len(rows)
	return out
}

func emptyView(asOf time.Time, tradeDate string) *View {
	return &View{
		Found:          false,
		Positions:      []PositionView{},
		UpdatedAt:      asOf,
		AsOf:           asOf,
		TradeDate:      tradeDate,
		DataSourceNote: dataSourceNote,
		Disclaimer:     disclaimer,
	}
}

func pnlPercent(mark, avg float64) *float64 {
	if avg <= 0 {
		return nil
	}
	v := (mark - avg) / avg
	return &v
}

// applyDisplay adds GET-time quote overlay. Never mutates MarkPrice / MarketValue / PnL / equity.
func applyDisplay(row *PositionView, quotes map[string]marketdata.Quote, asOf time.Time) {
	display := row.MarkPrice
	source := QuoteSourcePersisted
	var preClose float64
	var qPtr *marketdata.Quote
	if q := findQuote(quotes, row.StockCode); q != nil {
		qPtr = q
		if q.Price > 0 {
			display = q.Price
			source = QuoteSourceLive
		} else if q.Open > 0 {
			display = q.Open
			source = QuoteSourceOpenFallback
		}
		if q.PreClose > 0 {
			preClose = q.PreClose
			pc := q.PreClose
			row.QuotePreClose = &pc
		}
	}
	mv := display * float64(row.TotalQty)
	pnl := (display - row.AvgCost) * float64(row.TotalQty)
	row.DisplayPrice = &display
	row.DisplayQuoteSource = source
	row.DisplayMarketValue = &mv
	row.DisplayPnL = &pnl
	row.DisplayPnLPercent = pnlPercent(display, row.AvgCost)
	// Phase16.27-P0 / Phase17.2: single-name today PnL vs yesterday close (quote). Display-only.
	if display > 0 && preClose > 0 && row.TotalQty > 0 && source != QuoteSourcePersisted {
		tp := (display - preClose) * float64(row.TotalQty)
		row.TodayPnL = &tp
	}
	// Phase17.2 truth-layer: quote_price only when live/open exists — never alias mark as quote.
	applyQuoteTruth(row, qPtr, source, display, asOf)
}

func applyQuoteTruth(row *PositionView, q *marketdata.Quote, source string, display float64, asOf time.Time) {
	if source == QuoteSourcePersisted || display <= 0 {
		row.PriceFreshness = PriceFreshnessUnknown
		return
	}
	qp := display
	row.QuotePrice = &qp
	ts := resolveQuoteTimestamp(q)
	if ts != nil {
		row.QuoteTimestamp = ts
	}
	row.PriceFreshness = classifyPriceFreshness(ts, asOf)
}

func resolveQuoteTimestamp(q *marketdata.Quote) *time.Time {
	if q == nil {
		return nil
	}
	if !q.FetchedAt.IsZero() {
		t := q.FetchedAt
		return &t
	}
	date := strings.TrimSpace(q.Date)
	tim := strings.TrimSpace(q.Time)
	if date == "" || tim == "" {
		return nil
	}
	// Best-effort parse of upstream Date+Time (local); display only.
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"20060102 15:04:05",
		"2006-01-02 15:04",
	} {
		if t, err := time.ParseInLocation(layout, date+" "+tim, time.Local); err == nil {
			return &t
		}
	}
	return nil
}

func classifyPriceFreshness(ts *time.Time, asOf time.Time) string {
	if ts == nil || ts.IsZero() {
		return PriceFreshnessUnknown
	}
	ref := asOf
	if ref.IsZero() {
		ref = time.Now()
	}
	age := ref.Sub(*ts)
	if age < 0 {
		age = -age
	}
	if age > defaultQuoteStaleAfter {
		return PriceFreshnessStale
	}
	return PriceFreshnessFresh
}

func findQuote(quotes map[string]marketdata.Quote, code string) *marketdata.Quote {
	if len(quotes) == 0 {
		return nil
	}
	if q, ok := quotes[strings.TrimSpace(code)]; ok {
		cp := q
		return &cp
	}
	list := make([]marketdata.Quote, 0, len(quotes))
	for _, q := range quotes {
		list = append(list, q)
	}
	return marketdata.FindQuote(list, code)
}

func lotsFor(m map[string][]positionstate.LotRecord, code string) []positionstate.LotRecord {
	if m == nil {
		return nil
	}
	key := strings.ToLower(strings.TrimSpace(code))
	if v, ok := m[key]; ok {
		return v
	}
	return m[strings.TrimSpace(code)]
}
