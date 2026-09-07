/**
 * Phase17-A.1 — Home「我的计划」双槽展示 helpers（UI only）.
 * Does not change upcoming / same-day-candidates API semantics.
 */
import { planUserStatus } from '../api/investmentHome.ts'
import {
  provenanceSourceChipMeta,
  resolveProvenanceSourceBucket,
} from './portfolioSourceChip.js'
import { toStockDisplay } from './stockDisplay.js'

export const HOME_PLAN_EMPTY_TODAY = '今日暂无执行计划'
export const HOME_PLAN_EMPTY_NEXT = '下一交易日暂无准备计划'

/** Map same-day-candidates product `source` → chip bucket. */
export function resolveSameDaySourceBucket(source) {
  switch (String(source || '').trim().toLowerCase()) {
    case 'strategy':
      return 'strategy'
    case 'watchlist':
      return 'watchlist'
    case 'manual_sell':
      return 'manual'
    default:
      return 'unknown'
  }
}

export function homePlanSourceChipFromSession(sourceSession) {
  return provenanceSourceChipMeta(resolveProvenanceSourceBucket(sourceSession))
}

export function homePlanSourceChipFromSameDay(source) {
  return provenanceSourceChipMeta(resolveSameDaySourceBucket(source))
}

function planId(plan) {
  return Math.trunc(Number(plan?.id) || 0)
}

function itemCount(plan) {
  const items = plan?.items
  return Array.isArray(items) ? items.length : null
}

function stockModels(plan, limit = 5) {
  const items = plan?.items
  if (!Array.isArray(items)) return []
  const out = []
  for (const it of items) {
    const model = toStockDisplay({
      stock_code: it?.stock_code ?? it?.stockCode,
      stock_name: it?.stock_name ?? it?.stockName,
    })
    if (model) out.push(model)
    if (out.length >= limit) break
  }
  return out
}

/**
 * Build one home plan row from an upcoming plan DTO.
 * @param {object|null|undefined} plan
 */
export function buildHomePlanPrimaryRow(plan) {
  if (!plan) return null
  const id = planId(plan)
  if (id <= 0) return null
  const status = planUserStatus(plan)
  const session = String(plan.source_session || plan.sourceSession || '').trim()
  return {
    key: `plan-${id}`,
    planId: id,
    tradeDate: String(plan.trade_date || plan.tradeDate || '').trim(),
    statusKey: status.key,
    statusLabel: status.label,
    sourceSession: session,
    sourceChip: homePlanSourceChipFromSession(session),
    itemCount: itemCount(plan),
    stocks: stockModels(plan),
    isPrimary: true,
  }
}

/**
 * Extra same-day rows (exclude primary id). No items payload from candidates API.
 * @param {Array} candidates
 * @param {number} primaryId
 */
export function buildHomePlanExtraRows(candidates, primaryId = 0) {
  const list = Array.isArray(candidates) ? candidates : []
  const pid = Math.trunc(Number(primaryId) || 0)
  const out = []
  for (const c of list) {
    const id = Math.trunc(Number(c?.id) || 0)
    if (id <= 0 || (pid > 0 && id === pid)) continue
    const session = String(c.source_session || c.sourceSession || '').trim()
    const frozen = !!(c.is_frozen ?? c.isFrozen)
    const statusLabel = frozen
      ? '已锁定'
      : String(c.status || '').toLowerCase() === 'ready'
        ? '已锁定'
        : String(c.status || '').toLowerCase() === 'draft'
          ? '待确认'
          : String(c.status || '').toUpperCase() || '—'
    const statusKey =
      frozen || String(c.status || '').toLowerCase() === 'ready'
        ? 'locked'
        : String(c.status || '').toLowerCase() === 'draft'
          ? 'pending_confirm'
          : 'none'
    out.push({
      key: `cand-${id}`,
      planId: id,
      tradeDate: String(c.trade_date || c.tradeDate || '').trim(),
      statusKey,
      statusLabel,
      sourceSession: session,
      sourceChip: session
        ? homePlanSourceChipFromSession(session)
        : homePlanSourceChipFromSameDay(c.source),
      itemCount: null,
      stocks: [],
      isPrimary: false,
    })
  }
  return out
}

/**
 * @param {'today'|'next'} kind
 * @param {object|null} slot from buildDashboardPlanContext current_plan / next_plan
 * @param {Array} candidates same-day-candidates for slot.trade_date
 */
export function buildHomePlanSlotModel(kind, slot, candidates = []) {
  const tradeDate = String(slot?.trade_date || '').trim()
  const primary = buildHomePlanPrimaryRow(slot?.plan)
  const extras = buildHomePlanExtraRows(candidates, primary?.planId || 0)
  const emptyText = kind === 'today' ? HOME_PLAN_EMPTY_TODAY : HOME_PLAN_EMPTY_NEXT
  const title = kind === 'today' ? '今日执行' : '下一交易日准备'
  return {
    kind,
    title,
    tradeDate,
    emptyText,
    primary,
    extras,
    rows: primary ? [primary, ...extras] : extras,
    hasPlan: !!primary || extras.length > 0,
  }
}
