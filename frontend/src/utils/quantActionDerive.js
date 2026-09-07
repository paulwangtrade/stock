/** QuantDecision Phase1-A：唯一 Action 派生（与 resolveSignalActionHint 文案对齐） */

import { BUY_ENTRY_TAGS, formatPriceTick } from './buyPriceRange.js'
import {
  formatSellPositionPct,
  sellPositionHint,
} from './sellPositionRatio.js'
import {
  formatAddPositionPct,
  addPositionHint,
} from './addPositionRatio.js'
import {
  formatRushReducePct,
  rushReduceHint,
} from './rushReduceRatio.js'
import { resolveAuthorityHoldingDecision } from './riskintel/holdingDecisionAdapter.js'

export const QUANT_ACTION_ENTER = 'ENTER'
export const QUANT_ACTION_WAIT_PULLBACK = 'WAIT_PULLBACK'
export const QUANT_ACTION_WATCH = 'WATCH'
export const QUANT_ACTION_SCALE_IN = 'SCALE_IN'
export const QUANT_ACTION_REDUCE = 'REDUCE'
export const QUANT_ACTION_EXIT_PARTIAL = 'EXIT_PARTIAL'
export const QUANT_ACTION_HOLD = 'HOLD'
export const QUANT_ACTION_BLOCKED = 'BLOCKED'

/** 与 icePointSignals / addPositionSignals / rushReduceSignals 判定对齐（避免拉入 extensionless 依赖链） */
function isSellSignalTag(tag) {
  return tag === '止' || tag === '减'
}
function isAddPositionTag(tag) {
  return tag === '加'
}
function isRushReduceTag(tag) {
  return tag === '冲'
}

/**
 * 从与 resolveSignalActionHint 相同的 ctx 派生 Action。
 * label/type/lines/tooltip 必须与旧 hint 一致（Golden Test 锁定）。
 *
 * @returns {{
 *   code: string,
 *   label: string,
 *   type: string,
 *   side: string,
 *   tag?: string,
 *   lines: string[],
 *   tooltip: string,
 *   buyPriceRange?: object,
 * } | null}
 */
export function deriveQuantAction(ctx = {}) {
  const {
    tag,
    buyPriceRange,
    sellPositionPct,
    daysAgo = 0,
    isHistorical = false,
    checklistReady,
  } = ctx
  // Phase9-C.1: authority decision via adapter choke (shape-compatible with legacy holdingAdvice)
  const holdingAdvice = resolveAuthorityHoldingDecision({
    holdingDecision: ctx.holdingDecision,
    holdingAdvice: ctx.holdingAdvice,
    riskAdviceProjection: ctx.riskAdviceProjection,
    symbol: ctx.symbol || ctx.code,
  })

  if (!tag) {
    if (holdingAdvice?.action && holdingAdvice.action !== 'hold') {
      const pct = holdingAdvice.suggestPctDisplay
      const lines = [
        `持仓辅助 · ${holdingAdvice.actionLabel}`,
        holdingAdvice.summaryLine,
        pct > 0 ? `参考比例约 ${pct}%` : '综合量价与板块，参考非投资建议',
      ]
      return {
        code: QUANT_ACTION_HOLD,
        label: holdingAdvice.actionLabel,
        type: holdingAdvice.action === 'add' ? 'success' : holdingAdvice.action === 'reduce' ? 'error' : 'default',
        side: 'none',
        lines,
        tooltip: lines.join('\n'),
      }
    }
    return null
  }

  if (isSellSignalTag(tag)) {
    const pct = formatSellPositionPct(sellPositionPct)
    const action = tag === '减' ? '减仓' : '止盈'
    const hint = sellPositionHint(tag, sellPositionPct)
    const sellVol = ctx.sellVolume
    const lines = [
      `${tag} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议${action} ${pct}` : `建议${action}`),
    ]
    if (sellVol > 0 && ctx.costPrice > 0) {
      lines.push(`成本 ${Number(ctx.costPrice).toFixed(2)} · 卖 ${sellVol} 股`)
    }
    if (tag === '止') lines.push('须已有浮盈且自高点回撤；趋势未破 MA20 勿当清仓')
    if (tag === '减') lines.push('收盘跌破 MA20，结构转弱')
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      code: tag === '止' ? QUANT_ACTION_EXIT_PARTIAL : QUANT_ACTION_REDUCE,
      label: isHistorical ? '历史参考' : '先风控',
      type: 'error',
      side: 'sell',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (tag === '冰') {
    return {
      code: QUANT_ACTION_WATCH,
      label: '仅观察',
      type: 'info',
      side: 'none',
      tag,
      lines: ['冰点区 · 等待 RSI 上穿 30 出「买」', '参考，非投资建议'],
      tooltip: '冰点区 · 等待 RSI 上穿 30 出「买」\n参考，非投资建议',
    }
  }

  if (isRushReduceTag(tag)) {
    const pctVal = ctx.rushReducePct ?? ctx.sellPositionPct
    const pct = formatRushReducePct(pctVal)
    const hint = rushReduceHint(pctVal)
    const lines = [
      `早减 · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议早减 ${pct}` : '新高阳后首阴，宜提前减仓'),
      '有浮盈时早于「减」触发，参考非投资建议',
    ]
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      code: QUANT_ACTION_REDUCE,
      label: isHistorical ? '历史参考' : '早减',
      type: 'warning',
      side: 'sell',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (isAddPositionTag(tag)) {
    const { addPositionPct, sourceTag } = ctx
    const pct = formatAddPositionPct(addPositionPct)
    const hint = addPositionHint(addPositionPct)
    const lines = [
      `加仓${sourceTag ? `(${sourceTag})` : ''} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
      hint || (pct ? `建议加仓 ${pct}` : '趋势未破，可考虑分批加码'),
      '须已有持仓且浮盈，参考非投资建议',
    ]
    if (isHistorical) lines.push('历史信号，仅供对照')
    return {
      code: QUANT_ACTION_SCALE_IN,
      label: isHistorical ? '历史参考' : '可加仓',
      type: 'success',
      side: 'buy',
      tag,
      lines,
      tooltip: lines.join('\n'),
    }
  }

  if (!BUY_ENTRY_TAGS.has(tag)) return null

  const rangeText = buyPriceRange?.text || '—'
  const instant =
    buyPriceRange?.instantText ||
    (buyPriceRange?.instantPrice != null ? formatPriceTick(buyPriceRange.instantPrice) : '—')
  const lines = [
    `${tag} · ${daysAgo === 0 ? '今日' : `${daysAgo}日前`}`,
    `出信号价 ${instant}`,
    `买入参考 ${rangeText}`,
  ]

  let code = QUANT_ACTION_WATCH
  let label = '仅观察'
  let type = 'info'

  if (isHistorical) {
    code = QUANT_ACTION_WATCH
    label = '历史参考'
    type = 'default'
    lines.push('历史信号，仅供对照')
  } else if (checklistReady === true) {
    code = QUANT_ACTION_ENTER
    label = '可买'
    type = 'success'
    lines.push('清单已通过，落入区间可考虑')
  } else if (buyPriceRange?.mode === 'above' || buyPriceRange?.deferMode === 'wait') {
    code = QUANT_ACTION_WAIT_PULLBACK
    label = '等回踩'
    type = 'warning'
    lines.push('现价偏高，等落入参考区间')
  } else if (buyPriceRange?.mode === 'inZone' || buyPriceRange?.mode === 'near') {
    if (tag === '买' && daysAgo === 0) {
      code = QUANT_ACTION_WATCH
      label = '仅观察'
      type = 'info'
      lines.push('出买当日宜观察，1～2 日回踩再试')
    } else {
      code = QUANT_ACTION_ENTER
      label = '可买'
      type = 'success'
      lines.push('现价接近参考区间，可考虑')
    }
  } else if (buyPriceRange?.mode === 'below') {
    code = QUANT_ACTION_ENTER
    label = '可买'
    type = 'success'
    lines.push('现价低于出信号价，区间仍有效')
  } else if (tag === '买') {
    code = QUANT_ACTION_WATCH
    label = '仅观察'
    lines.push('等回踩确认后再小仓试')
  } else if (tag === '强' || tag === '突' || tag === '弹' || tag === '转') {
    code = QUANT_ACTION_WAIT_PULLBACK
    label = '等回踩'
    type = 'warning'
    lines.push(tag === '转' ? '转势确认后等回踩再进' : '不宜追信号当日大阳线')
  }

  lines.push('参考区间，非投资建议')
  return {
    code,
    label,
    type,
    side: code === QUANT_ACTION_ENTER ? 'buy' : 'none',
    tag,
    lines,
    tooltip: lines.join('\n'),
    buyPriceRange,
  }
}

/** 投影为旧 resolveSignalActionHint 返回 shape（不含 code/side） */
export function toLegacyActionHint(derived) {
  if (!derived) return null
  const out = {
    label: derived.label,
    type: derived.type,
    lines: derived.lines,
    tooltip: derived.tooltip,
  }
  if (derived.tag != null) out.tag = derived.tag
  if (derived.buyPriceRange !== undefined) out.buyPriceRange = derived.buyPriceRange
  return out
}
