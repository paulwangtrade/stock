package risk

import "fmt"

// CalculateMetrics 使用模拟口径：总资产 / 总负债为维持担保比例；无负债时返回 0（表示不适用，确保 JSON 可序列化）。
// 阈值由账户配置传入，本包不声称任何真实监管或券商阈值。
func CalculateMetrics(ctx RiskContext) Metrics {
	liabilities := ctx.FinancePrincipal + ctx.FinanceInterest + ctx.ShortMarketValue + ctx.SecuritiesFee
	assets := ctx.Cash + ctx.LongMarketValue
	ratio := 0.0
	if liabilities > 0 {
		ratio = assets / liabilities
	}
	equity := ctx.EquityBase
	if equity <= 0 {
		equity = assets
	}
	grossPct := 0.0
	namePct := 0.0
	if equity > 0 {
		if ctx.PostGrossExposure > 0 {
			grossPct = ctx.PostGrossExposure / equity
		} else {
			grossPct = (ctx.LongMarketValue + ctx.ShortMarketValue) / equity
		}
		if ctx.PostNameExposure > 0 {
			namePct = ctx.PostNameExposure / equity
		} else if ctx.CurrentNameValue > 0 {
			namePct = ctx.CurrentNameValue / equity
		}
	}
	return Metrics{
		TotalAssets:      assets,
		TotalLiabilities: liabilities,
		NetExposure:      ctx.LongMarketValue - ctx.ShortMarketValue,
		GrossExposure:    ctx.LongMarketValue + ctx.ShortMarketValue,
		MaintenanceRatio: ratio,
		MarginAvailable:  ctx.MarginAvailable,
		GrossExposurePct: grossPct,
		SingleNamePct:    namePct,
	}
}

func reject(ctx RiskContext, code ReasonCode, message string, required float64) RiskDecision {
	metrics := CalculateMetrics(ctx)
	metrics.RequiredOrderBond = required
	return RiskDecision{Allowed: false, Code: code, Message: message, Metrics: metrics}
}

func approve(ctx RiskContext, required float64) RiskDecision {
	metrics := CalculateMetrics(ctx)
	metrics.RequiredOrderBond = required
	code := ReasonApproved
	message := "通过模拟风控检查"
	if metrics.TotalLiabilities > 0 && ctx.WarningRatio > 0 && metrics.MaintenanceRatio < ctx.WarningRatio {
		code = ReasonWarningTriggered
		message = "通过，但账户已低于模拟警戒线"
	}
	return RiskDecision{Allowed: true, Code: code, Message: message, Metrics: metrics}
}

func isRiskIncreasing(kind string) bool {
	return kind == OrderMarginBuy || kind == OrderShortSell
}

func isNewEntry(kind string) bool {
	return kind == OrderNormalBuy || kind == OrderMarginBuy || kind == OrderShortSell
}

func effectiveMaxGrossPct(ctx RiskContext) float64 {
	if ctx.MaxGrossExposurePct > 0 {
		return ctx.MaxGrossExposurePct
	}
	if ctx.MaxExposurePct > 0 {
		return ctx.MaxExposurePct
	}
	return 0
}

func checkMarketAndPortfolio(ctx RiskContext) (RiskDecision, bool) {
	if isNewEntry(ctx.Order.Kind) && ctx.BlockNewEntries && ctx.MarketLevel >= 1 && ctx.MarketLevel <= 2 {
		return reject(ctx, ReasonMarketLevelBlocked,
			fmt.Sprintf("当前市场等级 %d 级防守，已禁止新开仓（市场层，与两融杠杆无关）", ctx.MarketLevel), 0), true
	}
	if isNewEntry(ctx.Order.Kind) && ctx.MaxDailyLossPct > 0 && ctx.CurrentDailyPnlPct < 0 &&
		-ctx.CurrentDailyPnlPct >= ctx.MaxDailyLossPct {
		return reject(ctx, ReasonDailyLossHalt,
			fmt.Sprintf("当日亏损已达 %.1f%%，触发模拟日亏熔断", -ctx.CurrentDailyPnlPct*100), 0), true
	}
	maxGross := effectiveMaxGrossPct(ctx)
	metrics := CalculateMetrics(ctx)
	if isNewEntry(ctx.Order.Kind) && maxGross > 0 && metrics.GrossExposurePct > maxGross {
		return reject(ctx, ReasonGrossExposureExceeded,
			fmt.Sprintf("下单后总敞口 %.1f%% 超过上限 %.1f%%", metrics.GrossExposurePct*100, maxGross*100), 0), true
	}
	maxName := ctx.MaxSingleNamePct
	if maxName <= 0 {
		maxName = 0.20
	}
	if isNewEntry(ctx.Order.Kind) && metrics.SingleNamePct > maxName {
		return reject(ctx, ReasonSingleNameExceeded,
			fmt.Sprintf("下单后单票敞口 %.1f%% 超过上限 %.1f%%", metrics.SingleNamePct*100, maxName*100), 0), true
	}
	return RiskDecision{}, false
}

// PreTradeCheck 对所有模拟下单入口执行统一检查（市场 → 组合 → 两融账户 → 订单）。
func PreTradeCheck(ctx RiskContext) RiskDecision {
	order := ctx.Order
	if order.StockCode == "" || order.Price <= 0 || order.Volume <= 0 {
		return reject(ctx, ReasonInvalidOrder, "证券、价格或数量无效", 0)
	}
	if decision, blocked := checkMarketAndPortfolio(ctx); blocked {
		return decision
	}

	amount := order.Price * float64(order.Volume)
	metrics := CalculateMetrics(ctx)
	// 无负债时维持担保比例不适用（为 0），不能按平仓线拒单；仅在已有负债时生效。
	if isRiskIncreasing(order.Kind) && metrics.TotalLiabilities > 0 && ctx.CloseoutRatio > 0 && metrics.MaintenanceRatio < ctx.CloseoutRatio {
		return reject(ctx, ReasonCloseoutTriggered, "账户低于模拟平仓线，仅允许降低风险的交易", 0)
	}

	switch order.Kind {
	case OrderNormalBuy:
		if ctx.Cash < amount {
			return reject(ctx, ReasonCashInsufficient, "可用现金不足", 0)
		}
	case OrderNormalSell:
		if ctx.PositionSellable < order.Volume {
			return reject(ctx, ReasonPositionInsufficient, "可卖持仓不足", 0)
		}
	case OrderMarginBuy:
		if ctx.AccountMode != "margin" {
			return reject(ctx, ReasonMarginModeRequired, "账户未启用模拟两融模式", 0)
		}
		if ctx.FinanceCreditAvailable < amount {
			return reject(ctx, ReasonFinanceCreditExceeded, "模拟融资授信不足", 0)
		}
		required := amount * ctx.FinanceMarginRatio
		if ctx.MarginAvailable < required {
			return reject(ctx, ReasonMarginInsufficient, "保证金可用不足", required)
		}
		return approve(ctx, required)
	case OrderSellRepay:
		if ctx.PositionSellable < order.Volume {
			return reject(ctx, ReasonPositionInsufficient, "卖券可卖持仓不足", 0)
		}
		if ctx.FinanceDebt <= 0 {
			return reject(ctx, ReasonDebtNotFound, "不存在可偿还的融资负债", 0)
		}
	case OrderShortSell:
		if ctx.AccountMode != "margin" {
			return reject(ctx, ReasonMarginModeRequired, "账户未启用模拟两融模式", 0)
		}
		if ctx.BorrowAvailable < order.Volume {
			return reject(ctx, ReasonBorrowUnavailable, "模拟券源不足", 0)
		}
		if ctx.SecuritiesCreditAvailable < amount {
			return reject(ctx, ReasonSecurityCreditExceeded, "模拟融券授信不足", 0)
		}
		required := amount * ctx.SecuritiesMarginRatio
		if ctx.MarginAvailable < required {
			return reject(ctx, ReasonMarginInsufficient, "保证金可用不足", required)
		}
		return approve(ctx, required)
	case OrderBuyReturn:
		if ctx.SecuritiesDebtQuantity < order.Volume {
			return reject(ctx, ReasonDebtNotFound, "融券负债数量不足", 0)
		}
		if ctx.Cash < amount {
			return reject(ctx, ReasonCashInsufficient, "买券还券现金不足", 0)
		}
	default:
		return reject(ctx, ReasonInvalidOrder, "不支持的模拟订单类型", 0)
	}
	return approve(ctx, 0)
}
