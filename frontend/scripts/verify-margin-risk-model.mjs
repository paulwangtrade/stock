import assert from 'node:assert/strict'
import {
  MARGIN_ORDER_TYPES,
  createMarginFallback,
  marginOrderSide,
  normalizeMarginSnapshot,
  normalizeRiskEvent,
} from '../src/utils/marginRiskModel.js'

assert.deepEqual(
  MARGIN_ORDER_TYPES.map(item => item.value),
  ['normal_buy', 'normal_sell', 'margin_buy', 'sell_repay', 'short_sell', 'buy_return'],
)
assert.equal(marginOrderSide('margin_buy'), 'buy')
assert.equal(marginOrderSide('sell_to_repay'), 'sell')
assert.equal(marginOrderSide('sell_repay'), 'sell')
assert.equal(marginOrderSide('short_sell'), 'sell')
assert.equal(marginOrderSide('buy_to_return'), 'buy')
assert.equal(marginOrderSide('buy_return'), 'buy')
assert.equal(marginOrderSide('normal_buy'), 'buy')

const fallback = createMarginFallback({
  account: { cash: 800_000, equity: 1_000_000 },
  positions: [{ stockCode: '600000.SH', volume: 10_000, avgCost: 10 }],
  orders: [{ id: 1, side: 'buy' }],
})
assert.equal(fallback.marginApiAvailable, false)
assert.equal(fallback.margin.accountMode, 'cash')
assert.equal(fallback.margin.availableMargin, 800_000)
assert.equal(fallback.positions[0].positionType, 'cash_long')
assert.equal(fallback.orders.length, 1)

const margin = normalizeMarginSnapshot({
  Account: { AccountMode: 'margin', Cash: 200_000 },
  Margin: {
    FinancingBalance: 300_000,
    SecuritiesLiability: 80_000,
    AvailableMargin: 150_000,
    MaintenanceGuaranteeRatio: 2.4,
  },
  Positions: [
    { StockCode: '600000.SH', PositionType: 'margin_long', Volume: 10_000, MarketPrice: 12 },
    { StockCode: '000001.SZ', PositionType: 'short_sell', Volume: 5_000, MarketPrice: 10 },
  ],
  RiskEvents: [{
    ReasonCode: 'maintenance-ratio-low',
    CurrentValue: 240,
    Threshold: 260,
    Unit: '%',
  }],
})
assert.equal(margin.marginApiAvailable, true)
assert.equal(margin.margin.maintenanceRatio, 240)
assert.equal(margin.margin.netExposure, 70_000)
assert.equal(margin.margin.grossExposure, 170_000)
assert.deepEqual(margin.positions.map(item => item.positionType), ['margin_long', 'short'])
assert.equal(margin.riskEvents[0].reasonCode, 'MAINTENANCE_RATIO_LOW')
assert.equal(margin.riskEvents[0].currentValue, 240)
assert.equal(margin.riskEvents[0].threshold, 260)

const unknown = normalizeRiskEvent({})
assert.equal(unknown.reasonCode, 'MARGIN_RISK_UNKNOWN')

// 对齐后端 execution.Snapshot：Metrics + FinanceLiabilities（维保为比值 1.5 → 展示 150）
const backendShape = normalizeMarginSnapshot({
  account: { cash: 100_000, equity: 500_000 },
  marginAccount: { mode: 'margin' },
  metrics: {
    totalAssets: 500_000,
    totalLiabilities: 200_000,
    netExposure: 300_000,
    grossExposure: 500_000,
    maintenanceRatio: 1.5,
    marginAvailable: 120_000,
  },
  financeLiabilities: [
    { stockCode: '600000.SH', principal: 180_000, accruedInterest: 1_200 },
  ],
  securitiesLiabilities: [
    { stockCode: '000001.SZ', avgPrice: 10, quantity: 2_000, accruedFee: 50 },
  ],
  positions: [
    { stockCode: '600000.SH', positionType: 'margin_long', volume: 10_000, marketPrice: 12 },
  ],
  riskEvents: [],
})
assert.equal(backendShape.margin.maintenanceRatio, 150)
assert.equal(backendShape.margin.availableMargin, 120_000)
assert.equal(backendShape.margin.financingBalance, 181_200)
assert.equal(backendShape.margin.securitiesLiability, 20_050)
assert.equal(backendShape.margin.netExposure, 300_000)
assert.equal(backendShape.margin.grossExposure, 500_000)

console.log('两融快照回退、暴露、持仓与风险原因码校验通过')
