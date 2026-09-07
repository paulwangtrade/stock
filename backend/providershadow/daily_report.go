package providershadow

import (
	"math"
	"sort"
	"strings"
)

const (
	DailyReportSchema = "portfolio_shadow_daily.g12-1"

	// Analysis is observation-only. These flags are compile-time documentation in JSON.
	dailyRecordOnly         = true
	dailyNotATradePlan      = true
	dailyNotAProviderSwitch = true
)

// ReasonCount is a frequency bucket for constraint / allocation explainability.
type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// PortfolioShadowDailyReport aggregates ShadowComparisonRecord rows for one trade date.
// It is read-only analysis: never selects a provider, never writes TradePlan / Draft.
type PortfolioShadowDailyReport struct {
	SchemaVersion      string `json:"schema_version"`
	TradeDate          string `json:"trade_date"`
	RecordCount        int    `json:"record_count"`
	ComparableCount    int    `json:"comparable_count"`
	IncomparableCount  int    `json:"incomparable_count"`
	RecordOnly         bool   `json:"record_only"`
	NotATradePlan      bool   `json:"not_a_trade_plan"`
	NotAProviderSwitch bool   `json:"not_a_provider_switch"`
	DataSourceNote     string `json:"data_source_note"`

	// AvgLegacyStockCount is mean Legacy envelope line_count over comparable rows.
	AvgLegacyStockCount float64 `json:"avg_legacy_stock_count"`
	// AvgPortfolioStockCount is mean Portfolio envelope line_count over comparable rows.
	AvgPortfolioStockCount float64 `json:"avg_portfolio_stock_count"`
	// AvgAmountDelta is mean signed (portfolio - legacy) over both_actual amount diffs.
	AvgAmountDelta float64 `json:"avg_amount_delta"`
	// AvgAbsAmountDelta is mean |delta| over the same set (empty → 0).
	AvgAbsAmountDelta     float64 `json:"avg_abs_amount_delta"`
	AmountDiffSampleCount int     `json:"amount_diff_sample_count"`

	// OnlyLegacyStocks is the sorted unique union of report.symbols.only_legacy.
	OnlyLegacyStocks []string `json:"only_legacy_stocks"`
	// OnlyPortfolioStocks is the sorted unique union of report.symbols.only_portfolio.
	OnlyPortfolioStocks []string `json:"only_portfolio_stocks"`

	// TopConstraintImpacts ranks role-pair and known constraint-like decision reasons.
	TopConstraintImpacts []ReasonCount `json:"top_constraint_impacts"`
	// AllocationChangeReasons ranks decision-layer allocation reason transitions and amount kinds.
	AllocationChangeReasons []ReasonCount `json:"allocation_change_reasons"`
}

const dailyDataSourceNote = "PortfolioShadowDailyReport · read-only aggregate of ShadowComparisonRecord; does not select providers or write Draft"

// BuildPortfolioShadowDailyReport aggregates records for tradeDate (empty = all non-empty dates kept as-is per row filter).
// When tradeDate is non-empty, only rows with matching TradeDate are included.
func BuildPortfolioShadowDailyReport(records []ShadowComparisonRecord, tradeDate string) *PortfolioShadowDailyReport {
	tradeDate = strings.TrimSpace(tradeDate)
	out := &PortfolioShadowDailyReport{
		SchemaVersion:           DailyReportSchema,
		TradeDate:               tradeDate,
		RecordOnly:              dailyRecordOnly,
		NotATradePlan:           dailyNotATradePlan,
		NotAProviderSwitch:      dailyNotAProviderSwitch,
		DataSourceNote:          dailyDataSourceNote,
		OnlyLegacyStocks:        []string{},
		OnlyPortfolioStocks:     []string{},
		TopConstraintImpacts:    []ReasonCount{},
		AllocationChangeReasons: []ReasonCount{},
	}

	var (
		legacyLinesSum float64
		portLinesSum   float64
		deltaSum       float64
		absDeltaSum    float64
		deltaN         int
		onlyL          = map[string]struct{}{}
		onlyP          = map[string]struct{}{}
		constraintHits = map[string]int{}
		allocReasons   = map[string]int{}
		inferredDate   string
	)

	for _, rec := range records {
		if tradeDate != "" && strings.TrimSpace(rec.TradeDate) != tradeDate {
			continue
		}
		out.RecordCount++
		if inferredDate == "" {
			inferredDate = strings.TrimSpace(rec.TradeDate)
		}

		comparable := rec.Comparable
		if rec.Report != nil {
			comparable = rec.Report.Comparable
		}
		if !comparable {
			out.IncomparableCount++
			continue
		}
		out.ComparableCount++

		legacyN := rec.LegacySummary.LineCount
		portN := rec.PortfolioSummary.LineCount
		if rec.Report != nil {
			if legacyN == 0 {
				legacyN = rec.Report.Legacy.LineCount
			}
			if portN == 0 {
				portN = rec.Report.Portfolio.LineCount
			}
		}
		legacyLinesSum += float64(legacyN)
		portLinesSum += float64(portN)

		if rec.Report == nil {
			continue
		}
		for _, s := range rec.Report.Symbols.OnlyLegacy {
			if s = normSymbol(s); s != "" {
				onlyL[s] = struct{}{}
			}
		}
		for _, s := range rec.Report.Symbols.OnlyPortfolio {
			if s = normSymbol(s); s != "" {
				onlyP[s] = struct{}{}
			}
		}
		for _, a := range rec.Report.Amounts {
			if a.Kind != KindBothActual || a.Delta == nil {
				if a.Kind != "" && a.Kind != KindBothActual {
					allocReasons["amount_kind:"+a.Kind]++
				}
				continue
			}
			d := *a.Delta
			if math.IsNaN(d) || math.IsInf(d, 0) {
				continue
			}
			deltaSum += d
			absDeltaSum += math.Abs(d)
			deltaN++
		}
		for _, role := range rec.Report.Roles {
			pair := strings.TrimSpace(role.Pair)
			if pair == "" {
				pair = strings.TrimSpace(role.LegacyRole) + "_to_" + strings.TrimSpace(role.PortfolioRole)
			}
			if pair == "" || pair == "_to_" {
				continue
			}
			if isConstraintLikeRolePair(pair) {
				constraintHits["role:"+pair]++
			}
		}
		for _, reason := range rec.Report.Reasons {
			if reason.Layer != "decision" {
				continue
			}
			from := strings.TrimSpace(reason.LegacyText)
			to := strings.TrimSpace(reason.PortfolioText)
			if from == "" && to == "" {
				continue
			}
			key := from + "→" + to
			allocReasons["decision:"+key]++
			if isConstraintLikeReason(to) {
				constraintHits["alloc:"+to]++
			}
		}
	}

	if out.TradeDate == "" {
		out.TradeDate = inferredDate
	}
	if out.ComparableCount > 0 {
		n := float64(out.ComparableCount)
		out.AvgLegacyStockCount = legacyLinesSum / n
		out.AvgPortfolioStockCount = portLinesSum / n
	}
	out.AmountDiffSampleCount = deltaN
	if deltaN > 0 {
		out.AvgAmountDelta = deltaSum / float64(deltaN)
		out.AvgAbsAmountDelta = absDeltaSum / float64(deltaN)
	}
	out.OnlyLegacyStocks = sortedKeys(onlyL)
	out.OnlyPortfolioStocks = sortedKeys(onlyP)
	out.TopConstraintImpacts = topReasons(constraintHits, 10)
	out.AllocationChangeReasons = topReasons(allocReasons, 10)
	return out
}

// AnalyzeStore builds a daily report from a RecordStore list (read-only).
func AnalyzeStore(store RecordStore, tradeDate string) *PortfolioShadowDailyReport {
	if store == nil {
		return BuildPortfolioShadowDailyReport(nil, tradeDate)
	}
	return BuildPortfolioShadowDailyReport(store.List(), tradeDate)
}

func isConstraintLikeRolePair(pair string) bool {
	p := strings.ToLower(pair)
	return strings.Contains(p, "waitlist") ||
		strings.Contains(p, "rejected") ||
		strings.Contains(p, "absent")
}

func isConstraintLikeReason(reason string) bool {
	r := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case r == "":
		return false
	case strings.Contains(r, "already_holding"):
		return true
	case strings.Contains(r, "name_limit"):
		return true
	case strings.Contains(r, "sector_limit"):
		return true
	case strings.Contains(r, "below_min_order"):
		return true
	case strings.Contains(r, "capped_single"):
		return true
	case strings.Contains(r, "no_account"):
		return true
	case strings.Contains(r, "no_allocation"):
		return true
	default:
		return false
	}
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func topReasons(counts map[string]int, limit int) []ReasonCount {
	out := make([]ReasonCount, 0, len(counts))
	for reason, n := range counts {
		out = append(out, ReasonCount{Reason: reason, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Reason < out[j].Reason
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
