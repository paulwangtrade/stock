/**
 * Phase14-A-R1-D: T-sell execution status & fill feedback (UI-only).
 */

export const SELL_CASH_NOTICE = '卖出款将在成交后计入现金'

export const T_SELL_EXEC_STATUS = {
  draft: 'draft',
  approved: 'approved',
  frozen: 'frozen',
  executing: 'executing',
  filled: 'filled',
}

export const T_SELL_EXEC_STATUS_LABEL = {
  [T_SELL_EXEC_STATUS.draft]: '等待批准卖出',
  [T_SELL_EXEC_STATUS.approved]: '等待冻结卖出',
  [T_SELL_EXEC_STATUS.frozen]: '可以执行卖出',
  [T_SELL_EXEC_STATUS.executing]: '卖出执行中',
  [T_SELL_EXEC_STATUS.filled]: '卖出完成',
}

/** Resolve high-level sell execution status for UI badge. */
export function resolveTSellExecutionStatus({
  executing = false,
  isApproved = false,
  isFrozen = false,
  isReady = false,
  hasFill = false,
} = {}) {
  if (executing) return T_SELL_EXEC_STATUS.executing
  if (hasFill) return T_SELL_EXEC_STATUS.filled
  if (isFrozen || isReady) return T_SELL_EXEC_STATUS.frozen
  if (isApproved) return T_SELL_EXEC_STATUS.approved
  return T_SELL_EXEC_STATUS.draft
}

export function tSellExecutionStatusLabel(status) {
  return T_SELL_EXEC_STATUS_LABEL[status] || '—'
}

export function tSellExecutionStatusTagType(status) {
  switch (status) {
    case T_SELL_EXEC_STATUS.filled:
      return 'success'
    case T_SELL_EXEC_STATUS.executing:
      return 'info'
    case T_SELL_EXEC_STATUS.frozen:
      return 'warning'
    case T_SELL_EXEC_STATUS.approved:
      return 'default'
    default:
      return 'default'
  }
}

function pickFilledVolume(item) {
  const raw =
    item?.filled_volume ??
    item?.filledVolume ??
    item?.FilledVolume ??
    0
  const n = Math.trunc(Number(raw) || 0)
  return n > 0 ? n : 0
}

function pickFillTime(item, lifecycle) {
  const fromItem = item?.filled_at ?? item?.filledAt ?? item?.FilledAt
  if (fromItem) return String(fromItem)
  const fromLifecycle = lifecycle?.firstFillAt ?? lifecycle?.first_fill_at
  if (fromLifecycle && fromLifecycle !== 'null') return String(fromLifecycle)
  return ''
}

/** Build fill rows from plan items (fallback when dashboard fills unavailable). */
export function buildFillRowsFromPlanItems(plan, lifecycle) {
  const items = Array.isArray(plan?.items) ? plan.items : []
  return items
    .filter((it) => String(it?.side || '').toLowerCase() === 'sell')
    .map((it) => {
      const qty = pickFilledVolume(it)
      if (qty <= 0 && String(it?.status || '').toLowerCase() !== 'filled') return null
      const price = Number(it?.filled_price ?? it?.filledPrice ?? it?.limit_price ?? 0) || 0
      return {
        stockCode: String(it?.stock_code ?? it?.stockCode ?? ''),
        stockName: String(it?.stock_name ?? it?.stockName ?? ''),
        quantity: qty || Number(it?.target_volume ?? 0) || 0,
        price,
        filledAt: pickFillTime(it, lifecycle),
        source: 'plan_item',
      }
    })
    .filter(Boolean)
}

/** Normalize dashboard fill row for sell feedback panel. */
export function mapDashboardSellFill(row) {
  if (!row) return null
  return {
    stockCode: String(row.stockCode ?? row.stock_code ?? ''),
    stockName: String(row.stockName ?? row.stock_name ?? ''),
    quantity: Math.trunc(Number(row.volume) || 0),
    price: Number(row.price) || 0,
    filledAt: String(row.filledAt ?? row.filled_at ?? ''),
    source: 'dashboard',
  }
}

/**
 * Aggregate executed quantity and fill lines for T-sell feedback panel.
 *
 * @param {{
 *   plan?: object | null,
 *   lifecycle?: object | null,
 *   dashboardFills?: object[],
 *   lastRun?: object | null,
 * }} input
 */
export function buildSellExecutionFeedback(input = {}) {
  const plan = input.plan || null
  const lifecycle = input.lifecycle || null
  const dashboardFills = Array.isArray(input.dashboardFills) ? input.dashboardFills : []
  const lastRun = input.lastRun || null

  const dashRows = dashboardFills
    .map(mapDashboardSellFill)
    .filter((r) => r && r.quantity > 0)

  const itemRows = dashRows.length ? [] : buildFillRowsFromPlanItems(plan, lifecycle)
  const fills = dashRows.length ? dashRows : itemRows

  const executedQty = fills.reduce((sum, r) => sum + (Number(r.quantity) || 0), 0)
  const runFilled = Math.trunc(Number(lastRun?.filledCount ?? lastRun?.filled_count) || 0)
  const totalExecuted = executedQty > 0 ? executedQty : runFilled

  const lifecycleFill = String(lifecycle?.firstFillAt ?? lifecycle?.first_fill_at ?? '').trim()
  const hasFill =
    totalExecuted > 0 ||
    (!!lifecycleFill && lifecycleFill !== 'null') ||
    (Array.isArray(plan?.items) &&
      plan.items.some(
        (it) =>
          String(it?.side || '').toLowerCase() === 'sell' &&
          (String(it?.status || '').toLowerCase() === 'filled' || pickFilledVolume(it) > 0),
      ))

  const fillTime =
    fills.find((f) => f.filledAt)?.filledAt ||
    (lifecycle?.firstFillAt && lifecycle.firstFillAt !== 'null' ? lifecycle.firstFillAt : '')

  return {
    hasFill,
    executedQuantity: totalExecuted,
    fills,
    fillTime,
    cashNotice: SELL_CASH_NOTICE,
    lastRunStatus: lastRun?.status ? String(lastRun.status) : '',
    lastRunMessage: lastRun?.message ? String(lastRun.message) : '',
  }
}
