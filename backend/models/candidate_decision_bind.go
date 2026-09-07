package models

import "strings"

// EnrichCandidatePoolDecisionIDs Phase3-A：Rank 冻结后旁路绑定 DecisionID。
//
// 仅写入 DecisionID；不改 Score / Rank / StockCode / 切片顺序。
// decisionIDByCode key 按 stockCode 匹配（大小写不敏感）；缺失则保持空串。
// 不组装 Decision、不切换 producer、不触达 TradePlan / Execution。
func EnrichCandidatePoolDecisionIDs(items []CandidatePoolItem, decisionIDByCode map[string]string) {
	if len(items) == 0 || len(decisionIDByCode) == 0 {
		return
	}
	normalized := make(map[string]string, len(decisionIDByCode))
	for k, v := range decisionIDByCode {
		id := strings.TrimSpace(v)
		if id == "" {
			continue
		}
		normalized[normalizeCandidateCode(k)] = id
	}
	if len(normalized) == 0 {
		return
	}
	for i := range items {
		code := normalizeCandidateCode(items[i].StockCode)
		if id, ok := normalized[code]; ok {
			items[i].DecisionID = id
		}
	}
}

func normalizeCandidateCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}
