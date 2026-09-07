/**
 * Portfolio Provenance display helpers (Phase15-A2).
 * Formats GET /api/portfolio/positions/{code}/provenance for Drawer UI.
 */
import {
  ORIGIN_EMPTY,
  ORIGIN_SIGNAL_PRICE_FOOTER,
  ORIGIN_SIGNAL_PRICE_TOOLTIP,
  formatOriginField,
  formatOriginSignalPrice,
  isOriginFieldMissing,
} from './tradePlanOriginDisplay.js'

export { ORIGIN_SIGNAL_PRICE_FOOTER, ORIGIN_SIGNAL_PRICE_TOOLTIP }

export const PROVENANCE_STATUS_LABEL = {
  complete: '来源完整',
  partial: '部分来源缺失',
}

export function provenanceStatusTagType(status) {
  return status === 'complete' ? 'success' : 'warning'
}

export function formatProvenanceFilledAt(value) {
  const s = String(value || '').trim()
  if (!s) return '—'
  const normalized = s.includes('T') ? s.replace('T', ' ').slice(0, 19) : s
  return normalized
}

export function formatProvenanceMoney(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export function formatProvenanceQty(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return '—'
  return n.toLocaleString('zh-CN')
}

export function formatSnapshotId(value) {
  const n = Math.trunc(Number(value) || 0)
  return n > 0 ? String(n) : ORIGIN_EMPTY
}

export function buildProvenanceOriginDisplay(origin) {
  const row = origin && typeof origin === 'object' ? origin : {}
  const signal = row.signal && typeof row.signal === 'object' ? row.signal : {}
  const reason = row.reason && typeof row.reason === 'object' ? row.reason : {}
  return {
    planId: Math.trunc(Number(row.planId) || 0),
    strategy: formatOriginField(row.strategy),
    strategyMissing: isOriginFieldMissing(row.strategy),
    signalTag: formatOriginField(signal.tag),
    signalTagMissing: isOriginFieldMissing(signal.tag),
    signalTime: formatOriginField(signal.time),
    signalTimeMissing: isOriginFieldMissing(signal.time),
    signalPrice: formatOriginSignalPrice(signal.price),
    signalPriceMissing: isOriginFieldMissing(signal.price),
    snapshotId: formatSnapshotId(signal.snapshotId),
    snapshotIdMissing: !(Math.trunc(Number(signal.snapshotId) || 0) > 0),
    buyReason: formatOriginField(reason.source),
    buyReasonMissing: isOriginFieldMissing(reason.source),
    selectionReason: formatOriginField(reason.selection),
    selectionReasonMissing: isOriginFieldMissing(reason.selection),
    score: formatOriginField(row.score),
  }
}

/** Join trades with matching origin by planId for section rendering. */
export function buildProvenanceSections(trades, origins) {
  const tradeList = Array.isArray(trades) ? trades : []
  const originList = Array.isArray(origins) ? origins : []
  const originByPlan = Object.create(null)
  for (const o of originList) {
    const pid = Math.trunc(Number(o?.planId) || 0)
    if (pid > 0) originByPlan[pid] = o
  }

  return tradeList.map((trade) => {
    const planId = Math.trunc(Number(trade?.planId) || 0)
    return {
      planId,
      trade: {
        fillPrice: formatProvenanceMoney(trade?.fillPrice),
        fillVolume: formatProvenanceQty(trade?.fillVolume),
        filledAt: formatProvenanceFilledAt(trade?.filledAt || trade?.tradeDate),
        fillId: trade?.fillId,
      },
      origin: buildProvenanceOriginDisplay(originByPlan[planId] || { planId }),
    }
  })
}

/** Origins without matching trade (edge case). */
export function orphanOrigins(trades, origins) {
  const tradePlanIds = new Set(
    (Array.isArray(trades) ? trades : [])
      .map((t) => Math.trunc(Number(t?.planId) || 0))
      .filter((id) => id > 0),
  )
  return (Array.isArray(origins) ? origins : [])
    .filter((o) => {
      const pid = Math.trunc(Number(o?.planId) || 0)
      return pid > 0 && !tradePlanIds.has(pid)
    })
    .map((o) => buildProvenanceOriginDisplay(o))
}
