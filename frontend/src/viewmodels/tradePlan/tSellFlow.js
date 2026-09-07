/**
 * Phase14-A-R1-C: T-sell (manual sell) TradePlan UI flow helpers.
 * UI-only — does not change TradePlan lifecycle or backend state machine.
 */

export const T_SELL_SOURCE_SESSION = 't_sell'
export const EXIT_REVIEW_SOURCE_SESSION = 'exit_review'

/** Whether the page is in manual sell (T-sell) mode. */
export function resolveIsTSellFlow({ routeFlow, sourceSession }) {
  if (String(routeFlow || '').trim().toLowerCase() === 'sell') return true
  const src = String(sourceSession || '').trim()
  return src === T_SELL_SOURCE_SESSION || src === EXIT_REVIEW_SOURCE_SESSION
}

/** User-facing label for TradePlan source_session. */
export function resolveTradePlanSourceLabel(sourceSession) {
  const s = String(sourceSession || '').trim()
  if (s === T_SELL_SOURCE_SESSION) return '人工卖出'
  if (s === EXIT_REVIEW_SOURCE_SESSION) return '退出复评'
  if (s === 'after_close') return '盘后计划'
  if (s === 'morning_rebuild') return '早盘重建'
  if (s === 'cash_rescale') return '现金缩放'
  if (s === 'watchlist') return '跟踪机会'
  return s || '手动生成'
}

/** Product bucket label from same-day-candidates `source` field. */
export function resolveTradePlanSourceBucketLabel(source) {
  switch (String(source || '').trim()) {
    case 'strategy':
      return '量化策略'
    case 'watchlist':
      return '人工关注'
    case 'manual_sell':
      return '人工卖出'
    default:
      return '其它来源'
  }
}

/** Compact status label for candidate chips. */
export function resolveCandidateStatusLabel(status, isFrozen) {
  const s = String(status || '').trim().toLowerCase()
  if (isFrozen || s === 'ready') return 'READY'
  if (s === 'draft') return 'DRAFT'
  if (s === 'executing') return 'EXECUTING'
  return (s || '—').toUpperCase()
}

export const T_SELL_GUIDANCE_PHASES = [
  { id: 1, mark: '①', label: '卖出草稿', note: 'T-sell 人工卖出 Draft；不会自动批准、冻结或执行。' },
  { id: 2, mark: '②', label: '批准卖出', note: '人工确认计划内容；批准后仍为 Draft。' },
  { id: 3, mark: '③', label: '冻结卖出', note: '冻结后变为 Ready，供 Paper Trading 读取。' },
  { id: 4, mark: '④', label: '执行卖出', note: '手动触发模拟卖出，写入 paper_sim_fills。' },
]

/** Active step ①–④ for T-sell (plan exists ⇒ step 1 done). */
export function resolveTSellGuidancePhase({ hasPlan, isApproved, isFrozen, isReady }) {
  if (!hasPlan) return 1
  if (isFrozen || isReady) return 4
  if (isApproved) return 3
  return 2
}

/** Sticky CTA kind for T-sell: approve → freeze → execute_sell. */
export function resolveTSellStickyCtaKind({ hasPlan, isApproved, isFrozen, isReady }) {
  if (!hasPlan) return 'none'
  if (isFrozen || isReady) return 'execute_sell'
  if (isApproved && !isFrozen) return 'freeze'
  return 'approve'
}

export function tSellStickyCtaLabel(kind) {
  switch (kind) {
    case 'approve':
      return '批准卖出'
    case 'freeze':
      return '冻结卖出'
    case 'execute_sell':
      return '执行卖出'
    default:
      return ''
  }
}

export function tSellStickyCtaHint(kind, ctx = {}) {
  const {
    approveDisabledReason = '',
    freezeDisabledReason = '',
    executeDisabledReason = '',
  } = ctx
  switch (kind) {
    case 'approve':
      return approveDisabledReason || '确认卖出数量与标的后批准'
    case 'freeze':
      return freezeDisabledReason || '冻结后计划变为 Ready，可手动执行卖出'
    case 'execute_sell':
      return executeDisabledReason || '手动触发 Paper Trading（plan_id 必填）'
    default:
      return ''
  }
}

export function tSellExecuteDisabledReason({
  hasPlan,
  isFrozen,
  isReady,
  executing,
  planId,
  hasFill,
}) {
  if (!hasPlan) return '当前无卖出计划'
  if (!isFrozen && !isReady) return '请先批准并冻结卖出计划'
  if (executing) return '执行中…'
  const id = Math.trunc(Number(planId) || 0)
  if (id <= 0) return '计划 ID 无效'
  if (hasFill) return '卖出已执行，可在下方生命周期查看成交时间'
  return ''
}

export function canExecuteTSell(ctx) {
  return !tSellExecuteDisabledReason(ctx)
}
