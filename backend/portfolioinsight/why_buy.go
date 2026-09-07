package portfolioinsight

import (
	"fmt"
	"strings"

	"go-stock/backend/portfoliorisk"
	"go-stock/backend/rebalance"
)

func buildWhyBuyLimited(
	risk *portfoliorisk.PortfolioRiskSnapshot,
	shadow *AllocationShadowView,
	reb *rebalance.RebalanceSuggestion,
	gaps *[]string,
) *WhyBuyLimited {
	factors := []BuyLimitFactor{}

	if risk == nil || !risk.Found {
		*gaps = append(*gaps, "why_buy_limited_partial_no_risk")
	} else {
		if risk.Market.Available && risk.Market.BlockNewEntries {
			factors = append(factors, BuyLimitFactor{
				Code:      "BLOCK_NEW",
				PlainText: "当前市场/风控档位关闭新开仓观察开关",
				Available: true,
			})
		}
		if risk.Exposure.Available {
			if risk.Exposure.HeadroomVsCap != nil && *risk.Exposure.HeadroomVsCap <= 1e-6 {
				factors = append(factors, BuyLimitFactor{
					Code:      "GROSS",
					PlainText: "当前仓位较高：毛敞口已接近或触及上限，新买空间有限",
					Available: true,
				})
			}
			if risk.Exposure.CashRatio != nil && *risk.Exposure.CashRatio < 0.20 {
				factors = append(factors, BuyLimitFactor{
					Code:      "CASH",
					PlainText: "风险预算/现金偏紧：可用现金比例偏低，按组合方式加仓空间受限（观察）",
					Available: true,
				})
			}
		} else {
			factors = append(factors, BuyLimitFactor{
				Code:      "GROSS",
				PlainText: "毛敞口数据暂不可用，无法确认新买空间",
				Available: false,
			})
		}
		if risk.Sector.Available && risk.Sector.MaxSectorWeight != nil {
			maxW := *risk.Sector.MaxSectorWeight
			for _, s := range risk.Sector.SectorExposure {
				if s.Weight > maxW+1e-12 {
					factors = append(factors, BuyLimitFactor{
						Code: "SECTOR",
						PlainText: fmt.Sprintf(
							"单行业暴露过大：行业「%s」权重 %.1f%% 超过观察上限 %.1f%%",
							s.Sector, s.Weight*100, maxW*100,
						),
						Available: true,
					})
					break
				}
			}
		} else if risk != nil && !risk.Sector.Available {
			factors = append(factors, BuyLimitFactor{
				Code:      "SECTOR",
				PlainText: "行业分类暂不可用，未将「行业过热」计为限制买入依据（避免误导）",
				Available: false,
			})
		}
		if risk.Concentration.Available && risk.Concentration.Top1Weight != nil &&
			risk.Concentration.CapSingle != nil && *risk.Concentration.CapSingle > 0 &&
			*risk.Concentration.Top1Weight > *risk.Concentration.CapSingle+1e-12 {
			factors = append(factors, BuyLimitFactor{
				Code:      "SINGLE_CAP",
				PlainText: "单票集中度已超过观察上限，新买/加仓在观察上更谨慎",
				Available: true,
			})
		}
	}

	if shadow == nil || !shadow.Present {
		*gaps = append(*gaps, "shadow_disabled_or_missing")
	} else {
		if shadow.TightenApplied || shadow.SuggestHasPatches {
			factors = append(factors, BuyLimitFactor{
				Code:      "TIGHTEN",
				PlainText: "根据当前账本风险，生成期偏好已收紧（说明过程，非改用户配置）",
				Available: true,
			})
		}
		if shadow.GrossHeadroomBinding {
			factors = append(factors, BuyLimitFactor{
				Code:      "GROSS",
				PlainText: "旁路分配对照显示：毛敞口余量绑定了组合路径可用资金",
				Available: true,
			})
		}
		if shadow.BlockedNewEntries {
			factors = append(factors, BuyLimitFactor{
				Code:      "BLOCK_NEW",
				PlainText: "旁路对照：组合路径出现阻断新开仓绑定",
				Available: true,
			})
		}
		bind := strings.ToLower(strings.TrimSpace(shadow.BudgetBinding))
		if bind == "cash" || bind == "gross" || bind == "blocked" {
			factors = append(factors, BuyLimitFactor{
				Code:      strings.ToUpper(bind),
				PlainText: fmt.Sprintf("组合路径预算绑定为 %s，新买额度受观察约束", bind),
				Available: true,
			})
		}
	}

	if reb != nil {
		if reb.RiskImpact.BlockNewEntries || reb.RiskImpact.NamesBuyBlocked > 0 {
			factors = append(factors, BuyLimitFactor{
				Code:      "BLOCK_NEW",
				PlainText: "再平衡观察：风险收紧导致部分买入被阻断或裁切（suggest_only）",
				Available: true,
			})
		}
		if reb.RiskImpact.NamesBuyClipped > 0 {
			factors = append(factors, BuyLimitFactor{
				Code:      "TIGHTEN",
				PlainText: fmt.Sprintf("再平衡观察：%d 笔买入名义被风险裁切", reb.RiskImpact.NamesBuyClipped),
				Available: true,
			})
		}
	}

	factors = dedupeFactors(factors)
	if len(factors) == 0 {
		// still emit a gentle empty-limited note only when we have some source
		if risk == nil && (shadow == nil || !shadow.Present) && reb == nil {
			return nil
		}
		factors = append(factors, BuyLimitFactor{
			Code:      "NONE",
			PlainText: "当前输入下未识别出明确的新买限制因子（不等于鼓励买入）",
			Available: true,
		})
	}

	out := &WhyBuyLimited{
		Headline:   "新开仓/加仓在观察上受到限制或约束说明",
		Factors:    factors,
		Disclaimer: DisclaimerZH,
	}
	if shadow != nil && shadow.Present {
		out.ShadowContrast = &ShadowContrast{
			LegacyNotionalSum:    shadow.LegacyBuyNotional,
			PortfolioNotionalSum: shadow.PortfolioBuyNotional,
			Binding:              shadow.BudgetBinding,
			Note:                 "旁路对照显示组合定额路径与固定额度路径的差异（仅观察，不切换 Provider）",
		}
	}
	return out
}

func dedupeFactors(in []BuyLimitFactor) []BuyLimitFactor {
	seen := map[string]struct{}{}
	out := make([]BuyLimitFactor, 0, len(in))
	for _, f := range in {
		key := f.Code + "|" + f.PlainText
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, f)
	}
	return out
}
