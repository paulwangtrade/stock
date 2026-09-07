/**
 * QuantDecision Phase1-B0：WatchlistStockCard UI 投影
 * - 完整消费 Decision.entryZone / gate / size / action.allowDraft
 * - Decision 在场时 allowDraft 只读 action.allowDraft（F6）
 * - 无 Decision 时走 legacy 启发式（可见性与 Step3 对齐）
 */

import { formatBuyPriceRangeText, buyPriceRangeLabel, formatPriceTick } from './buyPriceRange.js'
import { formatPositionPlanText } from './buyPositionSizing.js'
import { formatChecklistScore } from './buyChecklist.js'
import { formatSignalTagLabel } from './signalBuyGuide.js'
import { deriveQuantAction, toLegacyActionHint } from './quantActionDerive.js'
import { resolveAuthorityHoldingDecision } from './riskintel/holdingDecisionAdapter.js'
import {
  ensureDecisionId,
  recordActionObservation,
  OBS_CONSUMER_CARD,
  OBS_SOURCE_DECISION,
  OBS_SOURCE_LEGACY,
} from './quantDecisionObservability.js'

function formatActionTooltip(hint) {
  if (!hint) return ''
  return hint.tooltip || hint.lines?.join('\n') || ''
}

/** Decision.entryZone → 旧 buyPriceRange 形（展示用） */
export function zoneFromEntryZone(entryZone) {
  if (!entryZone) return null
  const hasSubstance =
    !!entryZone.text
    || entryZone.instantPrice != null
    || entryZone.low != null
    || entryZone.high != null
    || !!entryZone.instantText
  if (!hasSubstance) return null
  const instantPrice = entryZone.instantPrice
  return {
    low: entryZone.low,
    high: entryZone.high,
    rangeHigh: entryZone.rangeHigh ?? entryZone.high,
    instantPrice,
    instantText:
      entryZone.instantText
      || (instantPrice != null ? formatPriceTick(instantPrice) : undefined),
    mode: entryZone.mode || 'unknown',
    deferMode: entryZone.deferMode || 'same',
    daysAgo: entryZone.daysAgo ?? 0,
    text: entryZone.text || '',
    note: entryZone.note || '',
    extended: !!entryZone.extended,
    tag: entryZone.tag || '',
  }
}

/** Decision.signal (+ holdingBias) → 旧 signal 形 */
export function signalFromDecision(decision) {
  const s = decision?.signal
  if (!s) return null
  const tag = s.tag || ''
  const ratio = s.ratioPct
  const out = {
    tag,
    daysAgo: s.daysAgo ?? 0,
    recentSignalDaysAgo: s.daysAgo ?? 0,
    statusText: s.summary || '',
    sourceTag: s.sourceTag || '',
  }
  if (ratio != null && Number.isFinite(Number(ratio))) {
    if (tag === '加') out.addPositionPct = Number(ratio)
    else if (tag === '冲') out.rushReducePct = Number(ratio)
    else if (tag === '减' || tag === '止') out.sellPositionPct = Number(ratio)
    else if (s.tagKind === 'scale_in') out.addPositionPct = Number(ratio)
    else if (s.tagKind === 'rush_reduce') out.rushReducePct = Number(ratio)
    else if (s.tagKind === 'exit') out.sellPositionPct = Number(ratio)
  }
  const bias = decision.holdingBias
  if (bias) {
    out.holdingAdvice = {
      action: bias.action,
      actionLabel: bias.actionLabel,
      score: bias.score,
      suggestPctDisplay:
        bias.suggestPctDisplay
        ?? (bias.suggestPct != null ? Math.round(Number(bias.suggestPct) * 100) : 0),
      summaryLine: bias.summaryLine || '',
      factors: Array.isArray(bias.factors) ? bias.factors : [],
    }
  }
  return out
}

function buildActionHintFromParts({ signal, buyPriceRange, checklist, decision }) {
  if (decision?.action?.label) {
    const a = decision.action
    const hint = {
      label: a.label,
      type: a.type || 'default',
      lines: Array.isArray(a.lines) ? a.lines : [],
      tooltip: a.tooltip || (Array.isArray(a.lines) ? a.lines.join('\n') : ''),
      actionSource: OBS_SOURCE_DECISION,
      decisionId: decision.id || null,
    }
    if (decision.signal?.tag) hint.tag = decision.signal.tag
    if (buyPriceRange) hint.buyPriceRange = buyPriceRange
    return hint
  }
  const daysAgo = signal?.recentSignalDaysAgo ?? signal?.daysAgo ?? 0
  const authorityHolding = resolveAuthorityHoldingDecision(signal || {})
  const hint = toLegacyActionHint(deriveQuantAction({
    tag: signal?.tag,
    buyPriceRange,
    sellPositionPct: signal?.sellPositionPct,
    addPositionPct: signal?.addPositionPct,
    rushReducePct: signal?.rushReducePct,
    sellVolume: signal?.sellVolume,
    costPrice: signal?.costPrice,
    sourceTag: signal?.sourceTag,
    daysAgo,
    isHistorical: daysAgo > 0,
    checklistReady: checklist?.ready === true,
    holdingDecision: signal?.holdingDecision,
    holdingAdvice: authorityHolding,
  }))
  if (!hint) return null
  return {
    ...hint,
    actionSource: OBS_SOURCE_LEGACY,
    decisionId: decision?.id || null,
  }
}

function buildBuyPriceTooltip(bp) {
  if (!bp) return ''
  const parts = [bp.note]
  if (bp.instantText) parts.push(`出信号价 ${bp.instantText}`)
  if (bp.deferMode === 'wait') parts.push('今日偏高，明日等回踩，不必当日追入')
  else if (bp.deferMode === 'todayOrTomorrow') {
    parts.push('明日仍可买，开盘或盘中落入区间即可')
    if (bp.rangeHigh > bp.instantPrice) {
      parts.push(`出信号价 ${formatPriceTick(bp.instantPrice)}，区间上沿含小幅高开容忍`)
    }
  }
  parts.push('参考区间，非投资建议')
  return parts.filter(Boolean).join(' · ')
}

function buildSignalTooltip({ result, signal, actionHint }) {
  const parts = [`${result?.['股票名称'] || '未知股票'} (${result?.['股票代码'] || '--'})`]
  if (signal?.statusText) parts.push(signal.statusText)
  const advice = resolveAuthorityHoldingDecision(signal || {})
  if (advice?.factors?.length) {
    const lines = advice.factors.map((f) => `${f.impact >= 0 ? '+' : ''}${f.impact} ${f.label}：${f.detail}`)
    parts.push(`持仓辅助（参考）\n${lines.join('\n')}`)
  }
  const action = formatActionTooltip(actionHint)
  if (action) parts.push(action)
  return parts.filter(Boolean).join('\n\n')
}

function planForFormat(plan, decision) {
  if (plan) return plan
  const s = decision?.size
  if (!s) return null
  const hasSubstance =
    !!s.ok
    || !!s.reason
    || Number(s.targetShares) > 0
    || Number(s.addShares) > 0
    || s.stopPrice != null
  if (!hasSubstance) return null
  return {
    ok: s.ok,
    existingVolume: decision.meta?.existingVolume || 0,
    suggestedAddShares: s.addShares,
    suggestedShares: s.targetShares,
    positionPct: s.positionPct,
    reason: s.reason,
    stopPrice: s.stopPrice,
  }
}

function checklistForFormat(checklist, decision) {
  if (checklist) return checklist
  const g = decision?.gate
  if (!g) return null
  const hasItems = Array.isArray(g.items) && g.items.length > 0
  if (!hasItems && !g.ready && !(Number(g.score) > 0)) return null
  return {
    score: g.score,
    ready: g.ready,
    items: g.items,
  }
}

/** F6：Decision 在场时只读 action.allowDraft；无 Decision 时 legacy 启发式（保持可见性） */
function resolveCanCreateDraft({ decision, plan, signalTag, signal, hasHolding }) {
  if (decision) {
    return !!decision.action?.allowDraft
  }
  return !!(
    (plan?.ok && (plan.suggestedAddShares > 0 || plan.suggestedShares > 0))
    || (signalTag && Number(signal?.sellPositionPct) > 0 && hasHolding)
  )
}

/**
 * @returns {object} WatchlistStockCard 量化相关 UI 投影
 */
export function projectWatchlistCard({
  result = null,
  signal = null,
  buyPriceRange = null,
  entryTag = '',
  quantPlan = null,
  quantChecklist = null,
  decision = null,
} = {}) {
  const hasHolding = Number(result?.costVolume) > 0 && Number(result?.costPrice) > 0
  if (decision) ensureDecisionId(decision)

  const resolvedSignal = signal || signalFromDecision(decision)
  const checklist = checklistForFormat(quantChecklist, decision)
  const plan = planForFormat(quantPlan, decision)
  // F3：优先散装区间，否则完整消费 Decision.entryZone
  const zone = buyPriceRange || zoneFromEntryZone(decision?.entryZone)

  const actionHint = buildActionHintFromParts({
    signal: resolvedSignal,
    buyPriceRange: zone,
    checklist,
    decision,
  })

  const advice = resolveAuthorityHoldingDecision(resolvedSignal || {})
  let holdingAdviceLabel = ''
  if (advice && hasHolding) {
    const pct = advice.suggestPctDisplay
    if (pct > 0 && !resolvedSignal?.tag) {
      holdingAdviceLabel = `${advice.actionLabel} ${pct}%`
    } else {
      holdingAdviceLabel = advice.actionLabel
    }
  }
  const holdingAdviceType =
    advice?.action === 'add' ? 'success' : advice?.action === 'reduce' ? 'error' : 'default'

  const signalTag = resolvedSignal?.tag || ''
  const signalLabel = signalTag
    ? formatSignalTagLabel(
      signalTag,
      resolvedSignal?.sellPositionPct ?? resolvedSignal?.addPositionPct ?? resolvedSignal?.rushReducePct,
    )
    : ''

  const canCreateDraft = resolveCanCreateDraft({
    decision,
    plan,
    signalTag,
    signal: resolvedSignal,
    hasHolding,
  })

  let draftButtonLabel = '生成交易草稿'
  if (plan?.ok) draftButtonLabel = '生成买入草稿'
  else if (Number(resolvedSignal?.sellPositionPct) > 0) draftButtonLabel = '生成卖出草稿'

  const checklistText = formatChecklistScore(checklist)
  const checklistTooltip = checklist?.items?.length
    ? checklist.items.map((i) => `${i.passed ? '✓' : '✗'} ${i.label}`).join('\n')
    : ''

  const planText = formatPositionPlanText(plan)
  const planReason = plan?.reason || ''
  const planStopPrice = plan?.stopPrice
  const planStopText = planStopPrice ? ` · 止损≈${formatPriceTick(planStopPrice)}` : ''

  const showQuantStat = !!(plan?.ok || checklist)
  const showChecklist = !!checklist
  const showPlan = !!plan
  const checklistReady = !!checklist?.ready

  const tintTag =
    signalTag
    || entryTag
    || decision?.signal?.tag
    || zone?.tag
    || ''

  const decisionId = decision?.id || actionHint?.decisionId || null
  const actionSource = actionHint?.actionSource
    || (decision?.action?.label ? OBS_SOURCE_DECISION : OBS_SOURCE_LEGACY)
  const code = result?.['股票代码'] || decision?.instrument?.stockCode || ''

  recordActionObservation({
    consumer: OBS_CONSUMER_CARD,
    code,
    asOf: decision?.asOf || '',
    decisionId,
    actionLabel: actionHint?.label || '',
    actionCode: decision?.action?.code || '',
    actionSource,
  })

  return {
    signal: resolvedSignal,
    signalTag,
    signalLabel,
    signalTooltip: buildSignalTooltip({ result, signal: resolvedSignal, actionHint }),
    actionHint,
    actionLabel: actionHint?.label || '',
    actionType: actionHint?.type || 'default',
    holdingAdviceLabel,
    holdingAdviceType,
    showHoldingAdvice: !!(holdingAdviceLabel && !signalTag),
    showSignalHeader: !!(signalTag || holdingAdviceLabel),

    buyPriceRange: zone,
    showBuyRange: !!zone,
    buyRangeLabel: buyPriceRangeLabel(zone),
    buyPriceDisplay: formatBuyPriceRangeText(zone),
    buyPriceTooltip: buildBuyPriceTooltip(zone),
    buyRangeExtended: !!zone?.extended,

    showQuantStat,
    showChecklist,
    showPlan,
    checklistReady,
    checklistText,
    checklistTooltip,
    planText,
    planReason,
    planStopPrice,
    planStopTooltip: `${planReason}${planStopText}`,

    canCreateDraft,
    draftButtonLabel,
    tintTag,

    /** Phase1-C 观测（不进 UI 文案） */
    decisionId,
    actionSource,
    asOf: decision?.asOf || '',
  }
}

/** 从投影提取可见文案快照（UI Golden） */
export function watchlistProjectionUiSnapshot(p) {
  if (!p) return null
  return {
    signalLabel: p.signalLabel,
    actionLabel: p.actionLabel,
    actionType: p.actionType,
    actionTooltip: p.actionHint?.tooltip || '',
    holdingAdviceLabel: p.holdingAdviceLabel,
    buyRangeLabel: p.buyRangeLabel,
    buyPriceDisplay: p.buyPriceDisplay,
    buyPriceTooltip: p.buyPriceTooltip,
    checklistText: p.checklistText,
    checklistTooltip: p.checklistTooltip,
    planText: p.planText,
    planStopTooltip: p.planStopTooltip,
    draftButtonLabel: p.draftButtonLabel,
    canCreateDraft: p.canCreateDraft,
    showQuantStat: p.showQuantStat,
    showBuyRange: p.showBuyRange,
    showHoldingAdvice: p.showHoldingAdvice,
  }
}
