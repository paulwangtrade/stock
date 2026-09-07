/** Broker Reconcile 只读 View 映射（供 API client 与 node 校验共用） */

export const DIVERGENCE_KINDS = [
  'missing_order',
  'status_mismatch',
  'filled_qty_mismatch',
  'avg_price_mismatch',
  'cancel_pending_timeout',
  'unknown_timeout',
]

/**
 * @typedef {{ kind: string, order_id: string, detail: string, severity: string }} BrokerReconcileDivergence
 * @typedef {{
 *   status: string,
 *   checked_at: string,
 *   total_orders: number,
 *   divergence_count: number,
 *   divergences: BrokerReconcileDivergence[],
 *   by_kind: Record<string, number>,
 *   observation_error: string,
 * }} BrokerReconcileView
 */

function asRecord(v) {
  return v && typeof v === 'object' ? v : {}
}

/**
 * @param {unknown} raw
 * @returns {BrokerReconcileDivergence}
 */
export function mapDivergence(raw) {
  const d = asRecord(raw)
  return {
    kind: String(d.kind || ''),
    order_id: String(d.order_id || ''),
    detail: String(d.detail || ''),
    severity: String(d.severity || ''),
  }
}

/**
 * 固定六类 divergence 计数（缺失记 0）。
 * @param {BrokerReconcileDivergence[]} divergences
 * @returns {Record<string, number>}
 */
export function countDivergencesByKind(divergences) {
  /** @type {Record<string, number>} */
  const out = {}
  for (const k of DIVERGENCE_KINDS) out[k] = 0
  for (const d of divergences || []) {
    const kind = String(d?.kind || '')
    if (Object.prototype.hasOwnProperty.call(out, kind)) {
      out[kind] += 1
    }
  }
  return out
}

/**
 * @param {unknown} raw
 * @returns {BrokerReconcileView | null}
 */
export function mapBrokerReconcileView(raw) {
  if (!raw || typeof raw !== 'object') return null
  const body = /** @type {Record<string, unknown>} */ (raw)
  const listRaw = Array.isArray(body.divergences) ? body.divergences : []
  const divergences = listRaw.map(mapDivergence)
  const checkedAt = body.checked_at
  let checked_at = ''
  if (typeof checkedAt === 'string') {
    checked_at = checkedAt
  } else if (checkedAt != null) {
    checked_at = String(checkedAt)
  }
  return {
    status: String(body.status || ''),
    checked_at,
    total_orders: Number(body.total_orders) || 0,
    divergence_count: Number(body.divergence_count) || divergences.length,
    divergences,
    by_kind: countDivergencesByKind(divergences),
    observation_error: body.observation_error ? String(body.observation_error) : '',
  }
}
