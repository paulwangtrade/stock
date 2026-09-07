package portfolioobservation

import (
	"strings"
	"time"

	"go-stock/backend/portfolioinsight"
	"go-stock/backend/portfoliorisk"
	"go-stock/backend/portfoliovalidation"
	"go-stock/backend/sectorcoverage"
	"go-stock/backend/sellsuggestion"
)

// Assemble projects optional read-only sources into PortfolioObservationView.
// Pure function: no I/O, no TradePlan, no Execution, no Provider mutation.
func Assemble(in Input) *PortfolioObservationView {
	asOf := in.AsOf
	if asOf.IsZero() {
		if in.Risk != nil && !in.Risk.AsOf.IsZero() {
			asOf = in.Risk.AsOf
		} else if in.Insight != nil && !in.Insight.AsOf.IsZero() {
			asOf = in.Insight.AsOf
		} else {
			asOf = time.Now().UTC()
		}
	}
	tradeDate := strings.TrimSpace(in.TradeDate)
	if tradeDate == "" && in.Risk != nil {
		tradeDate = in.Risk.TradeDate
	}
	if tradeDate == "" && in.Insight != nil {
		tradeDate = in.Insight.TradeDate
	}

	gaps := []string{}
	out := &PortfolioObservationView{
		SchemaVersion:     SchemaVersion,
		AsOf:              asOf,
		TradeDate:         tradeDate,
		AccountID:         strings.TrimSpace(in.AccountID),
		RecordOnly:        true,
		ReadOnly:          true,
		NotTradingAdvice:  true,
		NotAutoTrade:      true,
		NotATradePlan:     true,
		NotOrder:          true,
		NotExecution:      true,
		NotProviderSwitch: true,
		AnalysisToolOnly:  true,
		Disclaimer:        DisclaimerZH,
		DisclaimerKey:     DisclaimerKey,
		DataSourceNote:    dataSourceNote,
		Warnings:          append([]string{}, in.Warnings...),
		DecisionExplain: DecisionExplain{
			WhyBuyLess:         []ExplainFactor{},
			WhyPositionLimited: []ExplainFactor{},
			WhySuggestReduce:   []ExplainFactor{},
			ReduceRows:         []ReduceSuggestionRow{},
		},
	}

	out.Sources = SourceFlags{
		RiskPresent:       in.Risk != nil,
		InsightPresent:    in.Insight != nil,
		ValidationPresent: in.Validation != nil,
		SellPresent:       in.Sell != nil && !in.Sell.Skipped,
		SectorCovPresent:  in.SectorCov != nil,
	}
	if !out.Sources.RiskPresent {
		gaps = append(gaps, "portfolio_risk_missing")
	}
	if !out.Sources.InsightPresent {
		gaps = append(gaps, "portfolio_insight_missing")
	}
	if !out.Sources.ValidationPresent {
		gaps = append(gaps, "portfolio_validation_missing")
	} else if in.Validation.Skipped {
		gaps = append(gaps, "portfolio_validation_skipped")
	}
	if in.Sell == nil || in.Sell.Skipped {
		gaps = append(gaps, "sell_suggestion_missing_or_disabled")
	}
	if !out.Sources.SectorCovPresent {
		gaps = append(gaps, "sector_coverage_missing")
	}

	out.CurrentPortfolio = buildCurrent(in.Risk, in.Insight, in.SectorCov, &gaps)
	out.DecisionExplain = buildExplain(in.Insight, in.Risk, in.Sell, in.Validation, &gaps)
	out.Validation = buildValidationBrief(in.Validation)
	out.SectorCoverage = buildSectorCovBrief(in.SectorCov)

	out.DataGaps = uniqueSorted(gaps)
	return out
}

func buildCurrent(
	risk *portfoliorisk.PortfolioRiskSnapshot,
	insight *portfolioinsight.PortfolioInsight,
	cov *sectorcoverage.SectorCoverageReport,
	gaps *[]string,
) CurrentPortfolio {
	cur := CurrentPortfolio{
		Exposure:      ExposureView{Available: false, Note: "risk_missing"},
		Concentration: ConcentrationView{Available: false, Note: "risk_missing"},
		Sector:        SectorView{Available: false, Note: "risk_missing", Buckets: []SectorBucket{}},
		Cash:          CashView{Available: false, Note: "risk_missing"},
	}
	if risk == nil || !risk.Found {
		cur.Note = "portfolio_risk_unavailable"
		*gaps = appendUniqueGap(*gaps, "current_portfolio_unavailable")
		if insight != nil {
			cur.RiskLevel = insight.PortfolioSummary.RiskLevel
			cur.RiskLevelLabel = insight.PortfolioSummary.RiskLevelLabel
			cur.RiskReasons = append([]string{}, insight.PortfolioSummary.RiskLevelReasons...)
			cur.NameCount = insight.PortfolioSummary.NameCount
		}
		return cur
	}
	cur.Available = true

	ex := risk.Exposure
	cur.Exposure = ExposureView{
		Available:     ex.Available,
		GrossExposure: ex.GrossExposure,
		GrossNotional: ex.GrossNotional,
		Equity:        ex.Equity,
		HeadroomVsCap: ex.HeadroomVsCap,
		CapGross:      ex.CapGross,
		Note:          ex.Note,
	}
	cur.Cash = CashView{
		Available: ex.Available && ex.CashRatio != nil,
		CashRatio: ex.CashRatio,
		Note:      ex.Note,
	}
	if !cur.Cash.Available && insight != nil && insight.PortfolioSummary.CashRatio != nil {
		cur.Cash.Available = true
		cur.Cash.CashRatio = insight.PortfolioSummary.CashRatio
		cur.Cash.Note = insight.PortfolioSummary.CashRatioNote
	}

	cc := risk.Concentration
	cur.Concentration = ConcentrationView{
		Available:  cc.Available,
		Top1Weight: cc.Top1Weight,
		Top5Weight: cc.Top5Weight,
		NameCount:  cc.NameCount,
		CapSingle:  cc.CapSingle,
		Note:       cc.Note,
	}

	sec := risk.Sector
	cur.Sector = SectorView{
		Available: sec.Available,
		Note:      sec.Note,
		Buckets:   []SectorBucket{},
	}
	if sec.MaxSectorWeight != nil {
		v := *sec.MaxSectorWeight
		cur.Sector.MaxSectorWeight = &v
	}
	for _, b := range sec.SectorExposure {
		cur.Sector.Buckets = append(cur.Sector.Buckets, SectorBucket{
			Sector: b.Sector, Weight: b.Weight, NameCount: b.NameCount,
		})
	}
	if cov != nil {
		allow := cov.AllowSectorConstraint
		cur.Sector.AllowSectorConstraint = &allow
		cur.Sector.CoverageNote = cov.Note
		if !allow {
			// fail-closed: do not pretend sector constraint is on
			if cur.Sector.Note == "" {
				cur.Sector.Note = "sector_constraint_disallowed_by_coverage"
			}
		}
	}

	if insight != nil {
		cur.RiskLevel = insight.PortfolioSummary.RiskLevel
		cur.RiskLevelLabel = insight.PortfolioSummary.RiskLevelLabel
		cur.RiskReasons = append([]string{}, insight.PortfolioSummary.RiskLevelReasons...)
		cur.NameCount = insight.PortfolioSummary.NameCount
		if !cur.Sector.Available && insight.PortfolioSummary.SectorUnavailableNote != "" {
			cur.Sector.Note = insight.PortfolioSummary.SectorUnavailableNote
		}
	}
	return cur
}

func buildExplain(
	insight *portfolioinsight.PortfolioInsight,
	risk *portfoliorisk.PortfolioRiskSnapshot,
	sell *sellsuggestion.Report,
	validation *portfoliovalidation.PortfolioValidationReport,
	gaps *[]string,
) DecisionExplain {
	ex := DecisionExplain{
		WhyBuyLess:         []ExplainFactor{},
		WhyPositionLimited: []ExplainFactor{},
		WhySuggestReduce:   []ExplainFactor{},
		ReduceRows:         []ReduceSuggestionRow{},
	}

	if insight != nil && insight.WhyBuyLimited != nil {
		for _, f := range insight.WhyBuyLimited.Factors {
			factor := ExplainFactor{
				Code: f.Code, PlainText: f.PlainText, Available: f.Available, Source: "insight",
			}
			switch strings.ToUpper(f.Code) {
			case "GROSS", "CASH", "BLOCK_NEW", "BUDGET":
				ex.WhyBuyLess = append(ex.WhyBuyLess, factor)
			case "SINGLE_CAP", "SECTOR", "RESERVE":
				ex.WhyPositionLimited = append(ex.WhyPositionLimited, factor)
			default:
				ex.WhyBuyLess = append(ex.WhyBuyLess, factor)
			}
		}
		if insight.WhyBuyLimited.Headline != "" && len(ex.WhyBuyLess) == 0 {
			ex.WhyBuyLess = append(ex.WhyBuyLess, ExplainFactor{
				Code: "HEADLINE", PlainText: insight.WhyBuyLimited.Headline,
				Available: true, Source: "insight",
			})
		}
		for _, r := range insight.WhyReduce {
			ex.WhySuggestReduce = append(ex.WhySuggestReduce, ExplainFactor{
				Code:      firstCode(r.SourceReasonCodes, r.ActionObserved),
				PlainText: firstNonEmpty(r.Headline, strings.Join(r.PlainReasons, "；")),
				Available: true,
				Source:    "insight",
			})
		}
	} else {
		*gaps = appendUniqueGap(*gaps, "why_buy_less_partial_no_insight")
	}

	// Risk-derived position limits when insight missing factors.
	if risk != nil && risk.Found {
		if risk.Concentration.Available && risk.Concentration.Top1Weight != nil &&
			risk.Concentration.CapSingle != nil && *risk.Concentration.CapSingle > 0 &&
			*risk.Concentration.Top1Weight > *risk.Concentration.CapSingle+1e-12 {
			ex.WhyPositionLimited = appendUniqueFactor(ex.WhyPositionLimited, ExplainFactor{
				Code: "SINGLE_CAP", PlainText: "单票集中度已超过观察上限", Available: true, Source: "risk",
			})
		}
		if risk.Exposure.Available && risk.Exposure.HeadroomVsCap != nil && *risk.Exposure.HeadroomVsCap <= 1e-6 {
			ex.WhyBuyLess = appendUniqueFactor(ex.WhyBuyLess, ExplainFactor{
				Code: "GROSS", PlainText: "毛敞口已接近或触及上限，新买空间有限", Available: true, Source: "risk",
			})
		}
	}

	if sell != nil && !sell.Skipped {
		for _, s := range sell.Suggestions {
			if s.Action != sellsuggestion.ActionReduce && s.Action != sellsuggestion.ActionExit {
				continue
			}
			ex.ReduceRows = append(ex.ReduceRows, ReduceSuggestionRow{
				Symbol: s.Symbol, Action: s.Action, Reason: s.Reason,
				SuggestSellQty: s.SuggestSellQty, TargetWeight: s.TargetWeight, RiskReason: s.RiskReason,
			})
			text := s.Reason
			if text == "" {
				text = s.RiskReason
			}
			if text == "" {
				text = s.Action + " 观察建议"
			}
			ex.WhySuggestReduce = append(ex.WhySuggestReduce, ExplainFactor{
				Code: s.Action, PlainText: s.Symbol + ": " + text, Available: true, Source: "sell_suggestion",
			})
		}
	}

	if validation != nil && !validation.Skipped && validation.Summary.OKCount > 0 {
		if validation.Summary.NameCountDeltaMean < 0 {
			ex.WhyBuyLess = appendUniqueFactor(ex.WhyBuyLess, ExplainFactor{
				Code:      "VALIDATION_NAME_DELTA",
				PlainText: "历史验证：Portfolio 路径平均选股数少于 Legacy（观察）",
				Available: true,
				Source:    "validation",
			})
		}
		if validation.Summary.TightenCountPortfolioTotal > 0 {
			ex.WhyPositionLimited = appendUniqueFactor(ex.WhyPositionLimited, ExplainFactor{
				Code:      "VALIDATION_TIGHTEN",
				PlainText: "历史验证：Portfolio 路径出现 Risk Tighten（观察）",
				Available: true,
				Source:    "validation",
			})
		}
	}

	return ex
}

func buildValidationBrief(v *portfoliovalidation.PortfolioValidationReport) ValidationBrief {
	if v == nil {
		return ValidationBrief{Present: false, Note: "missing"}
	}
	out := ValidationBrief{
		Present: true,
		Skipped: v.Skipped,
		Note:    v.SkipReason,
	}
	if v.Skipped {
		return out
	}
	out.DayCount = v.Summary.DayCount
	out.OKCount = v.Summary.OKCount
	out.NameCountDeltaMean = v.Summary.NameCountDeltaMean
	out.NotionalDeltaMean = v.Summary.NotionalDeltaMean
	out.TightenCountPortfolio = v.Summary.TightenCountPortfolioTotal
	out.SectorComparableDays = v.Summary.SectorComparableDays
	return out
}

func buildSectorCovBrief(c *sectorcoverage.SectorCoverageReport) SectorCoverageBrief {
	if c == nil {
		return SectorCoverageBrief{Present: false, Note: "missing"}
	}
	return SectorCoverageBrief{
		Present:               true,
		HoldingsCoverage:      c.HoldingsCoverage,
		PoolCoverage:          c.PoolCoverage,
		UnknownCount:          c.UnknownCount,
		AllowSectorConstraint: c.AllowSectorConstraint,
		MissingCount:          len(c.IndustryClassificationMissing),
		Note:                  c.Note,
	}
}
