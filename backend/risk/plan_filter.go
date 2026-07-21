package risk

import (
	"fmt"
	"strings"

	"go-stock/backend/logger"
)

const (
	defaultPlanMaxSingleNamePct = 0.20
	defaultPlanMaxGrossPct      = 0.85
	defaultPlanScanCap          = 30
)

// PlanCheck 计划态检查：仅市场 / 账户现金 / 组合敞口。不调用 PreTradeCheck，不做两融与执行价校验。
func PlanCheck(ctx PlanContext, stockCode string, targetAmount, postGross, postName, remainingCash float64) RiskDecision {
	stockCode = strings.TrimSpace(strings.ToLower(stockCode))
	if stockCode == "" || targetAmount <= 0 {
		return RiskDecision{Allowed: false, Code: ReasonInvalidOrder, Message: "计划候选无效"}
	}

	rc := RiskContext{
		Cash:                remainingCash,
		LongMarketValue:     ctx.LongMarketValue,
		ShortMarketValue:    ctx.ShortMarketValue,
		EquityBase:          planEquity(ctx),
		MarketLevel:         ctx.MarketLevel,
		BlockNewEntries:     ctx.BlockNewEntries,
		MaxExposurePct:      ctx.MaxGrossExposurePct,
		MaxGrossExposurePct: ctx.MaxGrossExposurePct,
		MaxSingleNamePct:    ctx.MaxSingleNamePct,
		MaxDailyLossPct:     ctx.MaxDailyLossPct,
		CurrentDailyPnlPct:  ctx.CurrentDailyPnlPct,
		PostGrossExposure:   postGross,
		PostNameExposure:    postName,
		Order: RiskOrder{
			Kind:      OrderNormalBuy,
			StockCode: stockCode,
		},
	}
	if decision, blocked := checkMarketAndPortfolio(rc); blocked {
		return decision
	}
	if remainingCash+1e-9 < targetAmount {
		return reject(rc, ReasonCashInsufficient,
			fmt.Sprintf("计划可用现金 %.2f 不足目标金额 %.2f", remainingCash, targetAmount), 0)
	}
	return approve(rc, 0)
}

// PlanFilter 按池顺序做计划态过滤；被拒项保留为 skipped。Enable=false 时等同 Phase1 TopN 全 pending。
func PlanFilter(candidates []PlanCandidate, ctx PlanContext) *PlanFilterResult {
	ctx = normalizePlanContext(ctx)
	equity := planEquity(ctx)
	baseGross := ctx.LongMarketValue + ctx.ShortMarketValue

	result := &PlanFilterResult{
		MarketLevel:   ctx.MarketLevel,
		Equity:        equity,
		GrossExposure: baseGross,
	}

	if !ctx.Enabled {
		return planFilterBypass(candidates, ctx, result)
	}

	scanLimit := ctx.ScanLimit
	if scanLimit <= 0 {
		scanLimit = defaultPlanScanCap
	}
	if scanLimit > len(candidates) {
		scanLimit = len(candidates)
	}

	// 整批门禁：市场 / 日亏 — 扫描窗口内全部 skipped
	if batchCode, batchMsg := planBatchBlock(ctx); batchCode != "" {
		for i := 0; i < scanLimit; i++ {
			c := withAmount(candidates[i], ctx.AmountPerStock)
			result.Items = append(result.Items, PlanFilterItem{
				Candidate:   c,
				Allowed:     false,
				Status:      "skipped",
				RiskCode:    batchCode,
				RiskMessage: batchMsg,
				Priority:    i + 1,
			})
		}
		result.FilteredCount = len(result.Items)
		result.RiskStatus = PlanRiskStatusBlocked
		finalizePlanFilterResult(result, ctx, equity, baseGross)
		logPlanFilter(result)
		return result
	}

	remainingCash := ctx.Cash
	committedExtra := 0.0
	accepted := 0
	priority := 0

	for i := 0; i < len(candidates); i++ {
		if accepted >= ctx.MaxNames {
			break
		}
		if i >= scanLimit {
			break
		}
		c := withAmount(candidates[i], ctx.AmountPerStock)
		priority++
		nameBase := 0.0
		if ctx.NameMarketValue != nil {
			nameBase = ctx.NameMarketValue[strings.ToLower(strings.TrimSpace(c.StockCode))]
		}
		postGross := baseGross + committedExtra + c.TargetAmount
		postName := nameBase + c.TargetAmount
		decision := PlanCheck(ctx, c.StockCode, c.TargetAmount, postGross, postName, remainingCash)
		if decision.Allowed {
			accepted++
			remainingCash -= c.TargetAmount
			committedExtra += c.TargetAmount
			result.Items = append(result.Items, PlanFilterItem{
				Candidate:   c,
				Allowed:     true,
				Status:      "pending",
				RiskCode:    ReasonApproved,
				RiskMessage: decision.Message,
				Priority:    priority,
			})
			continue
		}
		result.Items = append(result.Items, PlanFilterItem{
			Candidate:   c,
			Allowed:     false,
			Status:      "skipped",
			RiskCode:    decision.Code,
			RiskMessage: decision.Message,
			Priority:    priority,
		})
	}

	result.AcceptedCount = accepted
	result.FilteredCount = 0
	for _, it := range result.Items {
		if !it.Allowed {
			result.FilteredCount++
		}
	}
	switch {
	case accepted == 0:
		result.RiskStatus = PlanRiskStatusBlocked
	case result.FilteredCount == 0:
		result.RiskStatus = PlanRiskStatusPassed
	default:
		result.RiskStatus = PlanRiskStatusPartial
	}
	result.GrossExposure = baseGross + committedExtra
	finalizePlanFilterResult(result, ctx, equity, result.GrossExposure)
	logPlanFilter(result)
	return result
}

func planFilterBypass(candidates []PlanCandidate, ctx PlanContext, result *PlanFilterResult) *PlanFilterResult {
	n := ctx.MaxNames
	if n <= 0 {
		n = 5
	}
	if n > len(candidates) {
		n = len(candidates)
	}
	for i := 0; i < n; i++ {
		c := withAmount(candidates[i], ctx.AmountPerStock)
		result.Items = append(result.Items, PlanFilterItem{
			Candidate:   c,
			Allowed:     true,
			Status:      "pending",
			RiskCode:    ReasonApproved,
			RiskMessage: "risk filter bypassed",
			Priority:    i + 1,
		})
	}
	result.AcceptedCount = n
	result.FilteredCount = 0
	result.RiskStatus = PlanRiskStatusBypassed
	finalizePlanFilterResult(result, ctx, planEquity(ctx), ctx.LongMarketValue+ctx.ShortMarketValue)
	logPlanFilter(result)
	return result
}

func planBatchBlock(ctx PlanContext) (ReasonCode, string) {
	if ctx.BlockNewEntries && ctx.MarketLevel >= 1 && ctx.MarketLevel <= 2 {
		return ReasonMarketLevelBlocked,
			fmt.Sprintf("当前市场等级 %d 级防守，已禁止新开仓（市场层，与两融杠杆无关）", ctx.MarketLevel)
	}
	if ctx.MaxDailyLossPct > 0 && ctx.CurrentDailyPnlPct < 0 &&
		-ctx.CurrentDailyPnlPct >= ctx.MaxDailyLossPct {
		return ReasonDailyLossHalt,
			fmt.Sprintf("当日亏损已达 %.1f%%，触发模拟日亏熔断", -ctx.CurrentDailyPnlPct*100)
	}
	return "", ""
}

func normalizePlanContext(ctx PlanContext) PlanContext {
	if ctx.MaxNames <= 0 {
		ctx.MaxNames = 5
	}
	if ctx.AmountPerStock <= 0 {
		ctx.AmountPerStock = 100_000
	}
	if ctx.MaxSingleNamePct <= 0 {
		ctx.MaxSingleNamePct = defaultPlanMaxSingleNamePct
	}
	if ctx.MaxGrossExposurePct <= 0 {
		ctx.MaxGrossExposurePct = defaultPlanMaxGrossPct
	}
	if ctx.NameMarketValue == nil {
		ctx.NameMarketValue = map[string]float64{}
	}
	return ctx
}

func planEquity(ctx PlanContext) float64 {
	if ctx.EquityBase > 0 {
		return ctx.EquityBase
	}
	return ctx.Cash + ctx.LongMarketValue
}

func withAmount(c PlanCandidate, amount float64) PlanCandidate {
	if c.TargetAmount <= 0 {
		c.TargetAmount = amount
	}
	return c
}

func finalizePlanFilterResult(result *PlanFilterResult, ctx PlanContext, equity, gross float64) {
	result.Equity = equity
	result.GrossExposure = gross
	result.RiskSummary = fmt.Sprintf("status=%s level=%d accepted=%d rejected=%d equity=%.0f gross=%.0f",
		result.RiskStatus, result.MarketLevel, result.AcceptedCount, result.FilteredCount, equity, gross)
	result.marshalSnapshot(PlanSnapshot{
		Enabled:             ctx.Enabled,
		MarketLevel:         ctx.MarketLevel,
		BlockNewEntries:     ctx.BlockNewEntries,
		Cash:                ctx.Cash,
		Equity:              equity,
		LongMarketValue:     ctx.LongMarketValue,
		GrossExposure:       gross,
		MaxGrossExposurePct: ctx.MaxGrossExposurePct,
		MaxSingleNamePct:    ctx.MaxSingleNamePct,
		MaxDailyLossPct:     ctx.MaxDailyLossPct,
		CurrentDailyPnlPct:  ctx.CurrentDailyPnlPct,
		AmountPerStock:      ctx.AmountPerStock,
		MaxNames:            ctx.MaxNames,
		Accepted:            result.AcceptedCount,
		Rejected:            result.FilteredCount,
	})
}

func logPlanFilter(result *PlanFilterResult) {
	if result == nil {
		return
	}
	logger.SugaredLogger.Infof("risk filter: accepted=%d rejected=%d level=%d equity=%.2f gross=%.2f status=%s",
		result.AcceptedCount, result.FilteredCount,
		result.MarketLevel, result.Equity, result.GrossExposure, result.RiskStatus)
	for _, it := range result.Items {
		if it.Allowed {
			continue
		}
		logger.SugaredLogger.Infof("reject: %s %s", it.Candidate.StockCode, it.RiskCode)
	}
}
