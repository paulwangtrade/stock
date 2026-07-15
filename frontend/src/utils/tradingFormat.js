const hasOwn = (value, key) =>
  value != null && Object.prototype.hasOwnProperty.call(Object(value), key)

export function pickField(source, ...keys) {
  for (const key of keys.flat()) {
    if (hasOwn(source, key) && source[key] !== null && source[key] !== undefined) {
      return source[key]
    }
  }
  return undefined
}

export function toNumber(value, fallback = 0) {
  const parsed = typeof value === 'string'
    ? Number(value.replace(/,/g, '').replace(/%$/, '').trim())
    : Number(value)
  return Number.isFinite(parsed) ? parsed : fallback
}

export function formatMoney(value, digits = 2) {
  return toNumber(value).toLocaleString('zh-CN', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

export function formatPrice(value) {
  return formatMoney(value, 2)
}

export function formatVolume(value) {
  return Math.trunc(toNumber(value)).toLocaleString('zh-CN')
}

export function formatPercent(value, digits = 2) {
  return `${toNumber(value).toFixed(digits)}%`
}

export function formatEventTime(value) {
  if (value === null || value === undefined || value === '') return '--:--:--.---'
  const date = value instanceof Date ? value : new Date(value)
  if (!Number.isNaN(date.getTime())) {
    const pad = (part, size = 2) => String(part).padStart(size, '0')
    return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}.${pad(date.getMilliseconds(), 3)}`
  }
  const match = String(value).match(/(\d{2}):(\d{2}):(\d{2})(?:[.:](\d{1,3}))?/)
  if (!match) return String(value)
  return `${match[1]}:${match[2]}:${match[3]}.${String(match[4] || '0').padEnd(3, '0')}`
}

export function sideText(value) {
  const side = String(value || '').toLowerCase()
  const orderLabels = {
    margin_buy: '融资买入',
    sell_to_repay: '卖券还款',
    sell_repay: '卖券还款',
    short_sell: '融券卖出',
    buy_to_return: '买券还券',
    buy_return: '买券还券',
    normal_buy: '买入',
    normal_sell: '卖出',
  }
  if (orderLabels[side]) return orderLabels[side]
  if (['buy', 'b', 'long', '买', '买入'].includes(side)) return '买入'
  if (['sell', 's', 'short', '卖', '卖出'].includes(side)) return '卖出'
  return value || '--'
}

export function orderTypeText(value, fallbackSide = '') {
  const type = String(value || fallbackSide || '').toLowerCase()
  const labels = {
    buy: '普通买入',
    sell: '普通卖出',
    normal_buy: '普通买入',
    normal_sell: '普通卖出',
    margin_buy: '融资买入',
    sell_to_repay: '卖券还款',
    sell_repay: '卖券还款',
    short_sell: '融券卖出',
    buy_to_return: '买券还券',
    buy_return: '买券还券',
  }
  return labels[type] || sideText(type)
}

export function positionTypeText(value) {
  const labels = {
    cash_long: '普通多头',
    margin_long: '融资多头',
    short: '融券空头',
  }
  return labels[String(value || '').toLowerCase()] || value || '普通多头'
}

export function accountModeText(value) {
  const mode = String(value || '').toLowerCase()
  if (['margin', 'credit', '融资融券', '两融'].includes(mode)) return '两融账户'
  if (['cash', 'normal', '普通'].includes(mode)) return '普通账户'
  return value || '--'
}

export function formatRiskValue(value, unit = '') {
  if (value === null || value === undefined || value === '') return '--'
  if (unit === '%' || /percent|ratio/i.test(unit)) return formatPercent(value)
  if (/元|cny|money|amount/i.test(unit)) return `¥${formatMoney(value)}`
  return `${toNumber(value).toLocaleString('zh-CN')}${unit ? ` ${unit}` : ''}`
}

export function statusText(value) {
  const status = String(value || '').toLowerCase()
  const labels = {
    pending: '待成交',
    submitted: '已报',
    accepted: '已受理',
    partial: '部分成交',
    partially_filled: '部分成交',
    filled: '已成交',
    done: '已完成',
    cancelled: '已撤单',
    canceled: '已撤单',
    rejected: '已拒绝',
    failed: '失败',
  }
  return labels[status] || value || '--'
}

export function profitClass(value) {
  const amount = toNumber(value)
  return amount > 0 ? 'profit-up' : amount < 0 ? 'profit-down' : ''
}
