/**
 * Investment Narrative display helpers (Phase14-G0.5b).
 * Read-only labels; missing fields → 「暂无记录」; no speculative copy.
 */
import { exitReviewDecisionLabel } from './exitReviewDisplay.js'
import { formatPctWithSign } from './opportunityListMetrics.js'

export const NARRATIVE_EMPTY = '暂无记录'
export const SIGNAL_PRICE_LABEL = '出信号价'
export const CURRENT_PRICE_LABEL = '当前价'

export function narrativeField(value) {
  if (value == null || value === '') return NARRATIVE_EMPTY
  if (typeof value === 'number' && !Number.isFinite(value)) return NARRATIVE_EMPTY
  const s = String(value).trim()
  return s || NARRATIVE_EMPTY
}

export function narrativeMoney(value, digits = 2) {
  const n = Number(value)
  if (!Number.isFinite(n)) return NARRATIVE_EMPTY
  return n.toLocaleString('zh-CN', { minimumFractionDigits: digits, maximumFractionDigits: digits })
}

export function formatOpportunityActionLabel(action) {
  switch (String(action || '').trim().toUpperCase()) {
    case 'WATCH':
      return '已跟踪'
    case 'VIEW':
      return '已查看'
    case 'IGNORE':
      return '已忽略'
    default:
      return NARRATIVE_EMPTY
  }
}

export function buildDiscoverySection(narrative) {
  const d = narrative?.discovery
  if (!d?.exists) {
    return {
      hasData: false,
      signalTime: NARRATIVE_EMPTY,
      signalPrice: NARRATIVE_EMPTY,
      signalTag: NARRATIVE_EMPTY,
    }
  }
  return {
    hasData: true,
    signalTime: narrativeField(d.signal_time),
    signalPrice:
      d.signal_price != null && Number.isFinite(Number(d.signal_price))
        ? narrativeMoney(d.signal_price)
        : NARRATIVE_EMPTY,
    signalTag: narrativeField(d.reason),
  }
}

export function buildOpportunitySection(latestUserAction) {
  const action = latestUserAction?.action
  if (!action) {
    return {
      hasData: false,
      actionLabel: NARRATIVE_EMPTY,
      watchState: NARRATIVE_EMPTY,
    }
  }
  const label = formatOpportunityActionLabel(action)
  return {
    hasData: true,
    actionLabel: label,
    watchState: String(action || '').trim().toUpperCase() === 'WATCH' ? '已跟踪' : label,
  }
}

export function buildPlanOriginSection(narrative) {
  const p = narrative?.plan_origin
  if (!p?.exists) {
    return {
      hasData: false,
      strategy: NARRATIVE_EMPTY,
      reason: NARRATIVE_EMPTY,
      planId: NARRATIVE_EMPTY,
    }
  }
  return {
    hasData: true,
    strategy: narrativeField(p.strategy),
    reason: narrativeField(p.reason),
    planId: p.plan_id != null && p.plan_id > 0 ? `#${p.plan_id}` : NARRATIVE_EMPTY,
  }
}

export function buildHoldingSection(narrative) {
  const h = narrative?.holding_basis
  if (!h?.exists) {
    return {
      hasData: false,
      quantity: NARRATIVE_EMPTY,
      avgCost: NARRATIVE_EMPTY,
    }
  }
  return {
    hasData: true,
    quantity:
      h.quantity != null && Number.isFinite(Number(h.quantity))
        ? `${Number(h.quantity).toLocaleString('zh-CN')} 股`
        : NARRATIVE_EMPTY,
    avgCost:
      h.avg_cost != null && Number.isFinite(Number(h.avg_cost))
        ? narrativeMoney(h.avg_cost)
        : NARRATIVE_EMPTY,
  }
}

export function buildExitReviewSection(narrative) {
  const e = narrative?.exit_review
  if (!e?.exists) {
    const na = String(e?.status || '').trim() === 'not_applicable'
    return {
      hasData: false,
      notApplicable: na,
      status: NARRATIVE_EMPTY,
      outcome: NARRATIVE_EMPTY,
    }
  }
  const outcome = e.latest_outcome
  const outcomeLabel = outcome?.decision
    ? exitReviewDecisionLabel(outcome.decision)
    : NARRATIVE_EMPTY
  return {
    hasData: true,
    notApplicable: false,
    status: narrativeField(e.status),
    outcome: outcomeLabel,
    outcomeReason: outcome?.reason ? narrativeField(outcome.reason) : NARRATIVE_EMPTY,
  }
}

export function buildPriceStorySection(narrative) {
  const ps = narrative?.price_story
  if (!ps || ps.status === 'missing') {
    return {
      hasData: false,
      signalPrice: NARRATIVE_EMPTY,
      currentPrice: NARRATIVE_EMPTY,
      vsSignalPct: NARRATIVE_EMPTY,
    }
  }
  return {
    hasData: ps.status !== 'missing',
    signalPrice:
      ps.signal_price != null && Number.isFinite(Number(ps.signal_price))
        ? narrativeMoney(ps.signal_price)
        : NARRATIVE_EMPTY,
    currentPrice:
      ps.current_price != null && Number.isFinite(Number(ps.current_price))
        ? narrativeMoney(ps.current_price)
        : NARRATIVE_EMPTY,
    vsSignalPct:
      ps.vs_signal_pct != null && Number.isFinite(Number(ps.vs_signal_pct))
        ? formatPctWithSign(Number(ps.vs_signal_pct))
        : NARRATIVE_EMPTY,
  }
}

/** Merge narrative view model for panel/tests. */
export function buildNarrativeDisplayModel(narrative, latestUserAction = null) {
  return {
    stockCode: narrativeField(narrative?.stock_code),
    discovery: buildDiscoverySection(narrative),
    opportunity: buildOpportunitySection(latestUserAction),
    planOrigin: buildPlanOriginSection(narrative),
    holding: buildHoldingSection(narrative),
    exitReview: buildExitReviewSection(narrative),
    priceStory: buildPriceStorySection(narrative),
  }
}
