/** 信号操作提示：可买 / 等回踩 / 仅观察 / 先风控（参考，非投资建议）
 * Phase1-A/B2：resolveSignalActionHint / buildBarSignalHint 为兼容 adapter；
 * Action label 唯一生产点仍为 quantActionDerive。
 */

import { BUY_ENTRY_TAGS, calcBuyPriceRange } from './buyPriceRange'
import { isSellSignalTag } from './icePointSignals'
import { calcSellPositionPct } from './sellPositionRatio'
import { calcAddPositionPct } from './addPositionRatio'
import { isAddPositionTag } from './addPositionSignals'
import { isRushReduceTag } from './rushReduceSignals'
import { calcRushReducePct } from './rushReduceRatio'
import { deriveQuantAction, toLegacyActionHint } from './quantActionDerive.js'
import {
  ensureDecisionId,
  recordActionObservation,
  OBS_CONSUMER_KLINE,
  OBS_SOURCE_DECISION,
  OBS_SOURCE_LEGACY,
} from './quantDecisionObservability.js'

/** K 线 / 兼容：消费 Decision.action */
export const BAR_ACTION_SOURCE_DECISION = 'decision'
/** 无可用 Decision 时：adapter → derive（可观测） */
export const BAR_ACTION_SOURCE_LEGACY = 'legacy_fallback'

/**
 * 兼容 adapter：输出 shape 与历史 resolveSignalActionHint 一致。
 * @returns {{ label: string, type: string, tag?: string, lines: string[], tooltip: string, buyPriceRange?: object } | null}
 */
export function resolveSignalActionHint(ctx = {}) {
  return toLegacyActionHint(deriveQuantAction(ctx))
}

/** Decision.action → hint 形（只透传，不派生） */
export function actionHintFromDecision(decision) {
  const a = decision?.action
  if (!a?.label) return null
  ensureDecisionId(decision)
  const hint = {
    label: a.label,
    type: a.type || 'default',
    lines: Array.isArray(a.lines) ? a.lines : [],
    tooltip: a.tooltip || (Array.isArray(a.lines) ? a.lines.join('\n') : ''),
    decisionId: decision.id || null,
  }
  if (decision.signal?.tag) hint.tag = decision.signal.tag
  if (decision._legacyHint?.buyPriceRange) hint.buyPriceRange = decision._legacyHint.buyPriceRange
  return hint
}

/** 当前 K 线信号是否与 Decision.signal 对齐（同 tag + daysAgo） */
export function decisionMatchesBar(decision, tag, daysAgo) {
  if (!decision?.action?.label) return false
  if ((decision.signal?.tag || '') !== (tag || '')) return false
  const dDays = decision.signal?.daysAgo
  if (dDays != null && Number.isFinite(Number(dDays))) {
    return Number(dDays) === Number(daysAgo)
  }
  return Number(daysAgo) === 0
}

function withActionSource(hint, actionSource) {
  if (!hint) return null
  return { ...hint, actionSource }
}

/**
 * 从 K 线 bar 组装 derive ctx（不写 label；可选 options 覆盖区间/清单）
 * @returns {{ ctx: object, daysAgo: number } | null}
 */
export function buildBarSignalHintCtx({
  sig,
  bars,
  barIndex,
  tag,
  options = {},
  livePrice,
} = {}) {
  if (!tag || barIndex == null || barIndex < 0) return null
  const closes = bars?.closes ?? []
  const last = closes.length - 1
  if (last < 0) return null
  const daysAgo = Math.max(0, last - barIndex)
  const isHistorical = daysAgo > 0
  const nowPrice = livePrice ?? closes[last]

  if (isSellSignalTag(tag)) {
    const sellPositionPct =
      options.sellPositionPct ?? calcSellPositionPct(tag, sig, barIndex, bars, options)
    return {
      daysAgo,
      ctx: { tag, sellPositionPct, daysAgo, isHistorical },
    }
  }

  if (isRushReduceTag(tag)) {
    const rushReducePct =
      options.rushReducePct ?? calcRushReducePct(barIndex, bars, sig, options)
    return {
      daysAgo,
      ctx: {
        tag,
        rushReducePct,
        sellPositionPct: rushReducePct,
        daysAgo,
        isHistorical,
      },
    }
  }

  if (isAddPositionTag(tag)) {
    const sourceTag = options.sourceTag
    const addPositionPct =
      options.addPositionPct ?? calcAddPositionPct(sourceTag, barIndex, bars, sig, options)
    return {
      daysAgo,
      ctx: { tag, addPositionPct, sourceTag, daysAgo, isHistorical },
    }
  }

  if (BUY_ENTRY_TAGS.has(tag)) {
    const summary = {
      tag,
      recentSignalBar: barIndex,
      recentSignalDaysAgo: daysAgo,
      signalLastIndex: last,
      recentSignalConfirmBar: tag === '强' || tag === '突' ? barIndex : undefined,
    }
    const buyPriceRange = options.buyPriceRange ?? calcBuyPriceRange(summary, bars, options)
    return {
      daysAgo,
      ctx: {
        tag,
        buyPriceRange,
        daysAgo,
        isHistorical,
        livePrice: nowPrice,
        checklistReady: options.checklistReady === true,
      },
    }
  }

  if (tag === '冰') {
    return { daysAgo, ctx: { tag, daysAgo, isHistorical } }
  }

  return null
}

/**
 * K 线某根上的信号 → 操作提示。
 * - 优先消费 QuantDecision.action（tag/daysAgo 对齐时）
 * - 否则 legacy_fallback → resolveSignalActionHint（adapter）
 * @returns {{ label, type, lines, tooltip, actionSource, ... } | null}
 */
export function buildBarSignalHint({
  sig,
  bars,
  barIndex,
  tag,
  options = {},
  livePrice,
  decision = null,
} = {}) {
  const packed = buildBarSignalHintCtx({ sig, bars, barIndex, tag, options, livePrice })
  if (!packed) return null
  const { ctx, daysAgo } = packed
  if (decision) ensureDecisionId(decision)
  const code = options.code || decision?.instrument?.stockCode || ''

  if (decisionMatchesBar(decision, tag, daysAgo)) {
    const fromDecision = actionHintFromDecision(decision)
    if (fromDecision) {
      if (ctx.buyPriceRange && fromDecision.buyPriceRange == null) {
        fromDecision.buyPriceRange = ctx.buyPriceRange
      }
      const hint = withActionSource(fromDecision, BAR_ACTION_SOURCE_DECISION)
      hint.decisionId = decision.id || null
      recordActionObservation({
        consumer: OBS_CONSUMER_KLINE,
        code,
        asOf: decision.asOf || '',
        decisionId: decision.id || null,
        actionLabel: hint.label,
        actionCode: decision.action?.code || '',
        actionSource: OBS_SOURCE_DECISION,
      })
      return hint
    }
  }

  const legacy = withActionSource(resolveSignalActionHint(ctx), BAR_ACTION_SOURCE_LEGACY)
  if (legacy) {
    legacy.decisionId = decision?.id || null
    recordActionObservation({
      consumer: OBS_CONSUMER_KLINE,
      code,
      asOf: decision?.asOf || '',
      decisionId: decision?.id || null,
      actionLabel: legacy.label,
      actionCode: '',
      actionSource: OBS_SOURCE_LEGACY,
    })
  }
  return legacy
}

export function formatSignalActionTooltip(hint) {
  if (!hint) return ''
  return hint.tooltip || hint.lines?.join('\n') || ''
}
