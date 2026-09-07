/**
 * Phase14-A-R0-C: Intent materialize preflight (UI-only).
 * Uses plan.items from upcoming API — does not change backend materialize / selection / qty policy.
 */

export const INTENT_STATUS_SELECTED = 'selected'
export const INTENT_STATUS_PRICED = 'priced'
export const INTENT_STATUS_GAP_SKIP = 'gap_skip'
export const INTENT_STATUS_SIZE_SKIP = 'size_skip'

export const PREFLIGHT_BLOCK_NO_SELECTED = '暂无可执行计划，请检查候选选择'

/**
 * @param {unknown} status
 * @returns {string}
 */
export function normalizeIntentStatus(status) {
  return String(status || '')
    .trim()
    .toLowerCase()
}

/**
 * @param {{ side?: string } | null | undefined} item
 * @returns {boolean}
 */
export function isBuyIntentItem(item) {
  return String(item?.side || '')
    .trim()
    .toLowerCase() === 'buy'
}

/**
 * Price anchor for after-close selected intents (ref_price from plan item).
 * @param {{ ref_price?: number } | null | undefined} item
 * @returns {boolean}
 */
export function hasPriceAnchor(item) {
  const ref = Number(item?.ref_price)
  return Number.isFinite(ref) && ref > 0
}

/**
 * @param {{ target_volume?: number } | null | undefined} item
 * @returns {boolean}
 */
export function hasTargetVolume(item) {
  const vol = Number(item?.target_volume)
  return Number.isFinite(vol) && vol > 0
}

/**
 * @param {import('../../api/tradePlans').UpcomingTradePlanItem | Record<string, unknown>} item
 */
export function analyzeIntentItem(item) {
  const intent = normalizeIntentStatus(item?.intent_status)
  return {
    stockCode: String(item?.stock_code || '').trim(),
    side: String(item?.side || '').trim(),
    intentStatus: intent,
    isSelected: intent === INTENT_STATUS_SELECTED,
    isPriced: intent === INTENT_STATUS_PRICED,
    isGapSkip: intent === INTENT_STATUS_GAP_SKIP,
    isSizeSkip: intent === INTENT_STATUS_SIZE_SKIP,
    hasPriceAnchor: hasPriceAnchor(item),
    hasTargetVolume: hasTargetVolume(item),
  }
}

/**
 * Preflight before POST materialize-morning.
 *
 * @param {import('../../api/tradePlans').UpcomingTradePlan | null | undefined} plan
 * @param {{ isFrozen?: boolean } | undefined} opts
 */
export function buildIntentMaterializePreflight(plan, opts = {}) {
  const items = Array.isArray(plan?.items) ? plan.items : []
  const status = String(plan?.status || '')
    .trim()
    .toLowerCase()
  const isFrozen = !!plan?.freeze?.is_frozen || !!opts.isFrozen
  const riskPassed = !!plan?.risk?.passed

  let selectedCount = 0
  let emptyIntentCount = 0
  let pricedCount = 0
  let gapSkipCount = 0
  let selectedMissingAnchorCount = 0
  let selectedWithVolumeCount = 0
  let pricedMissingVolumeCount = 0

  for (const item of items) {
    if (!isBuyIntentItem(item)) continue
    const row = analyzeIntentItem(item)
    if (!row.intentStatus) emptyIntentCount += 1
    if (row.isSelected) {
      selectedCount += 1
      if (!row.hasPriceAnchor) selectedMissingAnchorCount += 1
      if (row.hasTargetVolume) selectedWithVolumeCount += 1
    }
    if (row.isPriced) {
      pricedCount += 1
      if (!row.hasTargetVolume) pricedMissingVolumeCount += 1
    }
    if (row.isGapSkip) gapSkipCount += 1
  }

  const canMaterialize = status === 'draft' && !isFrozen && selectedCount > 0

  let blockReason = ''
  if (selectedCount === 0) {
    blockReason = PREFLIGHT_BLOCK_NO_SELECTED
  }

  let warnReason = ''
  if (selectedCount > 0 && selectedMissingAnchorCount > 0) {
    warnReason = `${selectedMissingAnchorCount} 项 selected 缺少参考价，物化后可能无法产生可执行数量`
  }

  return {
    selectedCount,
    emptyIntentCount,
    pricedCount,
    gapSkipCount,
    selectedMissingAnchorCount,
    selectedWithVolumeCount,
    pricedMissingVolumeCount,
    canMaterialize,
    blockReason,
    warnReason,
    riskPassed,
    summaryLine: buildPreflightSummaryLine({
      selectedCount,
      pricedCount,
      selectedMissingAnchorCount,
      pricedMissingVolumeCount,
    }),
  }
}

function buildPreflightSummaryLine(counts) {
  const parts = [`selected ${counts.selectedCount}`]
  if (counts.pricedCount) parts.push(`priced ${counts.pricedCount}`)
  if (counts.selectedMissingAnchorCount) parts.push(`缺参考价 ${counts.selectedMissingAnchorCount}`)
  if (counts.pricedMissingVolumeCount) parts.push(`缺数量 ${counts.pricedMissingVolumeCount}`)
  return parts.join(' · ')
}

/**
 * Classify materialize-morning API outcome for toast / banner (UI-only).
 *
 * @param {{ success?: boolean, materialized_items?: number, readiness_ready?: boolean, blockers?: unknown[], message?: string, failed_step?: string, code?: number } | null | undefined} res
 */
export function evaluateMaterializeOutcome(res) {
  if (!res || typeof res !== 'object') {
    return {
      level: 'error',
      ok: false,
      toastMessage: '准备价格失败：无响应',
      title: '准备价格失败',
    }
  }

  if (!res.success) {
    const reason =
      String(res.message || '').trim() ||
      (res.failed_step ? `步骤失败：${res.failed_step}` : '') ||
      (res.code != null ? `业务码 ${res.code}` : '') ||
      '未知错误'
    return {
      level: 'error',
      ok: false,
      toastMessage: `准备价格失败：${reason}`,
      title: '准备价格失败',
    }
  }

  const materialized = Math.trunc(Number(res.materialized_items) || 0)
  if (materialized <= 0) {
    return {
      level: 'warning',
      ok: false,
      toastMessage: '物化未产生可执行项，请查看 Readiness 阻断明细',
      title: '物化未产生可执行项',
    }
  }

  const readiness = res.readiness_ready ? 'Ready' : 'Blocked'
  return {
    level: 'success',
    ok: true,
    toastMessage: `准备价格成功：物化 ${materialized} 项；Readiness ${readiness}`,
    title: '准备价格成功',
  }
}

/**
 * Display block for materialize result panel.
 *
 * @param {Parameters<typeof evaluateMaterializeOutcome>[0]} res
 * @returns {{ ok: boolean, level: string, title: string, detailLines: string[], toastMessage: string }}
 */
export function formatMaterializeOutcomeDisplay(res) {
  const outcome = evaluateMaterializeOutcome(res)
  const blockers = Array.isArray(res?.blockers) ? res.blockers : []
  const materialized = Math.trunc(Number(res?.materialized_items) || 0)

  if (!res || typeof res !== 'object') {
    return {
      ...outcome,
      detailLines: ['无响应'],
    }
  }

  if (!res.success) {
    const lines = [outcome.toastMessage.replace(/^准备价格失败：/, '')]
    if (res.failed_step) lines.push(`失败步骤：${res.failed_step}`)
    if (blockers.length) {
      lines.push(`阻断（${blockers.length}）：`)
      for (const b of blockers.slice(0, 5)) {
        const code = String(b?.code || b?.rule_code || '—')
        lines.push(`- [${code}] ${String(b?.message || '').trim()}`.trim())
      }
    }
    return { ...outcome, detailLines: lines.filter(Boolean) }
  }

  const lines = [
    `物化条目：${materialized}`,
    `Readiness：${res.readiness_ready ? 'Ready' : 'Blocked'}`,
  ]

  if (materialized <= 0) {
    lines.unshift('未写入限价/数量；请检查 selected 候选与参考价')
  }

  if (blockers.length) {
    lines.push(`阻断（${blockers.length}）：`)
    for (const b of blockers.slice(0, 8)) {
      const code = String(b?.code || b?.rule_code || '—')
      lines.push(`- [${code}] ${String(b?.message || '').trim()}`.trim())
    }
    if (blockers.length > 8) {
      lines.push(`…另有 ${blockers.length - 8} 项`)
    }
  } else {
    lines.push('阻断：无')
  }

  return { ...outcome, detailLines: lines }
}
