package risk

import (
	"math"
	"testing"
)

func TestCalculateMetrics(t *testing.T) {
	metrics := CalculateMetrics(RiskContext{
		Cash:             100,
		LongMarketValue:  900,
		ShortMarketValue: 200,
		FinancePrincipal: 400,
		FinanceInterest:  10,
		SecuritiesFee:    5,
		MarginAvailable:  123,
	})
	if metrics.TotalAssets != 1000 {
		t.Fatalf("total assets = %v, want 1000", metrics.TotalAssets)
	}
	if metrics.TotalLiabilities != 615 {
		t.Fatalf("total liabilities = %v, want 615", metrics.TotalLiabilities)
	}
	if math.Abs(metrics.MaintenanceRatio-1000.0/615.0) > 1e-9 {
		t.Fatalf("maintenance ratio = %v", metrics.MaintenanceRatio)
	}
	if metrics.NetExposure != 700 || metrics.GrossExposure != 1100 {
		t.Fatalf("unexpected exposure: net=%v gross=%v", metrics.NetExposure, metrics.GrossExposure)
	}
}

func TestPreTradeRejectRules(t *testing.T) {
	tests := []struct {
		name string
		ctx  RiskContext
		code ReasonCode
	}{
		{
			name: "normal buy cash insufficient",
			ctx: RiskContext{Cash: 99, Order: RiskOrder{
				Kind: OrderNormalBuy, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonCashInsufficient,
		},
		{
			name: "margin mode required",
			ctx: RiskContext{AccountMode: "cash", FinanceCreditAvailable: 1000, MarginAvailable: 1000, Order: RiskOrder{
				Kind: OrderMarginBuy, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonMarginModeRequired,
		},
		{
			name: "finance credit exceeded",
			ctx: RiskContext{AccountMode: "margin", FinanceCreditAvailable: 99, MarginAvailable: 1000, Order: RiskOrder{
				Kind: OrderMarginBuy, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonFinanceCreditExceeded,
		},
		{
			name: "margin insufficient",
			ctx: RiskContext{AccountMode: "margin", FinanceCreditAvailable: 1000, MarginAvailable: 49, FinanceMarginRatio: 0.5, Order: RiskOrder{
				Kind: OrderMarginBuy, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonMarginInsufficient,
		},
		{
			name: "borrow unavailable",
			ctx: RiskContext{AccountMode: "margin", BorrowAvailable: 9, SecuritiesCreditAvailable: 1000, MarginAvailable: 1000, Order: RiskOrder{
				Kind: OrderShortSell, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonBorrowUnavailable,
		},
		{
			name: "closeout blocks risk increase",
			ctx: RiskContext{
				AccountMode: "margin", Cash: 100, FinancePrincipal: 100, CloseoutRatio: 1.3,
				FinanceCreditAvailable: 1000, MarginAvailable: 1000,
				Order: RiskOrder{Kind: OrderMarginBuy, StockCode: "SZ000001", Price: 10, Volume: 10},
			},
			code: ReasonCloseoutTriggered,
		},
		{
			name: "buy return debt insufficient",
			ctx: RiskContext{Cash: 1000, SecuritiesDebtQuantity: 9, Order: RiskOrder{
				Kind: OrderBuyReturn, StockCode: "SZ000001", Price: 10, Volume: 10,
			}},
			code: ReasonDebtNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := PreTradeCheck(test.ctx)
			if decision.Allowed || decision.Code != test.code {
				t.Fatalf("decision = %+v, want rejection %s", decision, test.code)
			}
		})
	}
}

func TestPreTradeAllowsDeleveragingBelowCloseout(t *testing.T) {
	decision := PreTradeCheck(RiskContext{
		Cash:             100,
		FinancePrincipal: 100,
		CloseoutRatio:    1.3,
		PositionSellable: 100,
		FinanceDebt:      100,
		Order: RiskOrder{
			Kind: OrderSellRepay, StockCode: "SZ000001", Price: 10, Volume: 100,
		},
	})
	if !decision.Allowed {
		t.Fatalf("deleveraging order rejected: %+v", decision)
	}
}

func TestPreTradeMarketAndPortfolioLayers(t *testing.T) {
	tests := []struct {
		name string
		ctx  RiskContext
		code ReasonCode
	}{
		{
			name: "market level blocks new entries",
			ctx: RiskContext{
				MarketLevel: 2, BlockNewEntries: true,
				Cash: 10000, EquityBase: 10000,
				Order: RiskOrder{Kind: OrderNormalBuy, StockCode: "SZ000001", Price: 10, Volume: 10},
			},
			code: ReasonMarketLevelBlocked,
		},
		{
			name: "gross exposure exceeded",
			ctx: RiskContext{
				Cash: 1000, LongMarketValue: 800, EquityBase: 1000,
				MaxGrossExposurePct: 0.90,
				PostGrossExposure:   1000,
				Order:               RiskOrder{Kind: OrderNormalBuy, StockCode: "SZ000001", Price: 10, Volume: 20},
			},
			code: ReasonGrossExposureExceeded,
		},
		{
			name: "single name exceeded",
			ctx: RiskContext{
				Cash: 10000, EquityBase: 10000,
				MaxSingleNamePct: 0.15,
				PostNameExposure: 2000,
				Order:            RiskOrder{Kind: OrderNormalBuy, StockCode: "SZ000001", Price: 10, Volume: 10},
			},
			code: ReasonSingleNameExceeded,
		},
		{
			name: "daily loss halt",
			ctx: RiskContext{
				Cash: 10000, EquityBase: 10000,
				MaxDailyLossPct: 0.03, CurrentDailyPnlPct: -0.035,
				Order: RiskOrder{Kind: OrderNormalBuy, StockCode: "SZ000001", Price: 10, Volume: 10},
			},
			code: ReasonDailyLossHalt,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := PreTradeCheck(test.ctx)
			if decision.Allowed || decision.Code != test.code {
				t.Fatalf("decision = %+v, want rejection %s", decision, test.code)
			}
		})
	}

	// 卖出不受市场防守禁开仓影响
	ok := PreTradeCheck(RiskContext{
		MarketLevel: 1, BlockNewEntries: true, PositionSellable: 100,
		Order: RiskOrder{Kind: OrderNormalSell, StockCode: "SZ000001", Price: 10, Volume: 10},
	})
	if !ok.Allowed {
		t.Fatalf("sell should pass market block: %+v", ok)
	}
}
