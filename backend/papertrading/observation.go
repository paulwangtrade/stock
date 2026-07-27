// @Author spark
// @Date 2026/7/27
// @Desc Phase7-A3-1：Paper Observation Overlay — Position + QuoteService → 展示价/浮盈（只读，不写库）

package papertrading

import (
	"strings"
	"time"

	"go-stock/backend/data"
	"go-stock/backend/marketdata"
)

// Quote overlay sources for observation rows.
const (
	QuoteSourceLive         = "live"
	QuoteSourceOpenFallback = "open_fallback"
	QuoteSourcePersisted    = "persisted"
)

// observationQuoteFetcher is injectable for tests (defaults to data.GetQuoteService).
var observationQuoteFetcher = func() marketdata.QuoteService {
	return data.GetQuoteService()
}

// SetObservationQuoteServiceForTest injects QuoteService for observation tests. Pass nil to restore.
func SetObservationQuoteServiceForTest(svc marketdata.QuoteService) {
	if svc == nil {
		observationQuoteFetcher = func() marketdata.QuoteService {
			return data.GetQuoteService()
		}
		return
	}
	observationQuoteFetcher = func() marketdata.QuoteService { return svc }
}

// resolveDisplayPrice picks observation mark: Quote.Price → Quote.Open → persisted MarkPrice.
// Never writes DB. Returns source and optional quote timestamp.
func resolveDisplayPrice(persistedMark float64, q *marketdata.Quote) (display float64, source string, updatedAt *time.Time) {
	display = persistedMark
	source = QuoteSourcePersisted
	if q == nil {
		return display, source, nil
	}
	var ts *time.Time
	if !q.FetchedAt.IsZero() {
		t := q.FetchedAt
		ts = &t
	}
	if q.Price > 0 {
		return q.Price, QuoteSourceLive, ts
	}
	if q.Open > 0 {
		return q.Open, QuoteSourceOpenFallback, ts
	}
	return display, source, nil
}

func deriveObservationMetrics(displayPrice, avgCost float64, volume int64) (marketValue, unrealizedPnl, returnRate float64) {
	marketValue = displayPrice * float64(volume)
	unrealizedPnl = (displayPrice - avgCost) * float64(volume)
	if avgCost > 0 {
		returnRate = (displayPrice - avgCost) / avgCost
	}
	return marketValue, unrealizedPnl, returnRate
}

// fetchObservationQuotes batch-loads quotes; nil service / empty / error → empty map (all fallback).
func fetchObservationQuotes(svc marketdata.QuoteService, codes []string) map[string]marketdata.Quote {
	out := make(map[string]marketdata.Quote)
	if svc == nil || len(codes) == 0 {
		return out
	}
	quotes, err := svc.GetQuotes(codes)
	if err != nil || len(quotes) == 0 {
		return out
	}
	for i := range quotes {
		q := quotes[i]
		key := strings.TrimSpace(q.Code)
		if key != "" {
			out[key] = q
		}
	}
	return out
}

func findObservationQuote(byCode map[string]marketdata.Quote, stockCode string) *marketdata.Quote {
	if len(byCode) == 0 {
		return nil
	}
	if q, ok := byCode[strings.TrimSpace(stockCode)]; ok {
		cp := q
		return &cp
	}
	list := make([]marketdata.Quote, 0, len(byCode))
	for _, q := range byCode {
		list = append(list, q)
	}
	return marketdata.FindQuote(list, stockCode)
}

// BuildObservationPositionRows overlays QuoteService onto persisted positions (read-only).
// MarkPrice JSON field is set to DisplayPrice for UI compatibility; PersistedMarkPrice keeps DB mark.
func BuildObservationPositionRows(positions []PaperSimPosition, svc marketdata.QuoteService) []DashboardPositionRow {
	codes := make([]string, 0, len(positions))
	for _, p := range positions {
		if c := strings.TrimSpace(p.StockCode); c != "" {
			codes = append(codes, c)
		}
	}
	quotes := fetchObservationQuotes(svc, codes)

	rows := make([]DashboardPositionRow, 0, len(positions))
	for _, p := range positions {
		q := findObservationQuote(quotes, p.StockCode)
		display, source, qAt := resolveDisplayPrice(p.MarkPrice, q)
		mv, upnl, rr := deriveObservationMetrics(display, p.AvgCost, p.TotalVolume)
		rows = append(rows, DashboardPositionRow{
			StockCode:          p.StockCode,
			StockName:          p.StockName,
			TotalVolume:        p.TotalVolume,
			AvailableVolume:    p.AvailableVolume,
			LockedVolume:       p.LockedVolume,
			AvgCost:            p.AvgCost,
			MarkPrice:          display, // compat: UI「当前价」→ DisplayPrice
			PersistedMarkPrice: p.MarkPrice,
			DisplayPrice:       display,
			MarketValue:        mv,
			UnrealizedPnl:      upnl,
			ReturnRate:         rr,
			UpdatedAt:          p.UpdatedAt,
			QuoteSource:        source,
			QuoteUpdatedAt:     qAt,
			T1Locked:           p.LockedVolume > 0,
		})
	}
	return rows
}

// applyObservationAccountTotals fills optional observation account aggregates from rows.
func applyObservationAccountTotals(out *DashboardPositions, rows []DashboardPositionRow) {
	var obsMV, obsU float64
	live := 0
	for _, r := range rows {
		obsMV += r.MarketValue
		obsU += r.UnrealizedPnl
		if r.QuoteSource == QuoteSourceLive || r.QuoteSource == QuoteSourceOpenFallback {
			live++
		}
	}
	out.ObservationMarketValue = obsMV
	out.ObservationUnrealizedPnl = obsU
	out.ObservationEquity = out.Cash + obsMV
	out.QuoteOverlay = live > 0
	if out.QuoteOverlay {
		out.DataSourceNote = dashboardDataSourceNote + " · observation quote overlay (display only; DB mark unchanged)"
	}
}
