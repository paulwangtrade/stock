/**
 * Phase16.18-B2 — frontend price display semantics (labels only; no calc / API change).
 * Phase16.21-D — kind tooltips via statusDisplay.formatFieldTooltip.
 */

import { formatFieldTooltip } from './statusDisplay.js'

/** @typedef {'last'|'mark'|'close'|'signal'|'ref'|'limit'|'cost'|'filled'|'open'} PriceKind */

export const PRICE_KIND = {
  last: 'last',
  mark: 'mark',
  close: 'close',
  signal: 'signal',
  ref: 'ref',
  limit: 'limit',
  cost: 'cost',
  filled: 'filled',
  open: 'open',
}

/** Short Chinese labels for column headers / inline labels. */
const KIND_LABEL = {
  last: '最新行情价',
  mark: '持仓估值价',
  close: '收盘价',
  signal: '信号触发价',
  ref: '策略参考价',
  limit: '限价（参考）',
  cost: '持仓成本价',
  filled: '成交价',
  open: '开盘价',
}

/**
 * @param {PriceKind|string} kind
 * @returns {string}
 */
export function priceSourceLabel(kind) {
  const k = String(kind || '').trim().toLowerCase()
  if (k === 'mark_price' || k === 'markprice') return KIND_LABEL.mark
  if (k === 'signal_price' || k === 'signalprice') return KIND_LABEL.signal
  if (k === 'ref_price' || k === 'refprice') return KIND_LABEL.ref
  if (k === 'cost_price' || k === 'avg_cost' || k === 'avgcost') return KIND_LABEL.cost
  if (k === 'limit_price' || k === 'limitprice') return KIND_LABEL.limit
  if (k === 'filled_price' || k === 'fill_price') return KIND_LABEL.filled
  if (k === 'close_price') return KIND_LABEL.close
  if (k === 'last_price' || k === 'display_price') return KIND_LABEL.last
  return KIND_LABEL[k] || '价格'
}

/**
 * Format numeric price for display (no label).
 * @param {unknown} value
 * @param {{ digits?: number, empty?: string }} [opts]
 */
export function formatPriceValue(value, opts = {}) {
  const empty = opts.empty ?? '—'
  const digits = opts.digits ?? 2
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return empty
  const maxDigits = Math.max(digits, 4)
  return n.toLocaleString('zh-CN', {
    minimumFractionDigits: digits,
    maximumFractionDigits: maxDigits,
  })
}

/**
 * "最新行情价 14.50" or with asOf: "策略参考价（2026-09-03） 14.50"
 * @param {unknown} value
 * @param {PriceKind|string} kind
 * @param {{ asOf?: string, source?: string, digits?: number, empty?: string }} [opts]
 */
export function formatPriceLabel(value, kind, opts = {}) {
  const label = priceSourceLabel(kind)
  const formatted = formatPriceValue(value, opts)
  if (formatted === (opts.empty ?? '—')) return `${label} ${formatted}`
  const asOf = String(opts.asOf || '').trim()
  if (asOf) return `${label}（${asOf}） ${formatted}`
  return `${label} ${formatted}`
}

/**
 * Structured display payload for tables / tooltips.
 * @param {unknown} value
 * @param {PriceKind|string} kind
 * @param {{ asOf?: string, source?: string, digits?: number, empty?: string, extraTooltip?: string }} [opts]
 * @returns {{ label: string, value: string, tooltip: string, kind: string }}
 */
export function formatPriceWithContext(value, kind, opts = {}) {
  const label = priceSourceLabel(kind)
  const formatted = formatPriceValue(value, opts)
  const asOf = String(opts.asOf || '').trim()
  const source = String(opts.source || '').trim()
  const kindTip = priceKindTooltip(kind)
  const lines = [`${label}：${formatted}`]
  if (kindTip) lines.push(kindTip)
  if (asOf) lines.push(`时间/业务日：${asOf}`)
  if (source) lines.push(`来源：${source}`)
  if (opts.extraTooltip) lines.push(String(opts.extraTooltip))
  return {
    label,
    value: formatted,
    tooltip: lines.join('\n'),
    kind: String(kind || ''),
  }
}

/** Column title helper (short). */
export function priceColumnTitle(kind) {
  return priceSourceLabel(kind)
}

/** Beta-facing tooltip for a price kind (display only). */
export function priceKindTooltip(kind) {
  const k = String(kind || '').trim().toLowerCase()
  if (k === 'mark_price' || k === 'markprice' || k === 'mark') return '持仓估值用价格，仅供展示'
  if (k === 'signal_price' || k === 'signalprice' || k === 'signal') return formatFieldTooltip('signal_price')
  if (k === 'ref_price' || k === 'refprice' || k === 'ref') return formatFieldTooltip('ref')
  if (k === 'limit_price' || k === 'limitprice' || k === 'limit') return formatFieldTooltip('limit')
  if (k === 'last_price' || k === 'display_price' || k === 'last') return formatFieldTooltip('last')
  if (k === 'cost_price' || k === 'avg_cost' || k === 'avgcost' || k === 'cost') {
    return '持仓成本价，仅供展示'
  }
  if (k === 'filled_price' || k === 'fill_price' || k === 'filled') return '成交价，仅供展示'
  if (k === 'close_price' || k === 'close') return '收盘价，仅供展示'
  if (k === 'open') return '开盘价，仅供展示'
  return formatFieldTooltip(k)
}
