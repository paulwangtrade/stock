/**
 * Phase16-D4 — Opportunity list decision tier badge (read-only).
 * Maps batch OpportunityProjection to three-tier badge + tooltip summary.
 */
import { DECISION_STATUS_LABEL } from './opportunityProjectionDisplay.js'

export const TIER = {
  SIGNAL_ONLY: 'signal_only',
  POOL_CANDIDATE: 'pool_candidate',
  TRADE_CANDIDATE: 'trade_candidate',
}

export const TIER_LABEL = {
  signal_only: '普通信号',
  pool_candidate: '池候选',
  trade_candidate: '交易候选',
}

export const OPPORTUNITY_DECISION_FOOTER =
  '「决策」列来自 CandidatePool / TradePlan 只读投影，与「趋势分」无关；「交易候选」为系统分类，非买卖建议。'

/** @param {import('../api/opportunityProjection').OpportunityProjection | null | undefined} proj */
export function tierFromProjection(proj) {
  if (!proj) return TIER.SIGNAL_ONLY
  if (proj.decision?.decision_status === 'BUY_CANDIDATE') return TIER.TRADE_CANDIDATE
  if (proj.opportunity?.present) return TIER.POOL_CANDIDATE
  return TIER.SIGNAL_ONLY
}

/** @param {string} tier */
export function tierTagType(tier) {
  switch (tier) {
    case TIER.TRADE_CANDIDATE:
      return 'warning'
    case TIER.POOL_CANDIDATE:
      return 'info'
    default:
      return 'default'
  }
}

/** @param {string | undefined | null} status */
export function decisionStatusLabel(status) {
  const k = String(status || '').trim().toUpperCase()
  return DECISION_STATUS_LABEL[k] || (status ? String(status) : '未知')
}

/** @param {import('../api/opportunityProjection').OpportunityProjectionTradePlan | null | undefined} tradePlanBlock */
export function formatTradePlanLabel(tradePlanBlock) {
  if (!tradePlanBlock?.present || !(Number(tradePlanBlock.plan_id) > 0)) {
    return '未生成'
  }
  if (tradePlanBlock.frozen) return '已生成（已冻结）'
  return '已生成'
}

/** @param {import('../api/opportunityProjection').OpportunityProjection | null | undefined} proj */
export function formatOpportunityRank(proj) {
  if (!proj?.opportunity?.present) return '未入池'
  const rank = Number(proj.opportunity.rank)
  if (Number.isFinite(rank) && rank > 0) return String(rank)
  return '未入池'
}

function resolveSignalTagForTooltip(proj, rowSignalTag) {
  const tag = proj?.signal?.signal_tag
  if (tag) return String(tag)
  if (rowSignalTag) return String(rowSignalTag)
  return '暂无记录'
}

/**
 * @param {import('../api/opportunityProjection').OpportunityProjection | null | undefined} proj
 * @param {string} rowSignalTag — formatted scan tag from signal column context
 * @param {{ loading?: boolean, error?: boolean }} [opts]
 */
export function buildDecisionBadgeView(proj, rowSignalTag, { loading = false, error = false } = {}) {
  if (loading) {
    return {
      tier: TIER.SIGNAL_ONLY,
      tagLabel: '…',
      tagType: 'default',
      tooltipLines: ['决策态加载中'],
      clickable: false,
      retry: false,
    }
  }
  if (error) {
    return {
      tier: TIER.SIGNAL_ONLY,
      tagLabel: '—',
      tagType: 'default',
      tooltipLines: ['决策态暂不可用 · 点击重试'],
      clickable: true,
      retry: true,
    }
  }

  const tier = tierFromProjection(proj)
  const tooltipLines = [
    `Signal: ${resolveSignalTagForTooltip(proj, rowSignalTag)}`,
    `Opportunity Rank: ${formatOpportunityRank(proj)}`,
    `Decision: ${proj ? decisionStatusLabel(proj.decision?.decision_status) : '未知'}`,
    `TradePlan: ${formatTradePlanLabel(proj?.trade_plan)}`,
  ]
  if (proj?.metadata?.quality === 'partial') {
    tooltipLines.push('（部分字段缺失）')
  }
  tooltipLines.push('', '数据来源：OpportunityProjection（只读）', '点击打开完整解释')

  return {
    tier,
    tagLabel: TIER_LABEL[tier],
    tagType: tierTagType(tier),
    tooltipLines,
    clickable: true,
    retry: false,
  }
}

/** Build stock_code → projection map (lowercase follow code). */
export function buildProjectionMap(items) {
  const map = new Map()
  for (const item of items || []) {
    const code = String(item?.stock_code || '').trim().toLowerCase()
    if (code) map.set(code, item)
  }
  return map
}

/** Batch limit for ProjectList: min(rowCount, 100), at least pageSize. */
export function resolveProjectionBatchLimit(filteredRowCount, pageSize = 20) {
  const rows = Math.max(Number(filteredRowCount) || 0, Number(pageSize) || 1)
  return Math.min(Math.max(rows, 1), 100)
}
