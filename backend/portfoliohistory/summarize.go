package portfoliohistory

import (
	"sort"
	"strings"
	"time"

	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/providershadow"
)

// DayInput is one explicit save request. Summaries are projected; raw reports are not stored whole.
type DayInput struct {
	// Enabled must be true to persist. Default false → no-op (DefaultEnabled).
	Enabled bool

	TradeDate   string
	RecordedAt  time.Time
	AccountHint string // optional hash8; do not pass raw account secrets

	Validation *portfoliovalidation.PortfolioValidationReport
	Risk       *portfoliorisk.PortfolioRiskSnapshot
	Shadow     *providershadow.ShadowComparisonRecord
	// ShadowView optional alternative when only Insight DTO is available.
	ShadowView *portfolioinsight.AllocationShadowView

	Notes []string
}

// SummarizeDay builds a DailyRecord without touching a store.
func SummarizeDay(in DayInput) DailyRecord {
	at := in.RecordedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	td := strings.TrimSpace(in.TradeDate)
	if td == "" && in.Risk != nil {
		td = strings.TrimSpace(in.Risk.TradeDate)
	}
	if td == "" && in.Shadow != nil {
		td = strings.TrimSpace(in.Shadow.TradeDate)
	}

	rec := DailyRecord{
		SchemaVersion: DaySchema,
		TradeDate:     td,
		RecordedAt:    at.UTC(),
		AccountHint:   strings.TrimSpace(in.AccountHint),
		DataGaps:      []string{},
		Notes: append([]string{
			"portfolio history day · observation summaries only",
			"not a backtest / not pnl / not auto-tune",
		}, in.Notes...),
	}

	rec.Validation = summarizeValidation(in.Validation, td)
	if !rec.Validation.Present {
		rec.DataGaps = append(rec.DataGaps, "validation_missing")
	} else if rec.Validation.Skipped {
		rec.DataGaps = append(rec.DataGaps, "validation_skipped")
	}

	rec.Risk = summarizeRisk(in.Risk)
	if !rec.Risk.Present {
		rec.DataGaps = append(rec.DataGaps, "risk_missing")
	} else if !rec.Risk.Found {
		rec.DataGaps = append(rec.DataGaps, "risk_not_found")
	}

	rec.AllocationShadow = summarizeShadow(in.Shadow, in.ShadowView)
	if !rec.AllocationShadow.Present {
		rec.DataGaps = append(rec.DataGaps, "allocation_shadow_missing")
	}

	if rec.TradeDate == "" {
		rec.DataGaps = append(rec.DataGaps, "trade_date_missing")
	}
	return rec
}

func summarizeValidation(rep *portfoliovalidation.PortfolioValidationReport, tradeDate string) ValidationDaySummary {
	if rep == nil {
		return ValidationDaySummary{Present: false, Note: "missing"}
	}
	out := ValidationDaySummary{Present: true}
	if rep.Skipped || !rep.Enabled {
		out.Skipped = true
		out.SkipReason = rep.SkipReason
		if out.SkipReason == "" && !rep.Enabled {
			out.SkipReason = "validation enabled=false"
		}
		return out
	}

	day := pickValidationDay(rep, tradeDate)
	if day == nil {
		// Fall back to aggregate means as observation hints (still not PnL).
		out.OK = rep.Summary.OKCount > 0
		out.NameCountDelta = int(rep.Summary.NameCountDeltaMean)
		out.NotionalDelta = rep.Summary.NotionalDeltaMean
		out.TightenCountPortfolio = rep.Summary.TightenCountPortfolioTotal
		out.FilterRejectTop = topReasons(rep.Summary.FilterRejectReasonTotals, 5)
		out.Note = "validation_day_unmatched; aggregate summary used"
		if tradeDate != "" {
			out.TradeDate = tradeDate
		}
		return out
	}

	out.OK = day.OK
	out.CaseID = day.CaseID
	out.TradeDate = day.TradeDate
	out.NameCountLegacy = day.Difference.NameCountLegacy
	out.NameCountPortfolio = day.Difference.NameCountPortfolio
	out.NameCountDelta = day.Difference.NameCountDelta
	out.LegacyBuyNotional = day.LegacyDecision.BuyNotionalSum
	out.PortfolioBuyNotional = day.PortfolioDecision.BuyNotionalSum
	out.NotionalDelta = day.PortfolioDecision.BuyNotionalSum - day.LegacyDecision.BuyNotionalSum
	out.TightenCountLegacy = day.Difference.RiskTightenCountLegacy
	out.TightenCountPortfolio = day.Difference.RiskTightenCountPortfolio
	out.SectorAvailable = day.Difference.SectorAvailable
	out.FilterRejectTop = topReasons(day.Difference.FilterRejectReasonsPortfolio, 5)
	if !day.OK && day.Error != "" {
		out.Note = day.Error
	}
	return out
}

func pickValidationDay(rep *portfoliovalidation.PortfolioValidationReport, tradeDate string) *portfoliovalidation.DayValidation {
	if rep == nil || len(rep.Days) == 0 {
		return nil
	}
	td := strings.TrimSpace(tradeDate)
	if td != "" {
		for i := range rep.Days {
			if strings.TrimSpace(rep.Days[i].TradeDate) == td {
				return &rep.Days[i]
			}
		}
		return nil
	}
	if len(rep.Days) == 1 {
		return &rep.Days[0]
	}
	return nil
}

func summarizeRisk(snap *portfoliorisk.PortfolioRiskSnapshot) RiskSnapshotSummary {
	if snap == nil {
		return RiskSnapshotSummary{Present: false, Note: "missing"}
	}
	out := RiskSnapshotSummary{
		Present:           true,
		Found:             snap.Found,
		InputsFingerprint: snap.InputsFingerprint,
	}
	if snap.Exposure.Available {
		out.GrossExposure = cloneFloat(snap.Exposure.GrossExposure)
		out.CashRatio = cloneFloat(snap.Exposure.CashRatio)
		out.HeadroomVsCap = cloneFloat(snap.Exposure.HeadroomVsCap)
	} else if snap.Exposure.Note != "" {
		out.Note = snap.Exposure.Note
	}
	if snap.Concentration.Available {
		out.Top1Weight = cloneFloat(snap.Concentration.Top1Weight)
		out.NameCount = snap.Concentration.NameCount
	}
	out.SectorAvailable = snap.Sector.Available
	if snap.Sector.Available {
		out.MaxSectorWeight = cloneFloat(snap.Sector.MaxSectorWeight)
	}
	return out
}

func summarizeShadow(rec *providershadow.ShadowComparisonRecord, view *portfolioinsight.AllocationShadowView) AllocationShadowSummary {
	if rec != nil {
		out := AllocationShadowSummary{
			Present:            true,
			Comparable:         rec.Comparable,
			ShadowFingerprint:  rec.Fingerprint,
			OnlyLegacyCount:    rec.ComparisonSummary.OnlyLegacyCount,
			OnlyPortfolioCount: rec.ComparisonSummary.OnlyPortfolioCount,
			CommonCount:        rec.ComparisonSummary.CommonCount,
		}
		out.LegacyLineCount = rec.LegacySummary.LineCount
		out.PortfolioLineCount = rec.PortfolioSummary.LineCount
		if rec.Report != nil {
			out.Comparable = rec.Report.Comparable
			if out.LegacyLineCount == 0 {
				out.LegacyLineCount = rec.Report.Legacy.LineCount
			}
			if out.PortfolioLineCount == 0 {
				out.PortfolioLineCount = rec.Report.Portfolio.LineCount
			}
			m := rec.Report.Metrics
			if out.OnlyLegacyCount == 0 && out.OnlyPortfolioCount == 0 && out.CommonCount == 0 {
				out.OnlyLegacyCount = m.OnlyLegacyCount
				out.OnlyPortfolioCount = m.OnlyPortfolioCount
				out.CommonCount = m.CommonCount
			}
		}
		insight := portfolioinsight.FromShadowRecord(rec)
		if insight != nil {
			out.LegacyBuyNotional = insight.LegacyBuyNotional
			out.PortfolioBuyNotional = insight.PortfolioBuyNotional
			out.BudgetBinding = insight.BudgetBinding
			out.TightenApplied = insight.TightenApplied
			out.SuggestHasPatches = insight.SuggestHasPatches
			out.GrossHeadroomBinding = insight.GrossHeadroomBinding
			out.SingleCapApplied = insight.SingleCapApplied
			out.BlockedNewEntries = insight.BlockedNewEntries
			if out.ShadowFingerprint == "" {
				out.ShadowFingerprint = insight.ShadowFingerprint
			}
		}
		return out
	}
	if view != nil && view.Present {
		return AllocationShadowSummary{
			Present:              true,
			LegacyBuyNotional:    cloneFloat(view.LegacyBuyNotional),
			PortfolioBuyNotional: cloneFloat(view.PortfolioBuyNotional),
			BudgetBinding:        view.BudgetBinding,
			TightenApplied:       view.TightenApplied,
			SuggestHasPatches:    view.SuggestHasPatches,
			GrossHeadroomBinding: view.GrossHeadroomBinding,
			SingleCapApplied:     view.SingleCapApplied,
			BlockedNewEntries:    view.BlockedNewEntries,
			ShadowFingerprint:    view.ShadowFingerprint,
		}
	}
	return AllocationShadowSummary{Present: false, Note: "missing"}
}

func topReasons(m map[string]int, limit int) []ReasonCount {
	if len(m) == 0 {
		return nil
	}
	out := make([]ReasonCount, 0, len(m))
	for r, n := range m {
		out = append(out, ReasonCount{Reason: r, Count: n})
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

func cloneFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
