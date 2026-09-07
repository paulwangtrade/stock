/**
 * TradePlan Origin display helpers (Phase14-G2.2 + Phase16.28-B).
 * Read-only projection from GET /api/tradeplans/{id}/origin; missing → 「暂无记录」.
 * Horizontal summary + Drawer share the same mapped row (no strategy-explanation API).
 */
import { SIGNAL_PRICE_TOOLTIP } from './opportunityListMetrics.js'
import {
  provenanceSourceChipMeta,
  resolveProvenanceSourceBucket,
} from './portfolioSourceChip.js'

/** Backend sentinel when projection cannot resolve a field. */
export const ORIGIN_API_MISSING = 'missing'

export const ORIGIN_EMPTY = '暂无记录'

/** Table reason column when both selection_reason and source_reason are absent. */
export const ORIGIN_REASON_EMPTY = '暂无说明'

/** Table cell placeholder for strategy / score / signal. */
export const ORIGIN_TABLE_DASH = '—'

/** Footer: signal_price semantics (E1 aligned). */
export const ORIGIN_SIGNAL_PRICE_FOOTER =
  '信号价指出信号当日 K 线收盘价，与买入价、委托价、成交价无关。'

export const ORIGIN_SIGNAL_PRICE_TOOLTIP = SIGNAL_PRICE_TOOLTIP

export function isOriginFieldMissing(value) {
  if (value == null) return true
  const s = String(value).trim()
  return s === '' || s.toLowerCase() === ORIGIN_API_MISSING
}

export function formatOriginField(value) {
  if (isOriginFieldMissing(value)) return ORIGIN_EMPTY
  return String(value).trim()
}

export function formatOriginSignalPrice(value) {
  if (isOriginFieldMissing(value)) return ORIGIN_EMPTY
  const n = Number(String(value).replace(/,/g, ''))
  if (Number.isFinite(n) && n > 0) {
    return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  }
  return formatOriginField(value)
}

function strRaw(value) {
  if (isOriginFieldMissing(value)) return ''
  return String(value).trim()
}

/**
 * Prefer selection_reason, else source_reason; truncate for table.
 * @returns {string} summary or ORIGIN_REASON_EMPTY
 */
export function resolveOriginReasonSummary(item, maxLen = 28) {
  const row = item && typeof item === 'object' ? item : {}
  const sel = strRaw(row.selection_reason ?? row.selectionReason)
  const src = strRaw(row.source_reason ?? row.sourceReason)
  const text = sel || src
  if (!text) return ORIGIN_REASON_EMPTY
  if (text.length <= maxLen) return text
  return `${text.slice(0, Math.max(1, maxLen - 1))}…`
}

/**
 * Source chip bucket: plan source_session + item heuristics (follow → Manual).
 * Same function for table and Drawer — prevents Unknown vs Strategy drift.
 */
export function resolveOriginSourceBucket(input = {}) {
  const strategyName = strRaw(input.strategyName ?? input.strategy_name).toLowerCase()
  if (strategyName === 'follow') return 'manual'

  const sessionBucket = resolveProvenanceSourceBucket(input.sourceSession ?? input.source_session)
  if (sessionBucket !== 'unknown') return sessionBucket

  const sourceHint = String(input.source ?? input.sourceType ?? '').trim().toLowerCase()
  if (sourceHint === 'watchlist') return 'watchlist'
  if (sourceHint === 'follow' || sourceHint === 'manual') return 'manual'
  if (sourceHint === 'strategy' || sourceHint === 'strategy_run') return 'strategy'

  const reason = strRaw(input.sourceReason ?? input.source_reason).toLowerCase()
  if (reason.includes('watchlist') || reason.includes('跟踪名单') || reason.includes('自选')) {
    return 'watchlist'
  }
  if (strategyName) return 'strategy'
  if (reason) return 'strategy'
  return 'unknown'
}

export function originSourceChipMeta(input = {}) {
  return provenanceSourceChipMeta(resolveOriginSourceBucket(input))
}

/**
 * @param {object} item raw Origin API item
 * @param {{ sourceSession?: string, stockName?: string, source?: string }} [options]
 */
export function buildOriginItemDisplay(item, options = {}) {
  const row = item && typeof item === 'object' ? item : {}
  const sourceSession = options.sourceSession ?? options.source_session ?? ''
  const stockName =
    strRaw(options.stockName) ||
    strRaw(row.stock_name) ||
    strRaw(row.stockName) ||
    ''
  const stockCode = strRaw(row.stock_code) || strRaw(row.stockCode)
  const strategyName = strRaw(row.strategy_name) || strRaw(row.strategyName)
  const scoreRaw = strRaw(row.score)
  const signalTag = strRaw(row.signal_tag) || strRaw(row.signalTag)

  const chip = originSourceChipMeta({
    sourceSession,
    strategyName,
    sourceReason: row.source_reason ?? row.sourceReason,
    source: options.source,
  })

  const selectionMissing = isOriginFieldMissing(row.selection_reason ?? row.selectionReason)
  const sourceReasonMissing = isOriginFieldMissing(row.source_reason ?? row.sourceReason)

  return {
    stockCode: stockCode || ORIGIN_EMPTY,
    stockName,
    strategyName,
    strategyNameLabel: strategyName || ORIGIN_TABLE_DASH,
    score: scoreRaw,
    scoreLabel: scoreRaw || ORIGIN_TABLE_DASH,
    sourceReason: formatOriginField(row.source_reason ?? row.sourceReason),
    selectionReason: formatOriginField(row.selection_reason ?? row.selectionReason),
    reasonSummary: resolveOriginReasonSummary(row),
    signalTime: formatOriginField(row.signal_time ?? row.signalTime),
    signalPrice: formatOriginSignalPrice(row.signal_price ?? row.signalPrice),
    signalTag: signalTag ? signalTag : ORIGIN_EMPTY,
    signalTagLabel: signalTag || ORIGIN_TABLE_DASH,
    signalPresent: Boolean(signalTag) || !isOriginFieldMissing(row.signal_time ?? row.signalTime) || !isOriginFieldMissing(row.signal_price ?? row.signalPrice),
    sourceReasonMissing,
    selectionReasonMissing: selectionMissing,
    signalTimeMissing: isOriginFieldMissing(row.signal_time ?? row.signalTime),
    signalPriceMissing: isOriginFieldMissing(row.signal_price ?? row.signalPrice),
    signalTagMissing: !signalTag,
    strategyNameMissing: !strategyName,
    scoreMissing: !scoreRaw,
    sourceBucket: resolveOriginSourceBucket({
      sourceSession,
      strategyName,
      sourceReason: row.source_reason ?? row.sourceReason,
      source: options.source,
    }),
    sourceChipLabel: chip.label,
    sourceChipType: chip.type,
    sourceSession: String(sourceSession || '').trim(),
  }
}

/**
 * @param {unknown[]} items
 * @param {{ sourceSession?: string, nameByCode?: Record<string, string>, source?: string }} [options]
 */
export function buildOriginPanelModel(items, options = {}) {
  const list = Array.isArray(items) ? items : []
  const nameByCode =
    options.nameByCode && typeof options.nameByCode === 'object' ? options.nameByCode : {}
  const mapped = list.map((it) => {
    const code = strRaw(it?.stock_code) || strRaw(it?.stockCode)
    const hint = code ? nameByCode[code] || nameByCode[code.toLowerCase()] || '' : ''
    return buildOriginItemDisplay(it, {
      sourceSession: options.sourceSession,
      source: options.source,
      stockName: hint,
    })
  })
  return {
    items: mapped,
    hasAnyData: mapped.some(
      (d) =>
        !d.sourceReasonMissing ||
        !d.selectionReasonMissing ||
        !d.signalTimeMissing ||
        !d.signalPriceMissing ||
        !d.signalTagMissing ||
        !d.strategyNameMissing ||
        !d.scoreMissing,
    ),
  }
}
