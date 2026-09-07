/**
 * Phase13 TradePlan Presentation ViewModel (P0) — read-only projection.
 * Does not call trading APIs, mutate plans, or change PlanFilter / Execution.
 *
 * @typedef {import('../api/tradePlans').UpcomingTradePlan} UpcomingTradePlan
 * @typedef {import('../api/tradePlans').UpcomingTradePlanItem} UpcomingTradePlanItem
 */

export const TRADEPLAN_PRESENTATION_SCHEMA = 'tradeplan_presentation.v1'

/**
 * @param {unknown} status
 * @returns {string}
 */
export function normalizeItemStatus(status) {
  return String(status || '')
    .trim()
    .toLowerCase()
}

/**
 * @param {UpcomingTradePlanItem | null | undefined} item
 * @returns {boolean}
 */
export function isGapSkipIntent(item) {
  return String(item?.intent_status || '')
    .trim()
    .toLowerCase() === 'gap_skip'
}

/**
 * Executable presentation row: pending / filled, not gap_skip.
 * @param {UpcomingTradePlanItem} item
 * @returns {boolean}
 */
export function isExecutionCandidate(item) {
  if (!item || isGapSkipIntent(item)) return false
  const st = normalizeItemStatus(item.status)
  return st === 'pending' || st === 'filled'
}

/**
 * Blocked / rejected scan or intent row.
 * @param {UpcomingTradePlanItem} item
 * @returns {boolean}
 */
export function isBlockedCandidate(item) {
  if (!item) return false
  if (isGapSkipIntent(item)) return true
  const st = normalizeItemStatus(item.status)
  return st === 'skipped' || st === 'error'
}

/**
 * @param {UpcomingTradePlanItem} item
 * @returns {string}
 */
export function formatBlockReason(item) {
  if (isGapSkipIntent(item)) {
    return 'gap_skip: 开盘价偏离参考价，本次不生成买入订单'
  }
  const code = String(item?.risk_code || '').trim()
  const msg = String(item?.risk_message || '').trim()
  if (code && msg) return `${code}: ${msg}`
  if (code) return code
  if (msg) return msg
  const st = normalizeItemStatus(item?.status)
  if (st === 'error') return 'error'
  if (st === 'skipped') return 'skipped'
  return 'blocked'
}

/**
 * @param {UpcomingTradePlanItem} item
 * @param {'execution' | 'research' | 'blocked'} bucket
 * @returns {object}
 */
function toPresentationRow(item, bucket) {
  const row = {
    ...item,
    bucket,
    block_reason: bucket === 'blocked' ? formatBlockReason(item) : undefined,
  }
  return row
}

/**
 * Build read-only TradePlanPresentationView.
 * researchCandidates stay empty until CandidatePool is injected (P1).
 *
 * @param {UpcomingTradePlan | null | undefined} plan
 * @param {{ poolItems?: unknown[] } | undefined} opts
 * @returns {{
 *   schemaVersion: string,
 *   planId: number,
 *   tradeDate: string,
 *   planStatus: string,
 *   poolId: number,
 *   executionCandidates: object[],
 *   researchCandidates: object[],
 *   blockedCandidates: object[],
 *   summary: {
 *     executionCount: number,
 *     researchCount: number,
 *     blockedCount: number,
 *     rawItemCount: number,
 *     researchEmpty: boolean,
 *     researchEmptyReason: string,
 *     disclaimer: string,
 *   }
 * }}
 */
export function buildTradePlanPresentation(plan, opts = {}) {
  const items = Array.isArray(plan?.items) ? plan.items : []
  const poolItems = Array.isArray(opts?.poolItems) ? opts.poolItems : null

  const executionCandidates = []
  const blockedCandidates = []

  for (const item of items) {
    if (isExecutionCandidate(item)) {
      executionCandidates.push(toPresentationRow(item, 'execution'))
      continue
    }
    if (isBlockedCandidate(item)) {
      blockedCandidates.push(toPresentationRow(item, 'blocked'))
      continue
    }
    // Unknown status → treat as blocked for safe display (not execution).
    blockedCandidates.push(toPresentationRow(item, 'blocked'))
  }

  /** @type {object[]} */
  let researchCandidates = []
  let researchEmptyReason = 'pool_not_loaded'
  if (poolItems && poolItems.length > 0) {
    // P1 hook: accept injected pool rows as research universe (read-only).
    researchCandidates = poolItems.map((p, i) => ({
      stock_code: String(p?.stock_code || p?.stockCode || ''),
      stock_name: String(p?.stock_name || p?.stockName || ''),
      score: Number(p?.score ?? p?.signal_score ?? 0),
      rank: Number(p?.rank ?? i + 1),
      bucket: 'research',
    }))
    researchEmptyReason = ''
  }

  const researchEmpty = researchCandidates.length === 0

  return {
    schemaVersion: TRADEPLAN_PRESENTATION_SCHEMA,
    planId: Number(plan?.id || 0),
    tradeDate: String(plan?.trade_date || ''),
    planStatus: String(plan?.status || ''),
    poolId: Number(plan?.pool_id || 0),
    executionCandidates,
    researchCandidates,
    blockedCandidates,
    summary: {
      executionCount: executionCandidates.length,
      researchCount: researchCandidates.length,
      blockedCount: blockedCandidates.length,
      rawItemCount: items.length,
      researchEmpty,
      researchEmptyReason: researchEmpty
        ? researchEmptyReason || 'pool_not_loaded'
        : '',
      disclaimer:
        '主表仅展示执行候选（pending/可执行）。skipped 扫描行在「阻断」区，不计入拟买入数量。研究池需单独加载 CandidatePool（P0 空态）。',
    },
  }
}

/**
 * Empty presentation when plan is null.
 * @returns {ReturnType<typeof buildTradePlanPresentation>}
 */
export function emptyTradePlanPresentation() {
  return buildTradePlanPresentation(null)
}
