package portfolioinsight

import (
	"fmt"
	"sort"
	"strings"

	"go-stock/backend/holdingdecision/rules"
	"go-stock/backend/portfoliorisk"
)

func buildSummary(risk *portfoliorisk.PortfolioRiskSnapshot, holdings []rules.HoldingDecision, gaps *[]string) PortfolioSummary {
	sum := PortfolioSummary{
		RiskLevel:        RiskUnavailable,
		RiskLevelLabel:   riskLevelLabelZH[RiskUnavailable],
		RiskLevelReasons: []string{},
		NarrativeHints:   []string{"以下为系统观察说明，不是买卖指令。"},
	}

	if risk == nil || !risk.Found {
		*gaps = append(*gaps, "risk_snapshot_missing")
		sum.NameCountNote = "暂无持仓数据"
		sum.MaxPositionNote = "集中度暂不可用"
		sum.CashRatioNote = "现金比例暂不可用"
		sum.SectorUnavailableNote = "行业分类暂不可用，未评估集中度"
		sum.RiskLevelReasons = []string{"缺少可用的组合风险快照，风险等级暂不可评"}
		sum.NarrativeHints = append(sum.NarrativeHints, "组合风险快照缺失或未找到账户账本，摘要字段多为暂不可用。")
		return sum
	}

	// name count
	if risk.Concentration.Available && risk.Concentration.NameCount > 0 {
		n := risk.Concentration.NameCount
		sum.NameCount = &n
	} else if risk.Concentration.Available {
		n := 0
		sum.NameCount = &n
		sum.NameCountNote = "持仓名为 0"
	} else {
		sum.NameCountNote = "暂无持仓数据"
		*gaps = append(*gaps, "concentration_unavailable")
	}

	// max position
	if risk.Concentration.Available && risk.Concentration.Top1Weight != nil {
		sum.MaxPosition = &MaxPosition{Weight: *risk.Concentration.Top1Weight}
		if risk.Concentration.CapSingle != nil {
			sum.MaxPosition.Note = fmt.Sprintf("对照单票上限观察值 %.1f%%", *risk.Concentration.CapSingle*100)
		}
	} else {
		sum.MaxPositionNote = "集中度暂不可用"
		if !risk.Concentration.Available {
			*gaps = append(*gaps, "concentration_unavailable")
		}
	}

	// sector — never fake 0% when unavailable
	if risk.Sector.Available {
		if conc := topSector(risk.Sector); conc != nil {
			sum.SectorConcentration = conc
		} else {
			sum.SectorUnavailableNote = "行业数据可用但暂无行业暴露条目"
		}
	} else {
		sum.SectorUnavailableNote = "行业分类暂不可用，未评估集中度"
		*gaps = append(*gaps, "sector_unavailable")
		note := strings.TrimSpace(risk.Sector.Note)
		if note != "" {
			sum.SectorUnavailableNote = sum.SectorUnavailableNote + "（" + note + "）"
		}
	}

	// cash ratio
	if risk.Exposure.Available && risk.Exposure.CashRatio != nil {
		cr := *risk.Exposure.CashRatio
		sum.CashRatio = &cr
	} else {
		sum.CashRatioNote = "现金比例暂不可用"
		if !risk.Exposure.Available {
			*gaps = append(*gaps, "exposure_unavailable")
		}
	}

	level, reasons := synthesizeRiskLevel(risk, holdings)
	sum.RiskLevel = level
	sum.RiskLevelLabel = riskLevelLabelZH[level]
	sum.RiskLevelReasons = reasons
	sum.NarrativeHints = append(sum.NarrativeHints, narrativeFromLevel(level, risk)...)
	return sum
}

func topSector(block portfoliorisk.SectorBlock) *SectorConcentration {
	if len(block.SectorExposure) == 0 {
		return nil
	}
	best := block.SectorExposure[0]
	for _, s := range block.SectorExposure[1:] {
		if s.Weight > best.Weight {
			best = s
		}
	}
	out := &SectorConcentration{
		SectorName: best.Sector,
		Weight:     best.Weight,
		Cap:        block.MaxSectorWeight,
	}
	if block.MaxSectorWeight != nil && best.Weight > *block.MaxSectorWeight+1e-12 {
		out.OverCap = true
		out.Note = fmt.Sprintf("已对照上限 %.1f%% · 超限观察", *block.MaxSectorWeight*100)
	} else if block.MaxSectorWeight != nil {
		out.Note = fmt.Sprintf("已对照上限 %.1f%%", *block.MaxSectorWeight*100)
	}
	return out
}

func synthesizeRiskLevel(risk *portfoliorisk.PortfolioRiskSnapshot, holdings []rules.HoldingDecision) (string, []string) {
	if risk == nil || !risk.Found {
		return RiskUnavailable, []string{"风险快照不可用"}
	}
	// Need at least one primary block to score; else unavailable.
	usable := risk.Exposure.Available || risk.Concentration.Available || risk.Sector.Available
	if !usable {
		return RiskUnavailable, []string{"敞口/集中度/行业块均不可用，风险等级暂不可评"}
	}

	score := 0
	reasons := []string{}

	if risk.Concentration.Available && risk.Concentration.Top1Weight != nil && risk.Concentration.CapSingle != nil &&
		*risk.Concentration.CapSingle > 0 {
		top := *risk.Concentration.Top1Weight
		cap := *risk.Concentration.CapSingle
		if top > cap+1e-12 {
			score += 2
			reasons = append(reasons, fmt.Sprintf("最大持仓权重 %.1f%% 超过单票上限观察值 %.1f%%", top*100, cap*100))
		} else if top >= cap*0.85 {
			score += 1
			reasons = append(reasons, fmt.Sprintf("最大持仓权重 %.1f%% 接近单票上限观察值 %.1f%%", top*100, cap*100))
		}
	}

	if risk.Sector.Available && risk.Sector.MaxSectorWeight != nil {
		maxW := *risk.Sector.MaxSectorWeight
		for _, s := range risk.Sector.SectorExposure {
			if s.Weight > maxW+1e-12 {
				score += 2
				reasons = append(reasons, fmt.Sprintf("行业「%s」权重 %.1f%% 超过行业上限观察值 %.1f%%", s.Sector, s.Weight*100, maxW*100))
				break
			}
		}
	}

	if risk.Exposure.Available && risk.Exposure.HeadroomVsCap != nil {
		h := *risk.Exposure.HeadroomVsCap
		if h <= 1e-6 {
			score += 2
			reasons = append(reasons, "毛敞口余量已耗尽或触及上限，新买空间有限")
		} else if h < 0.05 {
			score += 1
			reasons = append(reasons, fmt.Sprintf("毛敞口余量偏低（约 %.1f%%）", h*100))
		}
	}

	riskReduce := 0
	for _, d := range holdings {
		act := strings.ToUpper(strings.TrimSpace(d.FinalAction))
		if act == "" {
			act = strings.ToUpper(strings.TrimSpace(d.Action))
		}
		if act != rules.ActionReduce && act != rules.ActionExit {
			continue
		}
		for _, c := range d.ReasonCodes {
			if strings.HasPrefix(c, "RISK_") || c == rules.ReasonPortfolioTighten {
				riskReduce++
			}
		}
	}
	if riskReduce >= 2 {
		score += 1
		reasons = append(reasons, fmt.Sprintf("多笔持仓因风险类规则进入减持/退出观察（%d）", riskReduce))
	}

	sort.Strings(reasons)
	level := RiskLow
	switch {
	case score >= 5:
		level = RiskHigh
	case score >= 3:
		level = RiskElevated
	case score >= 1:
		level = RiskModerate
	default:
		level = RiskLow
		if len(reasons) == 0 {
			reasons = []string{"当前可用风险块未触发抬升条件（解释用，非下单闸）"}
		}
	}
	return level, reasons
}

func narrativeFromLevel(level string, risk *portfoliorisk.PortfolioRiskSnapshot) []string {
	out := []string{}
	switch level {
	case RiskElevated, RiskHigh:
		out = append(out, "可关注单票权重与毛敞口余量是否超过说明中的观察上限（非下单指令）。")
	case RiskModerate:
		out = append(out, "组合存在需留意的集中或敞口信号，详见风险等级说明。")
	case RiskUnavailable:
		out = append(out, "风险等级暂不可评；请勿将缺失数据理解为「很安全」。")
	}
	if risk != nil && !risk.Sector.Available {
		out = append(out, "行业分类数据不足，集中度说明暂缺。")
	}
	return out
}
