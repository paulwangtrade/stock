package mobileprojection

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"go-stock/backend/portfolio/attention"
	"go-stock/backend/portfolioobservation"
)

var (
	forbiddenTokenRE = regexp.MustCompile(`(?i)(account_id|api[_-]?key|broker|secret|password|token|private[_-]?key)`)
	buySellRE        = regexp.MustCompile(`(?i)\b(BUY|SELL|AUTO_BUY|AUTO_SELL)\b`)
	qtyRE            = regexp.MustCompile(`(?i)\b(qty|quantity|suggest_sell_qty|target_amount|side)\b`)
)

const (
	summarySchema = "mobile_portfolio_summary.p13-v1"
	riskSchema    = "mobile_risk_summary.p13-v1"
	insightSchema = "ai_insight.p13-v1"
	alertsSchema  = "mobile_alerts.p13-v1"

	phaseObserve = "observe"
	cautionRisk  = "观察级别，非交易指令"
	cautionAI    = "观察说明，不构成买卖建议。"
)

// Build projects Input into MobileObservationProjection. Default OFF unless in.Enabled.
func Build(in Input) *MobileObservationProjection {
	at := in.UploadedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}

	out := &MobileObservationProjection{
		SchemaVersion: SchemaVersion,
		DeviceIDHash8: normalizeHash8(in.DeviceIDHash8),
		UploadedAt:      at,
		Enabled:         in.Enabled,
		RecordOnly:      true,
		ExportNote:      ExportNote,
	}

	if !in.Enabled {
		out.Skipped = true
		out.SkipReason = "disabled"
		out.TradeDate = resolveTradeDate(in)
		return out
	}

	out.Skipped = false
	out.TradeDate = resolveTradeDate(in)

	if in.Observation != nil {
		out.Summary = buildSummary(in)
		out.Risk = buildRisk(in)
	}
	if in.Attention != nil {
		out.Attention = buildAttention(in.Attention)
	}
	out.Insight = buildInsight(in)

	return out
}

func resolveTradeDate(in Input) string {
	if in.Observation != nil && strings.TrimSpace(in.Observation.TradeDate) != "" {
		return strings.TrimSpace(in.Observation.TradeDate)
	}
	if in.AIInsight != nil && strings.TrimSpace(in.AIInsight.TradeDate) != "" {
		return strings.TrimSpace(in.AIInsight.TradeDate)
	}
	if in.Attention != nil && strings.TrimSpace(in.Attention.TradeDate) != "" {
		return strings.TrimSpace(in.Attention.TradeDate)
	}
	return ""
}

func buildSummary(in Input) *MobilePortfolioSummary {
	obs := in.Observation
	if obs == nil {
		return nil
	}
	cur := obs.CurrentPortfolio
	gaps := append([]string{}, obs.DataGaps...)
	found := cur.Available

	nameCount := 0
	if cur.NameCount != nil {
		nameCount = *cur.NameCount
	} else if cur.Concentration.NameCount > 0 {
		nameCount = cur.Concentration.NameCount
	}

	var cashRatio, grossExposure, top1, headroom *float64
	if cur.Cash.Available && cur.Cash.CashRatio != nil {
		v := *cur.Cash.CashRatio
		cashRatio = &v
	}
	if cur.Exposure.Available {
		if cur.Exposure.GrossExposure != nil {
			v := *cur.Exposure.GrossExposure
			grossExposure = &v
		}
		if cur.Exposure.HeadroomVsCap != nil {
			v := *cur.Exposure.HeadroomVsCap
			headroom = &v
		}
	}
	if cur.Concentration.Available && cur.Concentration.Top1Weight != nil {
		v := *cur.Concentration.Top1Weight
		top1 = &v
	}

	if !found {
		gaps = appendUnique(gaps, "snapshot_missing")
	}

	decision := decisionAttention(in)
	quality := qualityOK
	if len(gaps) > 0 || !found {
		quality = qualityDegraded
	}

	return &MobilePortfolioSummary{
		SchemaVersion:     summarySchema,
		TradeDate:         obs.TradeDate,
		AsOf:              obs.AsOf,
		Found:             found,
		NameCount:         nameCount,
		CashRatio:         cashRatio,
		GrossExposure:     grossExposure,
		Top1Weight:        top1,
		HeadroomVsCap:     headroom,
		DecisionAttention: decision,
		TradingChain:      buildTradingChainObserve(obs),
		Quality:           quality,
		DataGaps:          uniqueSorted(gaps),
		DataSourceNote:    dataSourceNote,
	}
}

func buildTradingChainObserve(obs *portfolioobservation.PortfolioObservationView) MobileTradingChainObserve {
	reasons := []string{}
	for _, g := range obs.DataGaps {
		code := sanitizeCode(g)
		if code != "" {
			reasons = append(reasons, code)
		}
	}
	for _, w := range obs.Warnings {
		code := sanitizeCode(w)
		if code != "" {
			reasons = append(reasons, code)
		}
	}
	reasons = uniqueSorted(reasons)
	msg := "当前为组合观察阶段"
	if len(reasons) > 0 {
		msg = "观察阶段存在数据缺口或提示项"
	}
	return MobileTradingChainObserve{
		Phase:        phaseObserve,
		BlockReasons: reasons,
		MessageSafe:  sanitizeText(msg),
	}
}

func buildRisk(in Input) *MobileRiskSummary {
	obs := in.Observation
	if obs == nil {
		return nil
	}
	cur := obs.CurrentPortfolio
	gaps := append([]string{}, obs.DataGaps...)

	level := strings.ToLower(strings.TrimSpace(cur.RiskLevel))
	if level == "" {
		level = "unavailable"
	}
	label := sanitizeText(cur.RiskLevelLabel)
	if label == "" && level != "unavailable" {
		label = level
	}

	exposure := MobileRiskExposure{Available: false}
	if cur.Exposure.Available {
		exposure.Available = true
		if cur.Exposure.GrossExposure != nil {
			v := *cur.Exposure.GrossExposure
			exposure.GrossExposure = &v
		}
		if cur.Exposure.HeadroomVsCap != nil {
			v := *cur.Exposure.HeadroomVsCap
			exposure.HeadroomVsCap = &v
		}
		exposure.Note = sanitizeText(cur.Exposure.Note)
	} else if level == "unavailable" {
		gaps = appendUnique(gaps, "exposure_unavailable")
	}

	concentration := MobileRiskConcentration{Available: cur.Concentration.Available}
	if cur.Concentration.Available {
		if cur.Concentration.Top1Weight != nil {
			v := *cur.Concentration.Top1Weight
			concentration.Top1Weight = &v
		}
		nc := cur.Concentration.NameCount
		if nc > 0 {
			concentration.NameCount = &nc
		} else if cur.NameCount != nil {
			concentration.NameCount = cur.NameCount
		}
	}

	sector := MobileRiskSector{
		Available: cur.Sector.Available,
		CoverageNote: sanitizeText(cur.Sector.CoverageNote),
	}
	if cur.Sector.AllowSectorConstraint != nil {
		sector.AllowSectorConstraint = *cur.Sector.AllowSectorConstraint
	}
	if cur.Sector.MaxSectorWeight != nil {
		v := *cur.Sector.MaxSectorWeight
		sector.MaxSectorWeight = &v
	}

	codes := []string{}
	for _, r := range cur.RiskReasons {
		c := sanitizeCode(r)
		if c != "" {
			codes = append(codes, c)
		}
	}
	codes = uniqueSorted(codes)

	quality := qualityOK
	if level == "unavailable" || len(gaps) > 0 {
		quality = qualityDegraded
	}

	return &MobileRiskSummary{
		SchemaVersion:  riskSchema,
		TradeDate:      obs.TradeDate,
		AsOf:           obs.AsOf,
		RiskLevel:      level,
		RiskLevelLabel: label,
		Exposure:       exposure,
		Concentration:  concentration,
		Sector:         sector,
		ExplainCodes:   codes,
		Quality:        quality,
		DataGaps:       uniqueSorted(gaps),
		Caution:        cautionRisk,
	}
}

func buildInsight(in Input) *MobileAIInsightView {
	if in.AIInsight != nil {
		return mapAIInsight(in.AIInsight, in.Observation)
	}
	if in.Observation == nil {
		return &MobileAIInsightView{
			SchemaVersion: insightSchema,
			TradeDate:     resolveTradeDate(in),
			NarrativeMode: "unavailable",
			Available:     false,
			WhyBuyLimited: emptyExplain(cautionAI),
			WhyReducePosition: emptyExplain(cautionAI),
			WhyRiskElevated: emptyExplain(cautionAI),
			DataGaps:      []string{"ai_insight_missing"},
			DataSourceNote: dataSourceNote,
		}
	}
	return deriveInsightFromObservation(in.Observation)
}

func mapAIInsight(src *AIInsight, obs *portfolioobservation.PortfolioObservationView) *MobileAIInsightView {
	td := src.TradeDate
	asOf := src.AsOf
	if obs != nil {
		if td == "" {
			td = obs.TradeDate
		}
		if asOf.IsZero() {
			asOf = obs.AsOf
		}
	}
	mode := strings.TrimSpace(src.NarrativeMode)
	if mode == "" {
		if src.Available {
			mode = "rules_only"
		} else {
			mode = "unavailable"
		}
	}
	return &MobileAIInsightView{
		SchemaVersion:       insightSchema,
		TradeDate:           td,
		AsOf:                asOf,
		NarrativeMode:       mode,
		Available:           src.Available,
		WhyBuyLimited:       sanitizeExplainSection(src.WhyBuyLimited),
		WhyReducePosition:   sanitizeExplainSection(src.WhyReducePosition),
		WhyRiskElevated:     sanitizeExplainSection(src.WhyRiskElevated),
		Sources:             src.Sources,
		DataGaps:            uniqueSorted(src.DataGaps),
		EvidenceFingerprint: src.EvidenceFingerprint,
		DataSourceNote:      firstNonEmpty(src.DataSourceNote, dataSourceNote),
	}
}

func deriveInsightFromObservation(obs *portfolioobservation.PortfolioObservationView) *MobileAIInsightView {
	gaps := append([]string{}, obs.DataGaps...)
	buy := ExplainSection{Available: false, Caution: cautionAI, Paragraphs: []string{}, Bullets: []ExplainBullet{}}
	reduce := buy
	riskSec := buy

	if len(obs.DecisionExplain.WhyBuyLess) > 0 || len(obs.DecisionExplain.WhyPositionLimited) > 0 {
		buy.Available = true
		buy.Headline = "买入空间有限（观察）"
		for _, f := range append(obs.DecisionExplain.WhyBuyLess, obs.DecisionExplain.WhyPositionLimited...) {
			if !f.Available {
				continue
			}
			buy.Bullets = append(buy.Bullets, ExplainBullet{
				Code: sanitizeCode(f.Code), PlainText: sanitizeText(f.PlainText), Source: sanitizeCode(f.Source),
			})
		}
		if len(buy.Bullets) == 0 {
			buy.Available = false
			buy.UnavailableReason = "无可用解释因子"
		}
	} else {
		buy.UnavailableReason = "observation_explain_missing"
		gaps = appendUnique(gaps, "why_buy_limited_missing")
	}

	if len(obs.DecisionExplain.WhySuggestReduce) > 0 {
		reduce.Available = true
		reduce.Headline = "存在减仓观察项"
		for _, f := range obs.DecisionExplain.WhySuggestReduce {
			if !f.Available {
				continue
			}
			reduce.Bullets = append(reduce.Bullets, ExplainBullet{
				Code: sanitizeCode(f.Code), PlainText: sanitizeText(stripSymbolPrefix(f.PlainText)), Source: sanitizeCode(f.Source),
			})
		}
	} else {
		reduce.UnavailableReason = "无 REDUCE/EXIT 观察条目"
	}

	cur := obs.CurrentPortfolio
	if cur.RiskLevel != "" {
		riskSec.Available = true
		riskSec.Headline = firstNonEmpty(cur.RiskLevelLabel, cur.RiskLevel)
		for _, r := range cur.RiskReasons {
			riskSec.Bullets = append(riskSec.Bullets, ExplainBullet{
				Code: sanitizeCode(r), PlainText: sanitizeText(r), Source: "observation",
			})
		}
	} else {
		riskSec.UnavailableReason = "risk_level_missing"
		gaps = appendUnique(gaps, "why_risk_elevated_missing")
	}

	available := buy.Available || reduce.Available || riskSec.Available
	return &MobileAIInsightView{
		SchemaVersion:     insightSchema,
		TradeDate:         obs.TradeDate,
		AsOf:              obs.AsOf,
		NarrativeMode:     "rules_only",
		Available:         available,
		WhyBuyLimited:     buy,
		WhyReducePosition: reduce,
		WhyRiskElevated:   riskSec,
		Sources: AIInsightSources{
			Observation: true,
			Risk:        cur.Available,
		},
		DataGaps:            uniqueSorted(gaps),
		EvidenceFingerprint: fingerprintObservation(obs),
		DataSourceNote:      dataSourceNote,
	}
}

func buildAttention(src *attention.DailyAttentionView) *MobileAlertsView {
	if src == nil {
		return nil
	}
	items := make([]MobileAlertItem, 0, len(src.Items))
	for _, it := range src.Items {
		items = append(items, MobileAlertItem{
			ID:              sanitizeText(it.ID),
			ItemType:        sanitizeCode(it.ItemType),
			Priority:        it.Priority,
			Title:           sanitizeText(it.Title),
			Reason:          sanitizeText(it.Reason),
			Source:          sanitizeCode(it.Source),
			Severity:        clampSeverity(it.Severity),
			SuggestedAction: clampAction(it.SuggestedAction),
			StockCode:       optionalStockCode(it.StockCode),
		})
	}
	return &MobileAlertsView{
		SchemaVersion: alertsSchema,
		TradeDate:     src.TradeDate,
		AsOf:          src.AsOf,
		OverallAction: clampAction(src.OverallAction),
		Headline:      sanitizeText(src.Headline),
		Items:         items,
		Counts: MobileAlertCounts{
			Review: src.Counts.Review,
			Watch:  src.Counts.Watch,
			Hold:   src.Counts.Hold,
			Total:  src.Counts.Total,
		},
		Quality:       src.Quality,
		MissingInputs: uniqueSorted(src.MissingInputs),
		Disclaimer:    firstNonEmpty(sanitizeText(src.Disclaimer), DisclaimerZH),
	}
}

func decisionAttention(in Input) string {
	if in.Attention != nil {
		return clampAction(in.Attention.OverallAction)
	}
	if in.Observation == nil {
		return attention.ActionHold
	}
	switch strings.ToLower(in.Observation.CurrentPortfolio.RiskLevel) {
	case "high", "elevated":
		return attention.ActionReview
	case "moderate":
		return attention.ActionWatch
	default:
		return attention.ActionHold
	}
}

func clampAction(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case attention.ActionHold, attention.ActionWatch, attention.ActionReview:
		return strings.ToUpper(strings.TrimSpace(s))
	case "BUY", "SELL", "AUTO_BUY", "AUTO_SELL", "AUTO_ACTION":
		return attention.ActionReview
	default:
		if s == "" {
			return attention.ActionHold
		}
		return attention.ActionWatch
	}
}

func clampSeverity(s string) string {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case attention.SeverityInfo, attention.SeverityLow, attention.SeverityMedium, attention.SeverityHigh:
		return strings.ToUpper(strings.TrimSpace(s))
	default:
		return attention.SeverityInfo
	}
}

func sanitizeExplainSection(sec ExplainSection) ExplainSection {
	out := sec
	out.Headline = sanitizeText(out.Headline)
	out.UnavailableReason = sanitizeText(out.UnavailableReason)
	out.Caution = firstNonEmpty(sanitizeText(out.Caution), cautionAI)
	out.Paragraphs = sanitizeLines(out.Paragraphs)
	bullets := make([]ExplainBullet, 0, len(out.Bullets))
	for _, b := range out.Bullets {
		bullets = append(bullets, ExplainBullet{
			Code: sanitizeCode(b.Code), PlainText: sanitizeText(b.PlainText), Source: sanitizeCode(b.Source),
		})
	}
	out.Bullets = bullets
	return out
}

func emptyExplain(caution string) ExplainSection {
	return ExplainSection{
		Available: false, Caution: caution, Paragraphs: []string{}, Bullets: []ExplainBullet{},
	}
}

func sanitizeText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if forbiddenTokenRE.MatchString(s) {
		return ""
	}
	if buySellRE.MatchString(s) {
		s = buySellRE.ReplaceAllString(s, "REVIEW")
	}
	if qtyRE.MatchString(s) {
		return ""
	}
	return s
}

func sanitizeCode(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || forbiddenTokenRE.MatchString(s) {
		return ""
	}
	return strings.ToUpper(strings.ReplaceAll(s, " ", "_"))
}

func sanitizeLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if t := sanitizeText(l); t != "" {
			out = append(out, t)
		}
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func stripSymbolPrefix(s string) string {
	s = sanitizeText(s)
	if i := strings.Index(s, ":"); i >= 0 && i < 12 {
		return strings.TrimSpace(s[i+1:])
	}
	return s
}

func optionalStockCode(code string) string {
	code = strings.TrimSpace(code)
	if code == "" || forbiddenTokenRE.MatchString(code) {
		return ""
	}
	return code
}

func normalizeHash8(h string) string {
	h = strings.TrimSpace(h)
	if len(h) >= 8 {
		return strings.ToUpper(h[:8])
	}
	return strings.ToUpper(h)
}

func fingerprintObservation(obs *portfolioobservation.PortfolioObservationView) string {
	if obs == nil {
		return ""
	}
	h := sha256.Sum256([]byte(obs.SchemaVersion + obs.TradeDate + obs.AsOf.Format(time.RFC3339Nano)))
	return hex.EncodeToString(h[:8])
}

const qualityOK = "OK"
const qualityDegraded = "DEGRADED"

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func appendUnique(slice []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return slice
	}
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}

func uniqueSorted(in []string) []string {
	m := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		m[s] = struct{}{}
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
