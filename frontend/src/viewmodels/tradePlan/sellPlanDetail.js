/**
 * Phase14-A-R1-E: T-sell sell plan detail card (UI-only presentation).
 * Fields are derived from existing TradePlan item + plan metadata; optional snapshot join for live available qty.
 */

/** Primary sell line item on a T-sell plan. */
export function resolvePrimarySellItem(plan) {
  const items = Array.isArray(plan?.items) ? plan.items : []
  const sell = items.find((it) => String(it?.side || '').trim().toLowerCase() === 'sell')
  return sell || items[0] || null
}

/** Sell reason from item.reason (T-sell) with legacy fallbacks for buy-style rows. */
export function resolveSellReason(item, plan) {
  if (!item) return '—'
  const persisted = String(item.reason || '').trim()
  if (persisted) return persisted
  for (const raw of [item.risk_message, item.strategy_name, item.entry_rule]) {
    const line = String(raw || '').trim()
    if (line) return line
  }
  const reasons = plan?.risk?.reasons
  if (Array.isArray(reasons)) {
    for (const raw of reasons) {
      const line = String(raw || '').trim()
      if (line) return line
    }
  }
  return '—'
}

/** Lookup current available qty from portfolio snapshot positions (best-effort). */
export function resolveAvailableQty(snapshotPositions, stockCode) {
  const code = String(stockCode || '').trim()
  if (!code) return null
  const rows = Array.isArray(snapshotPositions) ? snapshotPositions : []
  const row = rows.find(
    (p) => String(p?.stockCode ?? p?.stock_code ?? '').trim() === code,
  )
  if (!row) return null
  const n = Math.trunc(Number(row.availableQty ?? row.available_qty) || 0)
  return Number.isFinite(n) ? n : null
}

function pad2(n) {
  return String(n).padStart(2, '0')
}

/** Format plan.generated_at for detail card. */
export function formatPlanTimestamp(raw) {
  const s = String(raw || '').trim()
  if (!s) return '—'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return (
    `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ` +
    `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
  )
}

export function formatShareQuantity(qty) {
  if (qty == null || !Number.isFinite(Number(qty))) return '—'
  const n = Math.trunc(Number(qty))
  if (n <= 0) return '—'
  return `${n.toLocaleString('zh-CN')} 股`
}

/**
 * Build SellPlanDetailCard view model from plan item (+ optional snapshot for available qty).
 *
 * @param {{ plan?: object | null, snapshotPositions?: object[] }} input
 */
export function buildSellPlanDetailCard(input = {}) {
  const plan = input.plan || null
  if (!plan) return null
  const item = resolvePrimarySellItem(plan)
  if (!item) return null

  const stockCode = String(item.stock_code || '').trim()
  const sellQty = Math.trunc(Number(item.target_volume) || 0)
  const availableQty = resolveAvailableQty(input.snapshotPositions, stockCode)

  return {
    stockCode: stockCode || '—',
    stockName: String(item.stock_name || '').trim() || '—',
    sellQuantity: sellQty,
    sellQuantityLabel: formatShareQuantity(sellQty),
    availableQty,
    availableQtyLabel: formatShareQuantity(availableQty),
    sellReason: resolveSellReason(item, plan),
    createdAtLabel: formatPlanTimestamp(plan.generated_at),
  }
}

/** Lifecycle status tag — Draft must not use success (green). */
export function resolvePlanLifecycleTagType({ lifecycleLabel, isReady, isFrozen } = {}) {
  const label = String(lifecycleLabel || '').trim()
  if (label === 'Draft') return 'warning'
  if (isReady || isFrozen || label === 'Ready') return 'success'
  return 'default'
}
