package qualitygate

import (
	"fmt"
	"strings"
)

func evalE1(plan PlanView, cfg Config) []Finding {
	if !cfg.RequireEntryPrice {
		return []Finding{passFinding(RuleE1, CodeEntryPriceMissing, "entry price check skipped", nil)}
	}
	missing := make([]string, 0)
	for _, it := range plan.Items {
		if !isBuy(it.Side) {
			continue
		}
		if it.LimitPrice <= 0 {
			missing = append(missing, it.StockCode)
		}
	}
	if len(missing) == 0 {
		return []Finding{passFinding(RuleE1, CodeEntryPriceMissing, "all buy items have limit_price", map[string]any{
			"checked_items": len(plan.Items),
		})}
	}
	return []Finding{failFinding(RuleE1, CodeEntryPriceMissing,
		fmt.Sprintf("buy items missing limit_price: %s", strings.Join(missing, ",")),
		map[string]any{"codes": missing, "count": len(missing)})}
}

func evalI1(plan PlanView, positions []AccountPosition) []Finding {
	held := map[string]float64{}
	for _, p := range positions {
		code := strings.TrimSpace(p.StockCode)
		if code == "" || p.Volume == 0 {
			continue
		}
		held[code] += p.Volume
	}
	overlap := make([]string, 0)
	for _, it := range plan.Items {
		code := strings.TrimSpace(it.StockCode)
		if code == "" {
			continue
		}
		if vol, ok := held[code]; ok && vol != 0 {
			overlap = append(overlap, code)
		}
	}
	if len(overlap) == 0 {
		return []Finding{passFinding(RuleI1, CodePositionConflict, "no position overlap", map[string]any{
			"positions": len(held),
		})}
	}
	return []Finding{failFinding(RuleI1, CodePositionConflict,
		fmt.Sprintf("plan codes overlap positions: %s", strings.Join(overlap, ",")),
		map[string]any{"codes": overlap})}
}

func evalG1(plan PlanView, md MarketDataSnapshot, cfg Config) []Finding {
	type agg struct {
		count  int
		amount float64
	}
	byInd := map[string]*agg{}
	var total float64
	unknown := 0
	for _, it := range plan.Items {
		if !isBuy(it.Side) {
			continue
		}
		ind := resolveIndustry(it, md)
		amt := itemAmount(plan, it)
		total += amt
		if ind == "" {
			unknown++
			continue
		}
		a := byInd[ind]
		if a == nil {
			a = &agg{}
			byInd[ind] = a
		}
		a.count++
		a.amount += amt
	}
	if total <= 0 {
		return []Finding{passFinding(RuleG1, CodeSectorConcentration, "no buy amount to concentrate", nil)}
	}

	type hit struct {
		industry string
		share    float64
		count    int
	}
	hits := make([]hit, 0)
	for ind, a := range byInd {
		share := a.amount / total
		if share > cfg.SectorWarnAmountShare || a.count > cfg.SectorWarnNameCount {
			hits = append(hits, hit{industry: ind, share: share, count: a.count})
		}
	}
	if len(hits) == 0 {
		return []Finding{passFinding(RuleG1, CodeSectorConcentration, "sector concentration within warn thresholds", map[string]any{
			"unknown_industry_items": unknown,
			"sectors":                len(byInd),
		})}
	}
	// MVP (6.5.6.3 request): G1 is WARN only (not hard BLOCK).
	top := hits[0]
	for _, h := range hits[1:] {
		if h.share > top.share || (h.share == top.share && h.count > top.count) {
			top = h
		}
	}
	details := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		details = append(details, map[string]any{
			"industry": h.industry, "amount_share": h.share, "name_count": h.count,
		})
	}
	return []Finding{warnFinding(RuleG1, CodeSectorConcentration,
		fmt.Sprintf("sector concentration high: %s share=%.2f names=%d", top.industry, top.share, top.count),
		map[string]any{
			"hits":                   details,
			"warn_amount_share":      cfg.SectorWarnAmountShare,
			"warn_name_count":        cfg.SectorWarnNameCount,
			"unknown_industry_items": unknown,
		})}
}

func evalM1(plan PlanView, md MarketDataSnapshot) []Finding {
	if md.SkipGapEval {
		return []Finding{passFinding(RuleM1, CodeOpenGapPending, "gap eval skipped", nil)}
	}
	buyCodes := make([]string, 0)
	for _, it := range plan.Items {
		if isBuy(it.Side) && strings.TrimSpace(it.StockCode) != "" {
			buyCodes = append(buyCodes, it.StockCode)
		}
	}
	if len(buyCodes) == 0 {
		return []Finding{passFinding(RuleM1, CodeOpenGapPending, "no buy items for gap check", nil)}
	}
	missing := make([]string, 0)
	for _, code := range buyCodes {
		if md.OpenPriceByCode == nil {
			missing = append(missing, code)
			continue
		}
		if px, ok := md.OpenPriceByCode[code]; !ok || px <= 0 {
			missing = append(missing, code)
		}
	}
	if len(missing) == 0 {
		return []Finding{passFinding(RuleM1, CodeOpenGapPending, "open prices present for all buy items", map[string]any{
			"codes": len(buyCodes),
		})}
	}
	return []Finding{warnFinding(RuleM1, CodeOpenGapPending,
		fmt.Sprintf("open gap evaluation pending for %d codes", len(missing)),
		map[string]any{"missing_open_price_codes": missing})}
}

func evalP1(plan PlanView, md MarketDataSnapshot, cfg Config) []Finding {
	if len(plan.Items) == 0 {
		return []Finding{warnFinding(RuleP1, CodePlanCompleteness, "plan has no items", map[string]any{
			"completeness_ratio": 0.0,
		})}
	}
	incomplete := make([]map[string]any, 0)
	complete := 0
	for _, it := range plan.Items {
		nameOK := resolveName(it, md) != ""
		indOK := resolveIndustry(it, md) != ""
		anchorOK := resolveAnchor(it, md) > 0
		ok := nameOK && indOK && anchorOK
		if ok {
			complete++
			continue
		}
		incomplete = append(incomplete, map[string]any{
			"stock_code":     it.StockCode,
			"has_name":       nameOK,
			"has_industry":   indOK,
			"has_anchor_px":  anchorOK,
		})
	}
	ratio := float64(complete) / float64(len(plan.Items))
	if ratio >= cfg.MinCompletenessRatio && len(incomplete) == 0 {
		return []Finding{passFinding(RuleP1, CodePlanCompleteness, "plan items complete", map[string]any{
			"completeness_ratio": ratio,
		})}
	}
	if ratio >= cfg.MinCompletenessRatio {
		// some incomplete but ratio ok — still WARN listing incomplete
		return []Finding{warnFinding(RuleP1, CodePlanCompleteness,
			fmt.Sprintf("plan completeness partial ratio=%.2f", ratio),
			map[string]any{"completeness_ratio": ratio, "incomplete": incomplete})}
	}
	return []Finding{warnFinding(RuleP1, CodePlanCompleteness,
		fmt.Sprintf("plan completeness below threshold ratio=%.2f", ratio),
		map[string]any{
			"completeness_ratio": ratio,
			"min_ratio":          cfg.MinCompletenessRatio,
			"incomplete":         incomplete,
		})}
}

func isBuy(side string) bool {
	s := strings.ToLower(strings.TrimSpace(side))
	return s == "" || s == "buy"
}

func itemAmount(plan PlanView, it ItemView) float64 {
	if it.TargetAmount > 0 {
		return it.TargetAmount
	}
	if plan.AmountPerStock > 0 {
		return plan.AmountPerStock
	}
	return 100_000
}

func resolveIndustry(it ItemView, md MarketDataSnapshot) string {
	if s := strings.TrimSpace(it.Industry); s != "" {
		return s
	}
	if md.IndustryByCode != nil {
		if s := strings.TrimSpace(md.IndustryByCode[it.StockCode]); s != "" {
			return s
		}
	}
	return ""
}

func resolveName(it ItemView, md MarketDataSnapshot) string {
	if s := strings.TrimSpace(it.StockName); s != "" {
		return s
	}
	if md.NameByCode != nil {
		if s := strings.TrimSpace(md.NameByCode[it.StockCode]); s != "" {
			return s
		}
	}
	return ""
}

func resolveAnchor(it ItemView, md MarketDataSnapshot) float64 {
	if it.LimitPrice > 0 {
		return it.LimitPrice
	}
	if md.AnchorPriceByCode != nil {
		if v := md.AnchorPriceByCode[it.StockCode]; v > 0 {
			return v
		}
	}
	return 0
}

func failFinding(rule, code, msg string, evidence map[string]any) Finding {
	return Finding{Passed: false, Severity: SeverityFAIL, RuleCode: rule, Code: code, Message: msg, Evidence: evidence}
}

func warnFinding(rule, code, msg string, evidence map[string]any) Finding {
	return Finding{Passed: false, Severity: SeverityWARN, RuleCode: rule, Code: code, Message: msg, Evidence: evidence}
}

func passFinding(rule, code, msg string, evidence map[string]any) Finding {
	return Finding{Passed: true, Severity: SeverityPASS, RuleCode: rule, Code: code, Message: msg, Evidence: evidence}
}
