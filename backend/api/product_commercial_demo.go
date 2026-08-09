package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"go-stock/backend/entitlement"
	"go-stock/backend/featuregate"
)

// Plan / comparison catalog for Phase13-H Commercial Demo Flow (no payment).

type productPlanDTO struct {
	Code        string   `json:"code"`
	DisplayName string   `json:"display_name"`
	Tagline     string   `json:"tagline"`
	Highlights  []string `json:"highlights"`
	Reserved    bool     `json:"reserved,omitempty"`
	Current     bool     `json:"current,omitempty"`
}

type featureCompareRowDTO struct {
	Capability string `json:"capability"`
	Feature    string `json:"feature,omitempty"` // empty = Trading Plane / base
	Free       string `json:"free"`              // included | locked | —
	Pro        string `json:"pro"`
	Enterprise string `json:"enterprise"`
	ValueHint  string `json:"value_hint,omitempty"`
}

func commercialPlansCatalog(currentTier featuregate.Tier) []productPlanDTO {
	tier := featuregate.NormalizeTier(currentTier)
	plans := []productPlanDTO{
		{
			Code:        "free",
			DisplayName: "Free",
			Tagline:     "先跑通看盘与纸面交易节奏",
			Highlights: []string{
				"基础看盘（自选 / 行情 / K 线）",
				"纸面交易全链路（计划 → 冻结 → 模拟观察）",
				"高级能力入口可见但锁定",
			},
			Current: tier == featuregate.TierFree,
		},
		{
			Code:        "pro",
			DisplayName: "Pro",
			Tagline:     "看懂计划、风险与分析摘要",
			Highlights: []string{
				"Strategy Explanation（策略解释）",
				"Advanced Risk（高级风险报告）",
				"AI Analysis（本地 Context / Mock）",
				"未来 Backtest 入口",
			},
			Current: tier == featuregate.TierPro,
		},
		{
			Code:        "enterprise",
			DisplayName: "Enterprise",
			Tagline:     "机构多账户与实时信号（预留）",
			Highlights: []string{
				"Multi Account（多账户）",
				"Realtime Signal（实时信号）",
				"含 Pro 全部高级能力",
			},
			Reserved: true,
			Current:  tier == featuregate.TierEnterprise,
		},
	}
	return plans
}

func featureComparisonCatalog() []featureCompareRowDTO {
	return []featureCompareRowDTO{
		{
			Capability: "基础看盘",
			Free:       "included", Pro: "included", Enterprise: "included",
			ValueHint: "自选、行情、K 线",
		},
		{
			Capability: "纸面交易",
			Free:       "included", Pro: "included", Enterprise: "included",
			ValueHint: "TradePlan 生成 / 批准 / 冻结与模拟观察（不因套餐阻断）",
		},
		{
			Capability: "Strategy Explanation",
			Feature:    string(featuregate.FeatureAdvancedObservation),
			Free:       "locked", Pro: "included", Enterprise: "included",
			ValueHint: "解释计划为何入选，辅助理解而非下单",
		},
		{
			Capability: "Advanced Risk",
			Feature:    string(featuregate.FeatureAdvancedRisk),
			Free:       "locked", Pro: "included", Enterprise: "included",
			ValueHint: "组合 / 持仓 / 执行风险只读报告",
		},
		{
			Capability: "AI Analysis",
			Feature:    string(featuregate.FeatureAIAnalysis),
			Free:       "locked", Pro: "included", Enterprise: "included",
			ValueHint: "本地 Context 归纳；Mock Provider，无外部上传",
		},
		{
			Capability: "未来 Backtest",
			Feature:    string(featuregate.FeatureBacktest),
			Free:       "locked", Pro: "included", Enterprise: "included",
			ValueHint: "研究回测商业入口（预留闸控）",
		},
		{
			Capability: "Multi Account",
			Feature:    string(featuregate.FeatureMultiAccount),
			Free:       "—", Pro: "—", Enterprise: "included",
			ValueHint: "企业多账户上下文（预留）",
		},
		{
			Capability: "Realtime Signal",
			Feature:    string(featuregate.FeatureRealtimeSignal),
			Free:       "—", Pro: "—", Enterprise: "included",
			ValueHint: "企业实时信号旗舰（预留）",
		},
	}
}

func (h *ProductCapabilitiesHandler) handlePlans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"tier":            string(user.Tier),
		"plans":           commercialPlansCatalog(user.Tier),
		"upgrade_note":    "演示升级仅切换本地档位，不接支付、账号或云服务。",
		"payment_enabled": false,
	})
}

func (h *ProductCapabilitiesHandler) handleFeatureComparison(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "GET required",
		})
		return
	}
	user := shellUserFromRequest(r)
	rows := featureComparisonCatalog()
	// Annotate allowed for current tier via FeatureGate when feature key present.
	enriched := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m := map[string]any{
			"capability": row.Capability,
			"feature":    row.Feature,
			"free":       row.Free,
			"pro":        row.Pro,
			"enterprise": row.Enterprise,
			"value_hint": row.ValueHint,
		}
		if row.Feature != "" {
			d := featuregate.CanAccess(user, featuregate.Feature(row.Feature))
			m["allowed_now"] = d.Allowed
			m["gate_reason"] = string(d.Reason)
		} else {
			m["allowed_now"] = true
			m["gate_reason"] = "OK"
		}
		enriched = append(enriched, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"tier":    string(user.Tier),
		"rows":    enriched,
		"message": "Commercial demo comparison; Trading Plane base features stay available on Free.",
	})
}

func (h *ProductCapabilitiesHandler) handleUpgradeDemo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"code": ProductCodeMethodNotAllowed, "ok": false, "message": "POST required",
		})
		return
	}
	// Demo-only: acknowledge target plan; UI persists localStorage tier. No payment.
	var body struct {
		Target string `json:"target"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	target := featuregate.NormalizeTier(featuregate.Tier(strings.TrimSpace(body.Target)))
	if target == featuregate.TierEnterprise {
		writeJSON(w, http.StatusOK, map[string]any{
			"code": ProductCodeOK, "ok": true,
			"accepted": false,
			"target":   "enterprise",
			"message":  "Enterprise 为预留档位，演示流暂不激活；可先体验 Pro。",
			"payment_enabled": false,
		})
		return
	}
	if target != featuregate.TierPro && target != featuregate.TierFree {
		target = featuregate.TierPro
	}
	user := &featuregate.User{ID: "shell:" + string(target), Tier: target}
	_ = entitlement.Default().EnsureTierDefaults(user)
	writeJSON(w, http.StatusOK, map[string]any{
		"code": ProductCodeOK, "ok": true,
		"accepted":        true,
		"target":          string(target),
		"message":         "演示档位已确认（无支付）。请在本地切换 productTier 后刷新 FeatureGate。",
		"payment_enabled": false,
	})
}
