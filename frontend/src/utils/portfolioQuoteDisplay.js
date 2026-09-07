import {
  formatPriceLabel,
  formatPriceValue,
  formatPriceWithContext,
  priceColumnTitle,
  priceSourceLabel,
  PRICE_KIND,
} from './priceDisplay.js'

/**
 * Portfolio quote display helpers (Phase15-B1 + Phase16.18-B2 labels).
 * Read-only overlay: display_price > mark_price; does not affect ledger equity.
 */

export const QUOTE_SOURCE_LIVE = 'live'
export const QUOTE_SOURCE_OPEN_FALLBACK = 'open_fallback'
export const QUOTE_SOURCE_PERSISTED = 'persisted'

/** True when display_price came from QuoteService (live or open fallback). */
export function isLiveQuoteSource(source) {
  const v = String(source || '').trim().toLowerCase()
  return v === QUOTE_SOURCE_LIVE || v === QUOTE_SOURCE_OPEN_FALLBACK || v === 'tencent'
}

function isPositivePrice(value) {
  const n = Number(value)
  return Number.isFinite(n) && n > 0
}

/**
 * Resolve row current price for display.
 * Priority: display_price (valid) > mark_price.
 */
export function resolveDisplayPrice(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  const display = rowObj.displayPrice ?? rowObj.display_price
  const mark = rowObj.markPrice ?? rowObj.mark_price
  if (isPositivePrice(display)) return Number(display)
  if (isPositivePrice(mark)) return Number(mark)
  const n = Number(mark)
  return Number.isFinite(n) ? n : null
}

/** Ledger mark price (unchanged accounting basis). */
export function resolveMarkPrice(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  const mark = rowObj.markPrice ?? rowObj.mark_price
  const n = Number(mark)
  return Number.isFinite(n) ? n : null
}

export function resolveQuoteSource(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  return String(rowObj.displayQuoteSource ?? rowObj.display_quote_source ?? '').trim()
}

/** Short UI label: live overlay → 行情；ledger mark → 估值. */
export function quoteSourceShortLabel(source) {
  return isLiveQuoteSource(source) ? '行情' : '估值'
}

/** Naive tag type for quote source chip. */
export function quoteSourceTagType(source) {
  return isLiveQuoteSource(source) ? 'success' : 'default'
}

/** Whether displayed price differs from ledger mark (overlay active for this row). */
export function isPriceOverlayActive(row) {
  const display = resolveDisplayPrice(row)
  const mark = resolveMarkPrice(row)
  if (display == null || mark == null) return false
  return isLiveQuoteSource(resolveQuoteSource(row)) && Math.abs(display - mark) > 1e-9
}

/** Semantic kind for the price shown in the「行情/估值」column. */
export function resolveDisplayPriceKind(row) {
  const source = resolveQuoteSource(row)
  if (isLiveQuoteSource(source)) return PRICE_KIND.last
  return PRICE_KIND.mark
}

/** Tooltip lines for current price column (Chinese semantics). */
export function buildQuotePriceTooltip(row, formatMoney) {
  const fmt = typeof formatMoney === 'function' ? formatMoney : (v) => formatPriceValue(v)
  const display = resolveDisplayPrice(row)
  const mark = resolveMarkPrice(row)
  const source = resolveQuoteSource(row) || QUOTE_SOURCE_PERSISTED
  const kind = resolveDisplayPriceKind(row)
  const quoteTs = formatQuoteTimestamp(row)
  const freshness = resolvePriceFreshness(row)
  const extra = [
    `${priceSourceLabel(PRICE_KIND.mark)}：${mark == null ? '—' : fmt(mark)}`,
    quoteTs ? `行情时间：${quoteTs}` : null,
    freshness && freshness !== 'UNKNOWN' ? `行情新鲜度：${freshness}` : null,
    '盈亏 / 总权益按持仓估值价（账本），不计入行情 overlay',
  ]
    .filter(Boolean)
    .join('\n')
  const ctx = formatPriceWithContext(display, kind, {
    source: isLiveQuoteSource(source) ? `display_quote_source=${source}` : 'mark_price（账本）',
    extraTooltip: extra,
  })
  return ctx.tooltip
}

/** Phase17.2: quote_price if present, else live display_price; never mark. */
export function resolveQuotePriceOnly(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  const qp = rowObj.quotePrice ?? rowObj.quote_price
  if (isPositivePrice(qp)) return Number(qp)
  if (!isLiveQuoteSource(resolveQuoteSource(rowObj))) return null
  const display = rowObj.displayPrice ?? rowObj.display_price
  if (isPositivePrice(display)) return Number(display)
  return null
}

export function resolvePriceFreshness(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  return String(rowObj.priceFreshness ?? rowObj.price_freshness ?? 'UNKNOWN').trim().toUpperCase() || 'UNKNOWN'
}

/** Format quote_timestamp for UI (local clock string). */
export function formatQuoteTimestamp(row) {
  const rowObj = row && typeof row === 'object' ? row : {}
  const raw = rowObj.quoteTimestamp ?? rowObj.quote_timestamp
  if (raw == null || raw === '') return ''
  const d = raw instanceof Date ? raw : new Date(String(raw))
  if (Number.isNaN(d.getTime())) return String(raw)
  const pad = (n) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

export function priceFreshnessShortLabel(freshness) {
  const f = String(freshness || '').toUpperCase()
  if (f === 'FRESH') return '新鲜'
  if (f === 'STALE') return '陈旧'
  return ''
}

export {
  formatPriceLabel,
  formatPriceValue,
  formatPriceWithContext,
  priceColumnTitle,
  priceSourceLabel,
  PRICE_KIND,
}
