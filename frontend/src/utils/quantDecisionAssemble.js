/** QuantDecision Phase1-A：Watchlist 扫描产物 → Decision 形对象（js_legacy，不接 Go） */

import { BUY_ENTRY_TAGS } from './buyPriceRange.js'
import {
  deriveQuantAction,
  QUANT_ACTION_ENTER,
  QUANT_ACTION_SCALE_IN,
  QUANT_ACTION_REDUCE,
  QUANT_ACTION_EXIT_PARTIAL,
} from './quantActionDerive.js'
import { resolveTradingLevel } from './tradingLevelRules.js'
import { ensureDecisionId } from './quantDecisionObservability.js'
import { resolveAuthorityHoldingDecision } from './riskintel/holdingDecisionAdapter.js'

const SCHEMA_VERSION = 1

function isSellSignalTag(tag) {
  return tag === '止' || tag === '减'
}
function isAddPositionTag(tag) {
  return tag === '加'
}
function isRushReduceTag(tag) {
  return tag === '冲'
}

function resolveTagKind(tag) {
  if (!tag) return 'none'
  if (isSellSignalTag(tag)) return 'exit'
  if (isRushReduceTag(tag)) return 'rush_reduce'
  if (isAddPositionTag(tag)) return 'scale_in'
  if (tag === '冰') return 'ice'
  if (BUY_ENTRY_TAGS.has(tag)) return 'entry'
  return 'none'
}

function mapEntryZone(buyPriceRange) {
  if (!buyPriceRange) return null
  return {
    low: buyPriceRange.low,
    high: buyPriceRange.high ?? buyPriceRange.rangeHigh,
    instantPrice: buyPriceRange.instantPrice,
    /** 保真：decision-only 投影需还原可见文案 */
    instantText: buyPriceRange.instantText || '',
    mode: buyPriceRange.mode || 'unknown',
    deferMode: buyPriceRange.deferMode || 'same',
    daysAgo: buyPriceRange.daysAgo ?? 0,
    text: buyPriceRange.text || '',
    note: buyPriceRange.note || '',
    extended: !!buyPriceRange.extended,
    tag: buyPriceRange.tag || '',
    rangeHigh: buyPriceRange.rangeHigh ?? buyPriceRange.high,
  }
}

function mapGate(checklist) {
  if (!checklist) {
    return {
      score: 0,
      ready: false,
      requiredPassed: false,
      readyThreshold: 0.85,
      items: [],
    }
  }
  return {
    score: checklist.score ?? 0,
    ready: !!checklist.ready,
    requiredPassed: !!checklist.requiredPassed,
    readyThreshold: checklist.readyThreshold ?? 0.85,
    items: Array.isArray(checklist.items) ? checklist.items : [],
  }
}

function mapSize(plan) {
  if (!plan) {
    return { ok: false, reason: '' }
  }
  return {
    ok: !!plan.ok,
    entryPrice: plan.entryPrice,
    stopPrice: plan.stopPrice,
    riskPerShare: plan.riskPerShare,
    confidence: plan.confidence,
    targetShares: plan.suggestedShares ?? 0,
    addShares: plan.suggestedAddShares ?? 0,
    targetAmount: plan.suggestedAmount ?? 0,
    positionPct: plan.positionPct,
    reason: plan.reason || '',
    bindingConstraint: plan.bindingConstraint || '',
  }
}

/** Map authority holding decision → Decision.holdingBias (shape unchanged). */
function mapHoldingBias(authorityDecision) {
  if (!authorityDecision) return null
  return {
    action: authorityDecision.action,
    actionLabel: authorityDecision.actionLabel,
    score: authorityDecision.score,
    suggestPct: authorityDecision.suggestPct ?? (authorityDecision.suggestPctDisplay != null
      ? authorityDecision.suggestPctDisplay / 100
      : 0),
    summaryLine: authorityDecision.summaryLine || '',
    factors: Array.isArray(authorityDecision.factors) ? authorityDecision.factors : [],
  }
}

function resolveRatioPct(entry) {
  if (entry?.sellPositionPct != null) return Number(entry.sellPositionPct)
  if (entry?.addPositionPct != null) return Number(entry.addPositionPct)
  if (entry?.rushReducePct != null) return Number(entry.rushReducePct)
  return undefined
}

function computeAllowDraft(derived, size, entry, existingVolume) {
  if (!derived) return false
  if (
    (derived.code === QUANT_ACTION_ENTER || derived.code === QUANT_ACTION_SCALE_IN)
    && size?.ok
    && ((size.addShares > 0) || (size.targetShares > 0))
  ) {
    return true
  }
  const sellPct = entry?.sellPositionPct ?? entry?.rushReducePct
  if (
    (derived.code === QUANT_ACTION_REDUCE || derived.code === QUANT_ACTION_EXIT_PARTIAL)
    && Number(sellPct) > 0
    && Number(existingVolume) > 0
  ) {
    return true
  }
  return false
}

/**
 * @param {object} params
 * @param {object} params.entry - scanWatchlistSignals 条目
 * @param {object} [params.checklist] - evaluateBuyChecklist 结果
 * @param {object} [params.plan] - calcBuyPositionPlan 结果
 * @param {string} [params.marketModeKey]
 * @param {number} [params.existingVolume]
 * @param {string} [params.purpose]
 * @returns {object} QuantDecision 形（schemaVersion=1, producer=js_legacy）
 */
export function assembleWatchlistDecision({
  entry,
  checklist = null,
  plan = null,
  marketModeKey = 'unknown',
  existingVolume = 0,
  purpose = 'watchlist',
  tradeDate = '',
  asOf = null,
} = {}) {
  const tag = entry?.tag || ''
  const daysAgo = entry?.daysAgo ?? entry?.recentSignalDaysAgo ?? 0
  const buyPriceRange = entry?.buyPriceRange ?? null
  const regimeRule = resolveTradingLevel(marketModeKey)
  // Phase9-C.1: authority via adapter choke (source locked legacy; consumers do not see source)
  const authorityHolding = resolveAuthorityHoldingDecision(entry || {})

  const deriveCtx = {
    tag,
    buyPriceRange,
    sellPositionPct: entry?.sellPositionPct,
    addPositionPct: entry?.addPositionPct,
    rushReducePct: entry?.rushReducePct,
    sellVolume: entry?.sellVolume,
    costPrice: entry?.costPrice,
    sourceTag: entry?.sourceTag,
    daysAgo,
    isHistorical: daysAgo > 0,
    checklistReady: checklist?.ready === true,
    holdingDecision: entry?.holdingDecision,
    holdingAdvice: authorityHolding,
  }

  const derived = deriveQuantAction(deriveCtx)
  const size = mapSize(plan)
  const allowDraft = computeAllowDraft(derived, size, entry, existingVolume || plan?.existingVolume || 0)
  const ratio = resolveRatioPct(entry)
  const resolvedAsOf = asOf || new Date().toISOString()

  const decision = {
    id: '',
    asOf: resolvedAsOf,
    tradeDate: tradeDate || '',
    purpose,
    instrument: {
      stockCode: entry?.code || '',
      stockName: entry?.name || '',
    },
    regime: {
      level: regimeRule.level,
      key: regimeRule.key,
      name: regimeRule.name,
      source: 'watchlist_js',
      exposureCap: regimeRule.maxPct,
    },
    signal: {
      tag,
      tagKind: resolveTagKind(tag),
      daysAgo,
      score: entry?.signalScore ?? 0,
      ratioPct: ratio,
      sourceTag: entry?.sourceTag || '',
      summary: entry?.statusText || '',
      barIndex: entry?.signalBar ?? null,
      dayKey: entry?.effectiveSignalDayKey || '',
    },
    entryZone: mapEntryZone(buyPriceRange),
    gate: mapGate(checklist),
    risk: {
      passed: true,
      code: 'APPROVED',
      message: 'phase1 watchlist placeholder (PlanFilter not wired)',
    },
    size,
    holdingBias: mapHoldingBias(authorityHolding),
    action: derived
      ? {
          code: derived.code,
          label: derived.label,
          allowDraft,
          side: derived.side,
          type: derived.type,
          lines: derived.lines,
          tooltip: derived.tooltip,
        }
      : {
          code: '',
          label: '',
          allowDraft: false,
          side: 'none',
        },
    blockers: [],
    meta: {
      schemaVersion: SCHEMA_VERSION,
      producer: 'js_legacy',
      existingVolume: existingVolume || plan?.existingVolume || 0,
      accountEquity: undefined,
    },
    /** 便于 Golden / 过渡期与旧 hint 对比 */
    _legacyHint: derived
      ? {
          label: derived.label,
          type: derived.type,
          tag: derived.tag,
          lines: derived.lines,
          tooltip: derived.tooltip,
          buyPriceRange: derived.buyPriceRange,
        }
      : null,
  }
  return ensureDecisionId(decision)
}
