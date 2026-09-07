package portfolioinsight

import (
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/holdingdecision/rules"
)

func buildWhyReduce(holdings []rules.HoldingDecision) []WhyReduce {
	out := make([]WhyReduce, 0)
	if len(holdings) == 0 {
		return out
	}
	// stable order by symbol
	idxs := make([]int, len(holdings))
	for i := range holdings {
		idxs[i] = i
	}
	sort.Slice(idxs, func(i, j int) bool {
		return strings.ToLower(holdings[idxs[i]].Symbol) < strings.ToLower(holdings[idxs[j]].Symbol)
	})

	for _, i := range idxs {
		d := holdings[i]
		act := strings.ToUpper(strings.TrimSpace(d.FinalAction))
		if act == "" {
			act = strings.ToUpper(strings.TrimSpace(d.Action))
		}
		if act != rules.ActionReduce && act != rules.ActionExit {
			continue
		}
		codes := append([]string{}, d.ReasonCodes...)
		if len(codes) == 0 {
			for _, h := range d.RuleHits {
				if h.ReasonCode != "" {
					codes = append(codes, h.ReasonCode)
				}
			}
		}
		codes = sortedUnique(codes)
		plains := make([]string, 0, len(codes))
		for _, c := range codes {
			plains = append(plains, plainReason(c))
		}
		if len(plains) == 0 && strings.TrimSpace(d.Explanation) != "" {
			plains = append(plains, strings.TrimSpace(d.Explanation))
		}
		if len(plains) == 0 {
			plains = append(plains, plainReason(""))
		}

		bullets := evidenceBullets(d)
		note := ""
		if !d.ExecutableHint {
			note = "当前可能受 T+1 等约束影响可卖数量；本条仅为观察说明，不是下单指令"
		}

		out = append(out, WhyReduce{
			Symbol:            strings.ToLower(strings.TrimSpace(d.Symbol)),
			Headline:          reduceHeadline(act),
			ActionObserved:    act,
			PlainReasons:      plains,
			EvidenceBullets:   bullets,
			ExecutableNote:    note,
			Disclaimer:        DisclaimerZH,
			SourceReasonCodes: codes,
		})
	}
	return out
}

func evidenceBullets(d rules.HoldingDecision) []string {
	out := []string{}
	ev := d.Evidence
	if ev == nil {
		ev = map[string]any{}
	}
	add := func(label string, key string, fmtPct bool) {
		v, ok := ev[key]
		if !ok || v == nil {
			return
		}
		switch t := v.(type) {
		case float64:
			if fmtPct {
				out = append(out, fmt.Sprintf("%s：%.2f%%", label, t*100))
			} else {
				out = append(out, fmt.Sprintf("%s：%.4g", label, t))
			}
		case int:
			out = append(out, fmt.Sprintf("%s：%d", label, t))
		case int64:
			out = append(out, fmt.Sprintf("%s：%d", label, t))
		case string:
			if strings.TrimSpace(t) != "" {
				out = append(out, fmt.Sprintf("%s：%s", label, strings.TrimSpace(t)))
			}
		}
	}
	add("收益率", "return_rate", true)
	add("权重", "weight", true)
	add("持有天数", "holding_days", false)
	add("行业", "industry", false)

	// also surface rule evidence maps briefly
	for _, h := range d.RuleHits {
		if h.Evidence == nil {
			continue
		}
		if w, ok := h.Evidence["weight"].(float64); ok {
			out = append(out, fmt.Sprintf("规则 %s 权重证据：%.2f%%", h.RuleID, w*100))
		}
	}
	return sortedUnique(out)
}
