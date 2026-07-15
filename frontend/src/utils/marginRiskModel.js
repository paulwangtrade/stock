import { pickField, toNumber } from './tradingFormat.js'

const firstArray = (source, ...keys) => {
  const value = pickField(source, ...keys)
  return Array.isArray(value) ? value : []
}

const ratioPercent = (value) => {
  const number = toNumber(value)
  return number > 0 && number <= 10 ? number * 100 : number
}

/** 与后端 risk/execution 订单 kind 对齐。 */
export const MARGIN_ORDER_TYPES = Object.freeze([
  { label: '普通买入', value: 'normal_buy' },
  { label: '普通卖出', value: 'normal_sell' },
  { label: '融资买入', value: 'margin_buy' },
  { label: '卖券还款', value: 'sell_repay' },
  { label: '融券卖出', value: 'short_sell' },
  { label: '买券还券', value: 'buy_return' },
])

/** 兼容旧前端枚举 → 后端 kind。 */
export function toBackendOrderKind(value) {
  const kind = String(value || '').toLowerCase()
  const map = {
    buy: 'normal_buy',
    sell: 'normal_sell',
    sell_to_repay: 'sell_repay',
    buy_to_return: 'buy_return',
  }
  return map[kind] || kind
}

export function normalizePositionType(value, volume = 0) {
  const type = String(value || '').trim().toLowerCase()
  if (['margin_long', 'financing_long', 'margin_buy', 'credit_long', '融资多头', '融资'].includes(type)) return 'margin_long'
  if (['short', 'short_sell', 'securities_short', 'credit_short', '融券空头', '融券'].includes(type)) return 'short'
  if (toNumber(volume) < 0) return 'short'
  return 'cash_long'
}

export function normalizeMarginPosition(item = {}) {
  const rawVolume = toNumber(pickField(item, 'volume', 'Volume', 'quantity', 'Quantity'))
  const positionType = normalizePositionType(
    pickField(item, 'positionType', 'PositionType', 'positionKind', 'PositionKind', 'marginType', 'MarginType'),
    rawVolume,
  )
  return {
    ...item,
    id: pickField(item, 'id', 'ID'),
    stockCode: String(pickField(item, 'stockCode', 'StockCode', 'code', 'Code') || ''),
    stockName: String(pickField(item, 'stockName', 'StockName', 'name', 'Name') || ''),
    positionType,
    volume: Math.abs(rawVolume),
    sellable: toNumber(pickField(item, 'sellable', 'Sellable', 'availableVolume', 'AvailableVolume')),
    avgCost: toNumber(pickField(item, 'avgCost', 'AvgCost', 'costPrice', 'CostPrice')),
    marketPrice: toNumber(pickField(item, 'marketPrice', 'MarketPrice', 'lastPrice', 'LastPrice', 'price', 'Price')),
    liability: toNumber(pickField(item, 'liability', 'Liability', 'debt', 'Debt', 'shortLiability', 'ShortLiability')),
  }
}

export function normalizeRiskEvent(item = {}, index = 0) {
  const current = pickField(item, 'currentValue', 'CurrentValue', 'current', 'Current', 'value', 'Value')
  const threshold = pickField(item, 'threshold', 'Threshold', 'limit', 'Limit')
  return {
    ...item,
    id: pickField(item, 'id', 'ID', 'eventId', 'EventID') ?? index,
    occurredAt: pickField(item, 'occurredAt', 'OccurredAt', 'createdAt', 'CreatedAt', 'timestamp', 'Timestamp'),
    level: String(pickField(item, 'level', 'Level', 'severity', 'Severity', 'riskLevel', 'RiskLevel') || 'info').toLowerCase(),
    reasonCode: String(pickField(item, 'reasonCode', 'ReasonCode', 'code', 'Code') || 'MARGIN_RISK_UNKNOWN')
      .trim().toUpperCase().replace(/[\s-]+/g, '_'),
    message: String(pickField(item, 'message', 'Message', 'reason', 'Reason') || ''),
    currentValue: current,
    threshold,
    unit: String(pickField(item, 'unit', 'Unit') || ''),
  }
}

export function createMarginFallback(snapshot = null, error = null) {
  const base = snapshot || {}
  const account = pickField(base, 'account', 'Account') || {}
  const positions = firstArray(base, 'positions', 'Positions').map(normalizeMarginPosition)
  const longExposure = positions.reduce((sum, item) => {
    const value = (item.marketPrice || item.avgCost) * item.volume
    return item.positionType === 'short' ? sum : sum + value
  }, 0)
  return {
    ...base,
    account,
    positions,
    orders: firstArray(base, 'orders', 'Orders'),
    fills: firstArray(base, 'fills', 'Fills'),
    equity: firstArray(base, 'equity', 'Equity'),
    riskEvents: [],
    margin: {
      accountMode: 'cash',
      financingBalance: 0,
      securitiesLiability: 0,
      availableMargin: toNumber(pickField(account, 'cash', 'Cash')),
      maintenanceRatio: 0,
      netExposure: longExposure,
      grossExposure: longExposure,
    },
    marginApiAvailable: false,
    marginFallbackReason: error ? (error.message || String(error)) : '两融绑定未生成',
  }
}

export function normalizeMarginSnapshot(raw, fallbackSnapshot = null) {
  if (!raw || typeof raw !== 'object') return createMarginFallback(fallbackSnapshot)
  const base = fallbackSnapshot || {}
  const account = pickField(raw, 'account', 'Account') || pickField(base, 'account', 'Account') || {}
  const marginAccount = pickField(raw, 'marginAccount', 'MarginAccount') || {}
  const metrics = pickField(raw, 'metrics', 'Metrics', 'margin', 'Margin', 'risk', 'Risk') || raw
  const positionsRaw = firstArray(raw, 'positions', 'Positions')
  const positions = (positionsRaw.length ? positionsRaw : firstArray(base, 'positions', 'Positions'))
    .map(normalizeMarginPosition)
  const financeDebts = firstArray(raw, 'financeLiabilities', 'FinanceLiabilities')
  const securityDebts = firstArray(raw, 'securitiesLiabilities', 'SecuritiesLiabilities')
  const financingBalance = toNumber(pickField(
    metrics, 'financingBalance', 'FinancingBalance', 'marginDebt', 'MarginDebt', 'financePrincipal', 'FinancePrincipal',
  ), financeDebts.reduce((sum, item) => sum
    + toNumber(pickField(item, 'principal', 'Principal'))
    + toNumber(pickField(item, 'accruedInterest', 'AccruedInterest')), 0))
  const securitiesLiability = toNumber(pickField(
    metrics, 'securitiesLiability', 'SecuritiesLiability', 'shortLiability', 'ShortLiability',
  ), securityDebts.reduce((sum, item) => sum
    + toNumber(pickField(item, 'avgPrice', 'AvgPrice')) * toNumber(pickField(item, 'quantity', 'Quantity'))
    + toNumber(pickField(item, 'accruedFee', 'AccruedFee')), 0))
  const longExposure = positions.reduce((sum, item) => item.positionType === 'short'
    ? sum
    : sum + (item.marketPrice || item.avgCost) * item.volume, 0)
  const shortExposure = positions.reduce((sum, item) => item.positionType === 'short'
    ? sum + (item.marketPrice || item.avgCost) * item.volume
    : sum, 0)
  const riskEventsRaw = firstArray(raw, 'riskEvents', 'RiskEvents', 'events', 'Events')
  const riskEvents = (riskEventsRaw.length
    ? riskEventsRaw
    : firstArray(metrics, 'riskEvents', 'RiskEvents', 'events', 'Events'))
    .map(normalizeRiskEvent)

  return {
    ...base,
    ...raw,
    account,
    positions,
    orders: firstArray(raw, 'orders', 'Orders').length
      ? firstArray(raw, 'orders', 'Orders')
      : firstArray(base, 'orders', 'Orders'),
    fills: firstArray(raw, 'fills', 'Fills').length
      ? firstArray(raw, 'fills', 'Fills')
      : firstArray(base, 'fills', 'Fills'),
    equity: firstArray(raw, 'equity', 'Equity').length
      ? firstArray(raw, 'equity', 'Equity')
      : firstArray(base, 'equity', 'Equity'),
    riskEvents,
    margin: {
      accountMode: String(pickField(
        marginAccount, 'mode', 'Mode', 'accountMode', 'AccountMode',
      ) || pickField(metrics, 'accountMode', 'AccountMode', 'mode', 'Mode')
        || pickField(account, 'accountMode', 'AccountMode') || 'margin').toLowerCase(),
      financingBalance,
      securitiesLiability,
      availableMargin: toNumber(pickField(metrics, 'availableMargin', 'AvailableMargin', 'marginAvailable', 'MarginAvailable')),
      maintenanceRatio: ratioPercent(pickField(
        metrics, 'maintenanceRatio', 'MaintenanceRatio', 'maintenanceGuaranteeRatio', 'MaintenanceGuaranteeRatio',
      )),
      netExposure: toNumber(pickField(metrics, 'netExposure', 'NetExposure'), longExposure - shortExposure),
      grossExposure: toNumber(
        pickField(metrics, 'grossExposure', 'GrossExposure', 'totalExposure', 'TotalExposure'),
        longExposure + shortExposure,
      ),
    },
    marginApiAvailable: true,
    marginFallbackReason: '',
  }
}

export function isMarginOrderType(value) {
  const kind = toBackendOrderKind(value)
  return ['margin_buy', 'sell_repay', 'short_sell', 'buy_return'].includes(kind)
}

export function marginOrderSide(value) {
  const kind = toBackendOrderKind(value)
  return ['normal_sell', 'sell', 'sell_repay', 'short_sell'].includes(kind) ? 'sell' : 'buy'
}
