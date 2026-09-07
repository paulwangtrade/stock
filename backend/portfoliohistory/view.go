package portfoliohistory

import (
	"strings"
	"time"
)

// ViewOptions filters the history strip for Dashboard.
type ViewOptions struct {
	AsOf time.Time
	// FromTradeDate / ToTradeDate inclusive YYYY-MM-DD; empty = no bound.
	FromTradeDate string
	ToTradeDate   string
	// MaxDays caps newest-first trim after filter (0 = no cap).
	MaxDays int
}

// BuildView assembles PortfolioHistoryView from a Store (read-only).
func BuildView(store Store, opts ViewOptions) *PortfolioHistoryView {
	asOf := opts.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}
	view := &PortfolioHistoryView{
		SchemaVersion:     SchemaVersion,
		AsOf:              asOf,
		RecordOnly:        true,
		ReadOnly:          true,
		NotABacktest:      true,
		NotPnL:            true,
		NotSharpe:         true,
		NotAutoTune:       true,
		NotATradePlan:     true,
		NotExecution:      true,
		NotProviderSwitch: true,
		AnalysisToolOnly:  true,
		Disclaimer:        DisclaimerZH,
		DisclaimerKey:     DisclaimerKey,
		Days:              []DailyRecord{},
		DataGaps:          []string{},
		DataSourceNote:    "portfoliohistory.BuildView · daily observation summaries; not backtest/pnl; not TradePlan/Execution/Provider switch",
	}

	if store == nil {
		view.DataGaps = append(view.DataGaps, "store_missing")
		return view
	}

	from := strings.TrimSpace(opts.FromTradeDate)
	to := strings.TrimSpace(opts.ToTradeDate)
	days := store.List()
	filtered := make([]DailyRecord, 0, len(days))
	for _, d := range days {
		td := strings.TrimSpace(d.TradeDate)
		if from != "" && td < from {
			continue
		}
		if to != "" && td > to {
			continue
		}
		filtered = append(filtered, d)
	}

	if opts.MaxDays > 0 && len(filtered) > opts.MaxDays {
		// Keep the newest MaxDays (list is ascending by date).
		filtered = filtered[len(filtered)-opts.MaxDays:]
	}

	view.Days = filtered
	view.Rollup = rollup(filtered)
	if len(filtered) == 0 {
		view.DataGaps = append(view.DataGaps, "no_history_days")
	}
	return view
}

func rollup(days []DailyRecord) HistoryRollup {
	r := HistoryRollup{DayCount: len(days)}
	for _, d := range days {
		if d.Validation.Present && !d.Validation.Skipped {
			r.ValidationPresentDays++
			if d.Validation.OK {
				r.ValidationOKDays++
			}
		}
		if d.Risk.Present && d.Risk.Found {
			r.RiskFoundDays++
		}
		if d.AllocationShadow.Present {
			r.ShadowPresentDays++
			if d.AllocationShadow.Comparable {
				r.ShadowComparableDays++
			}
		}
	}
	return r
}
